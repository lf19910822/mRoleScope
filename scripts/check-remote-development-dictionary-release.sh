#!/usr/bin/env bash
# Verify that the v0.1 and v0.2 dictionary lifecycles remain narrow root-owned
# boundaries. The workflow may select a reviewed operation and rule key only;
# it must never supply source text, a database URL, a container image, or SQL.

set -euo pipefail

readonly dictionary_helper="${1:-deploy/remote-development/mrolescope-dictionary}"
readonly operations_helper="${2:-deploy/remote-development/mrolescope-operations}"
readonly workflow="${3:-.github/workflows/remote-development-operations.yml}"
readonly bootstrap="${4:-deploy/remote-development/bootstrap.sh}"
readonly candidate_unit="${5:-deploy/remote-development/mrolescope-candidate-analysis.service}"
readonly candidate_v02_unit="${6:-deploy/remote-development/mrolescope-candidate-analysis-v0.2.service}"

die() {
  printf 'remote-development dictionary release check failed: %s\n' "$1" >&2
  exit 1
}

for path in "$dictionary_helper" "$operations_helper" "$workflow" "$bootstrap" "$candidate_unit" "$candidate_v02_unit"; do
  [[ -f "$path" ]] || die "required file does not exist: ${path}"
done

bash -n "$dictionary_helper" "$operations_helper" "$bootstrap"

grep -Fqx "readonly RULE_KEY_PATTERN='^[a-z0-9][a-z0-9.-]{0,127}\$'" "$dictionary_helper" \
  || die 'dictionary helper must restrict rule keys to the reviewed format'
grep -Fqx "readonly OPERATOR_PATTERN='^[A-Za-z0-9][A-Za-z0-9._-]{0,255}\$'" "$dictionary_helper" \
  || die 'dictionary helper must restrict operator identities'
for fixed_service in dictionary-seed dictionary-approve dictionary-prepare dictionary-v0.2-seed dictionary-v0.2-approve dictionary-v0.2-prepare; do
  grep -Fqx "    compose --profile dictionary run --rm --no-deps ${fixed_service}" "$dictionary_helper" \
    || die "dictionary helper must invoke only the fixed ${fixed_service} service"
done
grep -Fqx "    die 'usage: mrolescope-dictionary <seed-v0.1|approve-v0.1-rule|prepare-v0.1-analysis|seed-v0.2|approve-v0.2-rule|prepare-v0.2-analysis> <operator-or-rule-key> [operator]'" "$dictionary_helper" \
  || die 'dictionary helper must expose only fixed lifecycle actions'

for operation in seed-v0.1-dictionary approve-v0.1-rule prepare-v0.1-analysis seed-v0.2-dictionary approve-v0.2-rule prepare-v0.2-analysis; do
  grep -Fqx "  ${operation})" "$operations_helper" \
    || die "operations helper must allowlist ${operation}"
done
grep -Fqx '  run-v0.1-candidate-analysis)' "$operations_helper" \
  || die 'operations helper must allowlist the isolated candidate analysis run'
grep -Fqx '    systemctl start --wait "$CANDIDATE_ANALYSIS_SERVICE"' "$operations_helper" \
  || die 'candidate analysis must use its dedicated systemd unit'
grep -Fqx '  run-v0.2-candidate-analysis)' "$operations_helper" \
  || die 'operations helper must allowlist the isolated v0.2 candidate analysis run'
grep -Fqx '    systemctl start --wait "$CANDIDATE_V02_ANALYSIS_SERVICE"' "$operations_helper" \
  || die 'v0.2 candidate analysis must use its dedicated systemd unit'

for operation in seed-v0.1-dictionary approve-v0.1-rule prepare-v0.1-analysis run-v0.1-candidate-analysis seed-v0.2-dictionary approve-v0.2-rule prepare-v0.2-analysis run-v0.2-candidate-analysis; do
  grep -Fqx "          - ${operation}" "$workflow" \
    || die "workflow must expose the reviewed ${operation} choice"
done
grep -Fqx "              expected='SEED_V01_DICTIONARY'" "$workflow" \
  || die 'workflow must require seed confirmation'
grep -Fqx "              expected='APPROVE_V01_RULE'" "$workflow" \
  || die 'workflow must require per-rule approval confirmation'
grep -Fqx "              expected='PREPARE_V01_ANALYSIS'" "$workflow" \
  || die 'workflow must require candidate preparation confirmation'
grep -Fqx "              expected='RUN_V01_CANDIDATE_ANALYSIS'" "$workflow" \
  || die 'workflow must require candidate-analysis confirmation'
grep -Fqx "              expected='SEED_V02_DICTIONARY'" "$workflow" \
  || die 'workflow must require v0.2 seed confirmation'
grep -Fqx "              expected='APPROVE_V02_RULE'" "$workflow" \
  || die 'workflow must require v0.2 per-rule approval confirmation'
grep -Fqx "              expected='PREPARE_V02_ANALYSIS'" "$workflow" \
  || die 'workflow must require v0.2 candidate preparation confirmation'
grep -Fqx "              expected='RUN_V02_CANDIDATE_ANALYSIS'" "$workflow" \
  || die 'workflow must require v0.2 candidate-analysis confirmation'
grep -Fqx '          REVIEWER: ${{ github.actor }}' "$workflow" \
  || die 'workflow must derive the operator identity from the authenticated actor'
grep -Fqx '              [[ "$RULE_KEY" =~ ^[a-z0-9][a-z0-9.-]{0,127}$ ]] || {' "$workflow" \
  || die 'workflow must validate a reviewed v0.1 rule-key format'

grep -Fqx "readonly DICTIONARY_HELPER_DESTINATION='/usr/local/sbin/mrolescope-dictionary'" "$bootstrap" \
  || die 'bootstrap must install the root-owned dictionary helper'
grep -Fqx "readonly CANDIDATE_ANALYSIS_SERVICE_DESTINATION='/etc/systemd/system/mrolescope-candidate-analysis.service'" "$bootstrap" \
  || die 'bootstrap must install the candidate analysis service unit'
grep -Fqx "readonly CANDIDATE_V02_ANALYSIS_SERVICE_DESTINATION='/etc/systemd/system/mrolescope-candidate-analysis-v0.2.service'" "$bootstrap" \
  || die 'bootstrap must install the v0.2 candidate analysis service unit'
grep -Fqx "download_asset 'deploy/remote-development/mrolescope-dictionary' \"\$staging_dir/mrolescope-dictionary\"" "$bootstrap" \
  || die 'bootstrap must stage the dictionary helper before installation'
grep -Fqx "download_asset 'deploy/remote-development/mrolescope-candidate-analysis.service' \"\$staging_dir/mrolescope-candidate-analysis.service\"" "$bootstrap" \
  || die 'bootstrap must stage the candidate analysis unit before installation'
grep -Fqx "download_asset 'deploy/remote-development/mrolescope-candidate-analysis-v0.2.service' \"\$staging_dir/mrolescope-candidate-analysis-v0.2.service\"" "$bootstrap" \
  || die 'bootstrap must stage the v0.2 candidate analysis unit before installation'
grep -Fqx 'ExecStart=/usr/local/sbin/mrolescope-analysis candidate-v0.1' "$candidate_unit" \
  || die 'candidate unit must invoke only the fixed candidate-analysis mode'
grep -Fqx 'ExecStart=/usr/local/sbin/mrolescope-analysis candidate-v0.2' "$candidate_v02_unit" \
  || die 'v0.2 candidate unit must invoke only the fixed candidate-analysis mode'

printf '%s\n' 'remote-development dictionary release check passed'
