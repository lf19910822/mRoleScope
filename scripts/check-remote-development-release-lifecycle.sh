#!/usr/bin/env bash
# Verify the first-baseline release lifecycle is limited to fixed v0.2
# operations. The runner may convey only its authenticated actor; it cannot
# choose an analysis configuration, SQL, image, Compose path, or free text.

set -euo pipefail

readonly release_helper="${1:-deploy/remote-development/mrolescope-release}"
readonly operations_helper="${2:-deploy/remote-development/mrolescope-operations}"
readonly workflow="${3:-.github/workflows/remote-development-operations.yml}"
readonly bootstrap="${4:-deploy/remote-development/bootstrap.sh}"
readonly compose_file="${5:-deploy/remote-development/compose.yaml}"

die() {
  printf 'remote-development release lifecycle check failed: %s\n' "$1" >&2
  exit 1
}

for path in "$release_helper" "$operations_helper" "$workflow" "$bootstrap" "$compose_file"; do
  [[ -f "$path" ]] || die "required file does not exist: ${path}"
done

bash -n "$release_helper" "$operations_helper" "$bootstrap"

grep -Fqx "readonly OPERATOR_PATTERN='^[A-Za-z0-9][A-Za-z0-9._-]{0,255}\$'" "$release_helper" \
  || die 'release helper must validate the authenticated publisher format'
for service in analysis-release-v0.2 analysis-withdraw-v0.2-quality analysis-withdraw-v0.2-security; do
  grep -Fqx "    compose --profile release-v0.2 run --rm --no-deps ${service}" "$release_helper" \
    || die "release helper must invoke only fixed ${service}"
done
grep -Fqx "    die 'usage: mrolescope-release <publish-v0.2|withdraw-v0.2-quality|withdraw-v0.2-security> <authenticated-operator>'" "$release_helper" \
  || die 'release helper must expose only fixed v0.2 lifecycle operations'

for operation in publish-v0.2-baseline withdraw-v0.2-material-result-quality withdraw-v0.2-security-privacy; do
  grep -Fqx "  ${operation})" "$operations_helper" \
    || die "operations helper must allowlist ${operation}"
  grep -Fqx "          - ${operation}" "$workflow" \
    || die "workflow must expose the fixed ${operation} choice"
done
grep -Fqx '          PUBLISHER: ${{ github.actor }}' "$workflow" \
  || die 'workflow must derive the release publisher from the authenticated actor'
for confirmation in PUBLISH_V02_BASELINE WITHDRAW_V02_MATERIAL_RESULT_QUALITY_DEFECT WITHDRAW_V02_SECURITY_PRIVACY_INCIDENT; do
  grep -Fqx "              expected='${confirmation}'" "$workflow" \
    || die "workflow must require the ${confirmation} confirmation"
done

grep -Fqx "readonly RELEASE_HELPER_DESTINATION='/usr/local/sbin/mrolescope-release'" "$bootstrap" \
  || die 'bootstrap must install the root-owned release helper'
grep -Fqx "download_asset 'deploy/remote-development/mrolescope-release' \"\$staging_dir/mrolescope-release\"" "$bootstrap" \
  || die 'bootstrap must stage the release helper before installation'

for service in analysis-release-v0.2 analysis-withdraw-v0.2-quality analysis-withdraw-v0.2-security; do
  block=$(awk -v service="$service" '
    $0 == "  " service ":" { in_block = 1; next }
    in_block && /^  [A-Za-z0-9][A-Za-z0-9-]*:$/ { exit }
    in_block { print }
  ' "$compose_file")
  [[ -n "$block" ]] || die "${service} Compose service was not found"
  grep -Fqx '    profiles: [release-v0.2]' <<<"$block" \
    || die "${service} must use only the release-v0.2 profile"
  grep -Fqx '    networks: [backend]' <<<"$block" \
    || die "${service} must use only the internal backend network"
  if grep -Eq '^[[:space:]]+(COS_|ports:|volumes:)|^[[:space:]]+networks:.*edge' <<<"$block"; then
    die "${service} must not receive COS, edge-network, ports, or volumes"
  fi
done

printf '%s\n' 'remote-development release lifecycle check passed'
