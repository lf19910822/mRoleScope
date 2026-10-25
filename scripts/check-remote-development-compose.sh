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

[[ "$(grep -Fxc '    networks: [backend]' "$compose_file")" -eq 26 ]] \
  || die 'only PostgreSQL, internal one-shots, fixed lifecycle services, and terminal audits may use backend alone'

grep -Fqx '    profiles: [sync]' "$compose_file" \
  || die 'worker must remain an explicitly requested one-shot profile'

grep -Fqx '    profiles: [analysis]' "$compose_file" \
  || die 'analysis must remain an explicitly requested one-shot profile'

grep -Fqx '    profiles: [ai-analysis]' "$compose_file" \
  || die 'AI analysis must remain an explicitly requested one-shot profile'

[[ "$(grep -Fxc '    profiles: [ai-review]' "$compose_file")" -eq 2 ]] \
  || die 'the two terminal review services must remain explicitly requested'

[[ "$(grep -Fxc '    profiles: [ai-output-audit]' "$compose_file")" -eq 3 ]] \
  || die 'the two terminal output-audit services and safe status reader must remain explicitly requested'

[[ "$(grep -Fxc '    profiles: [ai-output-audit-diagnostic]' "$compose_file")" -eq 2 ]] \
  || die 'the two fixed output-audit diagnostic services must remain explicitly requested'

grep -Fqx '    profiles: [backfill]' "$compose_file" \
  || die 'backfill must remain an explicitly requested one-shot profile'

grep -Fqx '    profiles: [candidate-analysis]' "$compose_file" \
  || die 'candidate analysis must remain an explicitly requested one-shot profile'

grep -Fqx '    profiles: [candidate-analysis-v0.2]' "$compose_file" \
  || die 'v0.2 candidate analysis must remain an explicitly requested one-shot profile'

[[ "$(grep -Fxc '    profiles: [release-v0.2]' "$compose_file")" -eq 3 ]] \
  || die 'the fixed v0.2 release lifecycle services must remain explicitly requested'

[[ "$(grep -Fxc '    profiles: [dictionary]' "$compose_file")" -eq 7 ]] \
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

grep -Fqx '  ai-edge: {}' "$compose_file" \
  || die 'AI egress network must be an ordinary bridge network'

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
check_private_one_shot analysis-release-v0.2 release-v0.2 '["release-v0.2", "publish"]'
check_private_one_shot analysis-withdraw-v0.2-quality release-v0.2 '["release-v0.2", "withdraw-quality"]'
check_private_one_shot analysis-withdraw-v0.2-security release-v0.2 '["release-v0.2", "withdraw-security"]'
check_private_one_shot dictionary-seed dictionary '["dictionary-v0.1", "seed", "${MROLESCOPE_DICTIONARY_OPERATOR:-}"]'
check_private_one_shot dictionary-approve dictionary '["dictionary-v0.1", "approve", "${MROLESCOPE_DICTIONARY_RULE_KEY:-}", "${MROLESCOPE_DICTIONARY_OPERATOR:-}"]'
check_private_one_shot dictionary-prepare dictionary '["dictionary-v0.1", "prepare-analysis", "${MROLESCOPE_DICTIONARY_OPERATOR:-}"]'
check_private_one_shot dictionary-v0.2-seed dictionary '["dictionary-v0.2", "seed", "${MROLESCOPE_DICTIONARY_OPERATOR:-}"]'
check_private_one_shot dictionary-v0.2-approve dictionary '["dictionary-v0.2", "approve", "${MROLESCOPE_DICTIONARY_RULE_KEY:-}", "${MROLESCOPE_DICTIONARY_OPERATOR:-}"]'
check_private_one_shot dictionary-v0.2-approve-reviewed dictionary '["dictionary-v0.2", "approve-reviewed", "${MROLESCOPE_DICTIONARY_OPERATOR:-}"]'
check_private_one_shot dictionary-v0.2-prepare dictionary '["dictionary-v0.2", "prepare-analysis", "${MROLESCOPE_DICTIONARY_OPERATOR:-}"]'
check_private_one_shot ai-output-audit-diagnostic-prepare ai-output-audit-diagnostic '["analyze-ai", "prepare-output-audit-diagnostic"]'

check_terminal_review() {
  local service="$1" model="$2" block
  block=$(service_block "$service")
  [[ -n "$block" ]] || die "${service} service was not found"
  grep -Fqx '    profiles: [ai-review]' <<<"$block" \
    || die "${service} must use only the ai-review profile"
  grep -Fqx "    command: [\"review-ai\", \"${model}\"]" <<<"$block" \
    || die "${service} must use its fixed terminal-review command"
  grep -Fqx '    networks: [backend]' <<<"$block" \
    || die "${service} must use only the internal backend network"
  grep -Fqx '    stdin_open: true' <<<"$block" \
    || die "${service} must keep standard input open for its direct-terminal session"
  grep -Fqx '    tty: true' <<<"$block" \
    || die "${service} must allocate a TTY for its direct-terminal session"
  if grep -Eq '^[[:space:]]+(MROLESCOPE_QWEN_API_KEY:|COS_|ports:|volumes:)|^[[:space:]]+networks:.*edge' <<<"$block"; then
    die "${service} must not receive the provider key, COS, an external network, a host port, or raw-data volumes"
  fi
}

check_terminal_review ai-plus-review plus
check_terminal_review ai-flash-review flash

check_output_audit_mount() {
  local service="$1" mode="$2" block volume_block
  block=$(service_block "$service")
  [[ -n "$block" ]] || die "${service} service was not found"
  volume_block=$(awk '
    /^    volumes:$/ { in_volumes = 1; next }
    in_volumes && /^    [A-Za-z][A-Za-z0-9_-]*:/ { exit }
    in_volumes { print }
  ' <<<"$block")
  [[ -n "$volume_block" ]] || die "${service} must receive its fixed audit-directory mount"
  [[ "$(grep -Ec '^      - ' <<<"$volume_block")" -eq 1 ]] \
    || die "${service} must receive exactly one fixed audit-directory mount"
  grep -Fqx '      - type: bind' <<<"$volume_block" \
    || die "${service} audit mount must be a bind mount"
  grep -Fqx '        source: /var/lib/mrolescope/ai-output-audit' <<<"$volume_block" \
    || die "${service} must mount only the fixed audit directory"
  grep -Fqx '        target: /var/lib/mrolescope/ai-output-audit' <<<"$volume_block" \
    || die "${service} audit directory target is not fixed"
  if [[ "$mode" == 'reader' ]]; then
    grep -Fqx '        read_only: true' <<<"$volume_block" \
      || die "${service} audit directory must be read-only"
  elif grep -Fqx '        read_only: true' <<<"$volume_block"; then
    die "${service} must retain its required writable audit directory"
  fi
}

check_terminal_output_audit() {
  local service="$1" model="$2" block
  block=$(service_block "$service")
  [[ -n "$block" ]] || die "${service} service was not found"
  grep -Fqx '    profiles: [ai-output-audit]' <<<"$block" \
    || die "${service} must use only the ai-output-audit profile"
  grep -Fqx "    command: [\"audit-ai\", \"${model}\"]" <<<"$block" \
    || die "${service} must use its fixed output-audit command"
  grep -Fqx '    networks: [backend]' <<<"$block" \
    || die "${service} must use only the internal backend network"
  grep -Fqx '    stdin_open: true' <<<"$block" \
    || die "${service} must keep standard input open for its direct-terminal session"
  grep -Fqx '    tty: true' <<<"$block" \
    || die "${service} must allocate a TTY for its direct-terminal session"
  check_output_audit_mount "$service" reader
  if grep -Eq '^[[:space:]]+(MROLESCOPE_QWEN_API_KEY:|COS_|ports:)|^[[:space:]]+networks:.*edge' <<<"$block"; then
    die "${service} must not receive the provider key, COS, an external network, or a host port"
  fi
}

check_terminal_output_audit ai-plus-contract-audit plus
check_terminal_output_audit ai-flash-contract-audit flash

check_output_audit_status_reader() {
  local service='ai-output-audit-status' block
  block=$(service_block "$service")
  [[ -n "$block" ]] || die "${service} service was not found"
  grep -Fqx '    profiles: [ai-output-audit]' <<<"$block" \
    || die "${service} must use only the ai-output-audit profile"
  grep -Fqx '    command: ["audit-ai", "status"]' <<<"$block" \
    || die "${service} must use its fixed safe status command"
  grep -Fqx '    networks: [backend]' <<<"$block" \
    || die "${service} must use only the internal backend network"
  check_output_audit_mount "$service" reader
  if grep -Eq '^[[:space:]]+(MROLESCOPE_QWEN_API_KEY:|COS_|ports:)|^[[:space:]]+networks:.*edge' <<<"$block"; then
    die "${service} must not receive the provider key, COS, an external network, or a host port"
  fi
}

check_output_audit_status_reader

check_output_audit_diagnostic_runner() {
  local service='ai-plus-output-audit-diagnostic' block
  block=$(service_block "$service")
  [[ -n "$block" ]] || die "${service} service was not found"
  grep -Fqx '    profiles: [ai-output-audit-diagnostic]' <<<"$block" \
    || die "${service} must use only the ai-output-audit-diagnostic profile"
  grep -Fqx '    command: ["analyze-ai", "run-plus-output-audit-diagnostic"]' <<<"$block" \
    || die "${service} must use its fixed five-body Plus command"
  grep -Fqx '    networks: [backend, ai-edge]' <<<"$block" \
    || die "${service} must use only backend and the dedicated provider egress network"
  grep -Fqx '      MROLESCOPE_QWEN_API_KEY: ${MROLESCOPE_QWEN_API_KEY:-}' <<<"$block" \
    || die "${service} must receive its key only through the root-owned env-file variable"
  check_output_audit_mount "$service" writer
  if grep -Eq '^[[:space:]]+(COS_|ports:)' <<<"$block"; then
    die "${service} must not receive COS or a host port"
  fi
}

check_output_audit_diagnostic_runner

for service in ai-plus-trial ai-flash-trial; do
  ai_block=$(service_block "$service")
  [[ -n "$ai_block" ]] || die "${service} service was not found"
  grep -Fqx '    profiles: [ai-analysis]' <<<"$ai_block" \
    || die "${service} must use only the ai-analysis profile"
  grep -Fqx '    networks: [backend, ai-edge]' <<<"$ai_block" \
    || die "${service} must use only backend and its dedicated egress network"
  grep -Fqx '      MROLESCOPE_QWEN_API_KEY: ${MROLESCOPE_QWEN_API_KEY:-}' <<<"$ai_block" \
    || die "${service} must receive its key only through the root-owned env-file variable"
  check_output_audit_mount "$service" writer
  if grep -Eq '^[[:space:]]+(COS_|ports:)' <<<"$ai_block"; then
    die "${service} must not receive COS, a host port, raw-data volumes, or the ordinary edge network"
  fi
done
[[ "$(grep -Fxc '      MROLESCOPE_QWEN_API_KEY: ${MROLESCOPE_QWEN_API_KEY:-}' "$compose_file")" -eq 3 ]] \
  || die 'the Qwen key variable must appear only in the two fixed trials and five-body diagnostic caller'
