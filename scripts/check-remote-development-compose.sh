#!/usr/bin/env bash
# Guard the network boundary that lets the host reach only the API. Docker
# cannot publish a port for a container attached exclusively to an internal
# bridge network, so the API also needs the ordinary `edge` bridge.

set -euo pipefail

readonly compose_file="${1:-deploy/remote-development/compose.yaml}"

die() {
  printf 'remote-development Compose check failed: %s\n' "$1" >&2
  exit 1
}

[[ -f "$compose_file" ]] || die "Compose file does not exist: $compose_file"

grep -Fqx '    networks: [backend, edge]' "$compose_file" \
  || die 'API must be attached to backend and edge networks'

[[ "$(grep -Fxc '    networks: [backend]' "$compose_file")" -eq 12 ]] \
  || die 'only PostgreSQL, migration, analysis, backfill, and fixed dictionary services may use backend alone'

grep -Fqx '    profiles: [sync]' "$compose_file" \
  || die 'worker must remain an explicitly requested one-shot profile'

grep -Fqx '    profiles: [analysis]' "$compose_file" \
  || die 'analysis must remain an explicitly requested one-shot profile'

grep -Fqx '    profiles: [backfill]' "$compose_file" \
  || die 'backfill must remain an explicitly requested one-shot profile'

grep -Fqx '    profiles: [candidate-analysis]' "$compose_file" \
  || die 'candidate analysis must remain an explicitly requested one-shot profile'

grep -Fqx '    profiles: [candidate-analysis-v0.2]' "$compose_file" \
  || die 'v0.2 candidate analysis must remain an explicitly requested one-shot profile'

[[ "$(grep -Fxc '    profiles: [dictionary]' "$compose_file")" -eq 6 ]] \
  || die 'the fixed dictionary lifecycle services must remain explicitly requested'

[[ "$(grep -Fxc '    networks: [backend, edge]' "$compose_file")" -eq 2 ]] \
  || die 'only API and no-port worker may attach to the ordinary edge network'

[[ "$(grep -Fxc '    ports:' "$compose_file")" -eq 1 ]] \
  || die 'only API may publish a host port'

grep -Fqx '      - "127.0.0.1:8080:8080"' "$compose_file" \
  || die 'API must publish port 8080 only on the host loopback address'

grep -Fqx '    internal: true' "$compose_file" \
  || die 'backend network must remain internal'

grep -Fqx '  edge: {}' "$compose_file" \
  || die 'edge network must be an ordinary bridge network'

service_block() {
  local service="$1"
  awk -v service="$service" '
    $0 == "  " service ":" { in_block = 1; next }
    in_block && /^  [A-Za-z0-9][A-Za-z0-9-]*:$/ { exit }
    in_block { print }
  ' "$compose_file"
}

check_private_one_shot() {
  local service="$1" profile="$2" command="$3" block
  block=$(service_block "$service")
  [[ -n "$block" ]] || die "${service} service was not found"
  grep -Fqx "    profiles: [${profile}]" <<<"$block" \
    || die "${service} must use only the ${profile} profile"
  grep -Fqx "    command: ${command}" <<<"$block" \
    || die "${service} must use its fixed worker command"
  grep -Fqx '    networks: [backend]' <<<"$block" \
    || die "${service} must use only the internal backend network"
  if grep -Eq '^[[:space:]]+(COS_|ports:|volumes:)|^[[:space:]]+networks:.*edge' <<<"$block"; then
    die "${service} must not receive COS, edge-network, ports, or volumes"
  fi
}

check_private_one_shot analysis analysis '["analyze"]'
check_private_one_shot backfill backfill '["backfill-versioned-content"]'
check_private_one_shot candidate-analysis candidate-analysis '["analyze-candidate-v0.1"]'
check_private_one_shot candidate-analysis-v0.2 candidate-analysis-v0.2 '["analyze-candidate-v0.2"]'
check_private_one_shot dictionary-seed dictionary '["dictionary-v0.1", "seed", "${MROLESCOPE_DICTIONARY_OPERATOR:-}"]'
check_private_one_shot dictionary-approve dictionary '["dictionary-v0.1", "approve", "${MROLESCOPE_DICTIONARY_RULE_KEY:-}", "${MROLESCOPE_DICTIONARY_OPERATOR:-}"]'
check_private_one_shot dictionary-prepare dictionary '["dictionary-v0.1", "prepare-analysis", "${MROLESCOPE_DICTIONARY_OPERATOR:-}"]'
check_private_one_shot dictionary-v0.2-seed dictionary '["dictionary-v0.2", "seed", "${MROLESCOPE_DICTIONARY_OPERATOR:-}"]'
check_private_one_shot dictionary-v0.2-approve dictionary '["dictionary-v0.2", "approve", "${MROLESCOPE_DICTIONARY_RULE_KEY:-}", "${MROLESCOPE_DICTIONARY_OPERATOR:-}"]'
check_private_one_shot dictionary-v0.2-prepare dictionary '["dictionary-v0.2", "prepare-analysis", "${MROLESCOPE_DICTIONARY_OPERATOR:-}"]'
