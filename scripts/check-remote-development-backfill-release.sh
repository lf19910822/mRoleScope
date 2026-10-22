#!/usr/bin/env bash
# Keep the root-only baseline backfill bound to the verified worker digest and
# a fixed database-only Compose service. It must accept no caller arguments.

set -euo pipefail

readonly backfill_helper="${1:-deploy/remote-development/mrolescope-backfill-versioned-content}"
readonly backfill_service="${2:-deploy/remote-development/mrolescope-backfill-versioned-content.service}"
readonly operations_helper="${3:-deploy/remote-development/mrolescope-operations}"
readonly operations_workflow="${4:-.github/workflows/remote-development-operations.yml}"
readonly bootstrap="${5:-deploy/remote-development/bootstrap.sh}"

die() {
  printf 'remote-development versioned-content backfill release check failed: %s\n' "$1" >&2
  exit 1
}

[[ -f "$backfill_helper" ]] || die "backfill helper does not exist: $backfill_helper"
bash -n "$backfill_helper"
grep -Fqx '[[ "$#" -eq 1 && "$1" == '\''run'\'' ]] || die '\''usage: mrolescope-backfill-versioned-content run'\''' "$backfill_helper" \
  || die 'backfill helper must accept only the fixed run operation'
for mapping in \
  '  export MROLESCOPE_API_IMAGE="$api_image"' \
  '  export MROLESCOPE_MIGRATE_IMAGE="$migrate_image"' \
  '  export MROLESCOPE_WORKER_IMAGE="$worker_image"'; do
  grep -Fqx "$mapping" "$backfill_helper" \
    || die "backfill helper must export the verified image mapping: ${mapping}"
done
grep -Fqx 'compose --profile backfill run --rm --no-deps backfill' "$backfill_helper" \
  || die 'backfill helper must run only the fixed backfill Compose service'

[[ -f "$backfill_service" ]] || die "backfill service does not exist: $backfill_service"
grep -Fqx 'ExecStart=/usr/local/sbin/mrolescope-backfill-versioned-content run' "$backfill_service" \
  || die 'backfill service must start only the fixed root-owned helper'

[[ -f "$operations_helper" ]] || die "operations helper does not exist: $operations_helper"
grep -Fqx "readonly BACKFILL_HELPER='/usr/local/sbin/mrolescope-backfill-versioned-content'" "$operations_helper" \
  || die 'operations helper must use the fixed backfill helper path'
grep -Fqx '    systemctl start --wait "$BACKFILL_SERVICE"' "$operations_helper" \
  || die 'operations helper must use the fixed backfill systemd unit'

[[ -f "$operations_workflow" ]] || die "operations workflow does not exist: $operations_workflow"
grep -Fqx '          - backfill-versioned-content' "$operations_workflow" \
  || die 'operations workflow must allowlist the backfill operation'
grep -Fqx "              expected='BACKFILL_VERSIONED_CONTENT'" "$operations_workflow" \
  || die 'operations workflow must require the fixed backfill confirmation'

[[ -f "$bootstrap" ]] || die "bootstrap script does not exist: $bootstrap"
grep -Fqx "download_asset 'deploy/remote-development/mrolescope-backfill-versioned-content' \"\$staging_dir/mrolescope-backfill-versioned-content\"" "$bootstrap" \
  || die 'bootstrap must fetch the fixed backfill helper'
grep -Fqx "install -o root -g root -m 0750 \"\$staging_dir/mrolescope-backfill-versioned-content\" \"\$BACKFILL_HELPER_DESTINATION\"" "$bootstrap" \
  || die 'bootstrap must install the backfill helper root-owned and non-world-executable'
