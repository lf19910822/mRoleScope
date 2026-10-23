#!/usr/bin/env bash
# Keep the root-only analysis helper bound to the same verified worker digest
# used by deployment and synchronization; it must not accept caller images.

set -euo pipefail

readonly analysis_helper="${1:-deploy/remote-development/mrolescope-analysis}"
readonly sync_helper="${2:-deploy/remote-development/mrolescope-sync}"

die() {
  printf 'remote-development analysis release check failed: %s\n' "$1" >&2
  exit 1
}

[[ -f "$analysis_helper" ]] || die "analysis helper does not exist: $analysis_helper"
bash -n "$analysis_helper"
grep -Fqx '[[ "$#" -eq 1 && ( "$1" == '\''pending'\'' || "$1" == '\''candidate-v0.1'\'' || "$1" == '\''candidate-v0.2'\'' ) ]] \' "$analysis_helper" \
  || die 'analysis helper must accept only the three fixed analysis operations'
grep -Fqx '  || die '\''usage: mrolescope-analysis <pending|candidate-v0.1|candidate-v0.2>'\''' "$analysis_helper" \
  || die 'analysis helper must state the three fixed analysis operations'
for mapping in \
  '  export MROLESCOPE_API_IMAGE="$api_image"' \
  '  export MROLESCOPE_MIGRATE_IMAGE="$migrate_image"' \
  '  export MROLESCOPE_WORKER_IMAGE="$worker_image"'; do
  grep -Fqx "$mapping" "$analysis_helper" \
    || die "analysis helper must export the verified image mapping: ${mapping}"
done
grep -Fqx '    compose --profile analysis run --rm --no-deps analysis' "$analysis_helper" \
  || die 'analysis helper must run the fixed published-analysis Compose service'
grep -Fqx '    compose --profile candidate-analysis run --rm --no-deps candidate-analysis' "$analysis_helper" \
  || die 'analysis helper must run the fixed candidate-analysis Compose service'
grep -Fqx '    compose --profile candidate-analysis-v0.2 run --rm --no-deps candidate-analysis-v0.2' "$analysis_helper" \
  || die 'analysis helper must run the fixed v0.2 candidate-analysis Compose service'
[[ -f "$sync_helper" ]] || die "sync helper does not exist: $sync_helper"
grep -Fqx '    if ! systemctl start --no-block "$ANALYSIS_SERVICE"; then' "$sync_helper" \
  || die 'sync must trigger analysis only as a best-effort non-blocking action'
