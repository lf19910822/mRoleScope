#!/usr/bin/env bash
# Verify the AI semantic-analysis control plane accepts only reviewed fixed
# operations. This is a static, synthetic check: it never reads a secret,
# connects to CVM, starts Docker, or sends model traffic.

set -euo pipefail

readonly helper="${1:-deploy/remote-development/mrolescope-ai-analysis}"
readonly operations="${2:-deploy/remote-development/mrolescope-operations}"
readonly workflow="${3:-.github/workflows/remote-development-operations.yml}"
readonly bootstrap="${4:-deploy/remote-development/bootstrap.sh}"
readonly compose_file="${5:-deploy/remote-development/compose.yaml}"
readonly repository_root="$(cd -- "$(dirname -- "$0")/.." && pwd)"
readonly qwen_client="${6:-$repository_root/internal/jdanalysis/ai_qwen_client.go}"
readonly unit_directory="$(dirname "$bootstrap")"

die() {
  printf 'remote-development AI analysis check failed: %s\n' "$1" >&2
  exit 1
}

for path in "$helper" "$operations" "$workflow" "$bootstrap" "$compose_file"; do
  [[ -f "$path" ]] || die "required file does not exist: ${path}"
done
[[ -f "$qwen_client" ]] || die "Qwen client source does not exist: ${qwen_client}"
bash -n "$helper" "$operations" "$bootstrap"

if grep -Fq 'dashscope.aliyuncs.com' "$qwen_client"; then
  die 'Qwen client must not use the legacy DashScope endpoint'
fi
grep -Fq 'qwenBeijingWorkspaceCompatibleSuffix = ".cn-beijing.maas.aliyuncs.com"' "$qwen_client" \
  || die 'Qwen client must use the fixed Beijing Workspace endpoint suffix'

grep -Fqx "  || die 'usage: mrolescope-ai-analysis <prepare-trial|prepare-output-audit-diagnostic|run-plus-trial|run-plus-output-audit-diagnostic|expand-plus|publish-plus|review-plus|audit-plus-contract|audit-status|status|logs>'" "$helper" \
  || die 'AI helper must expose only fixed AI lifecycle operations'
grep -Fqx "readonly QWEN_WORKSPACE_ID_PATTERN='^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'" "$helper" \
  || die 'AI helper must define the reviewed Workspace ID format'
grep -Fqx "readonly AUDIT_OPERATOR='ubuntu'" "$helper" \
  || die 'AI helper must bind FAN audit access to the approved CVM operator'
grep -Fqx '  [[ "$workspace_id" =~ $QWEN_WORKSPACE_ID_PATTERN ]] || die '\''AI model workspace is not configured'\''' "$helper" \
  || die 'AI helper must reject malformed Workspace IDs before Compose starts'
grep -Fqx '  [[ "${SUDO_USER:-}" == "$AUDIT_OPERATOR" ]] || die '\''AI output audit is restricted to FAN direct terminal'\''' "$helper" \
  || die 'AI helper must require the approved FAN operator for raw-output audit'
[[ "$(grep -Fc '    require_audit_operator' "$helper")" -eq 1 ]] \
  || die 'the Plus terminal audit operation must require the approved FAN operator'
for action in prepare-trial prepare-output-audit-diagnostic run-plus-trial run-plus-output-audit-diagnostic expand-plus publish-plus review-plus audit-plus-contract audit-status status; do
  grep -Fqx "  ${action})" "$helper" \
    || die "AI helper must allowlist ${action}"
done
grep -Fqx "if [[ \"\$1\" == 'logs' ]]; then" "$helper" \
  || die 'AI helper must handle logs as a fixed journal-only operation'
for service in ai-trial-prepare ai-plus-trial ai-plus-expand ai-plus-publish ai-status; do
  grep -Fqx "    compose --profile ai-analysis run --rm --no-deps ${service}" "$helper" \
    || die "AI helper must invoke only fixed ${service} Compose service"
done

grep -Fqx '    compose --profile ai-output-audit-diagnostic run --rm --no-deps ai-output-audit-diagnostic-prepare' "$helper" \
  || die 'AI helper must invoke only the fixed diagnostic preparation service'
grep -Fqx '    compose --profile ai-output-audit-diagnostic run --rm --no-deps ai-plus-output-audit-diagnostic' "$helper" \
  || die 'AI helper must invoke only the fixed five-body diagnostic caller'
grep -Fqx '    compose --profile ai-output-audit run --rm --no-deps ai-output-audit-status' "$helper" \
  || die 'AI helper must invoke only the fixed text-free output-audit status reader'
grep -Fqx '    compose --profile ai-review run --rm --no-deps ai-plus-review' "$helper" \
  || die 'AI helper must invoke the terminal Plus review through its compose shell function'
grep -Fqx '    compose --profile ai-output-audit run --rm --no-deps ai-plus-contract-audit' "$helper" \
  || die 'AI helper must invoke the terminal Plus audit through its compose shell function'
if grep -Fq 'exec compose' "$helper"; then
  die 'AI helper must not exec the compose shell function'
fi

for operation in prepare-ai-trial run-ai-plus-trial expand-ai-plus publish-ai-plus ai-status ai-logs; do
  grep -Fqx "  ${operation})" "$operations" \
    || die "operations helper must allowlist ${operation}"
  grep -Fqx "          - ${operation}" "$workflow" \
    || die "workflow must expose fixed ${operation} choice"
done
for review_action in review-plus audit-plus-contract audit-status prepare-output-audit-diagnostic run-plus-output-audit-diagnostic; do
  if grep -Fq "$review_action" "$operations" || grep -Fq "$review_action" "$workflow"; then
    die "terminal-only ${review_action} must not be exposed through operations or GitHub Actions"
  fi
done
for confirmation in PREPARE_AI_TRIAL RUN_AI_PLUS_TRIAL EXPAND_AI_PLUS PUBLISH_AI_PLUS; do
  grep -Fqx "              expected='${confirmation}'" "$workflow" \
    || die "workflow must require ${confirmation} confirmation"
done

grep -Fqx "readonly AI_ANALYSIS_HELPER_DESTINATION='/usr/local/sbin/mrolescope-ai-analysis'" "$bootstrap" \
  || die 'bootstrap must install the root-owned AI helper'
grep -Fqx "download_asset 'deploy/remote-development/mrolescope-ai-analysis' \"\$staging_dir/mrolescope-ai-analysis\"" "$bootstrap" \
  || die 'bootstrap must stage the AI helper before installation'

check_ai_systemd_unit() {
  local unit_name="$1" helper_operation="$2" destination_variable="$3" expected_write_paths="$4" unit_path
  unit_path="${unit_directory}/${unit_name}.service"
  [[ -f "$unit_path" ]] || die "required AI systemd unit does not exist: ${unit_path}"
  grep -Fqx 'Type=oneshot' "$unit_path" \
    || die "${unit_name} must be a one-shot systemd unit"
  grep -Fqx "ExecStart=/usr/local/sbin/mrolescope-ai-analysis ${helper_operation}" "$unit_path" \
    || die "${unit_name} must invoke only its fixed AI helper operation"
  for directive in \
    'NoNewPrivileges=true' \
    'PrivateTmp=true' \
    'ProtectHome=true' \
    'ProtectSystem=strict' \
    "$expected_write_paths"; do
    grep -Fqx "$directive" "$unit_path" \
      || die "${unit_name} must retain systemd hardening: ${directive}"
  done

  grep -Fqx "readonly ${destination_variable}='/etc/systemd/system/${unit_name}.service'" "$bootstrap" \
    || die "bootstrap must define the ${unit_name} installation destination"
  grep -Fqx "download_asset 'deploy/remote-development/${unit_name}.service' \"\$staging_dir/${unit_name}.service\"" "$bootstrap" \
    || die "bootstrap must stage ${unit_name}"
  grep -Fqx "install -o root -g root -m 0644 \"\$staging_dir/${unit_name}.service\" \"\$${destination_variable}\"" "$bootstrap" \
    || die "bootstrap must install ${unit_name} root-owned with mode 0644"
}

check_ai_systemd_unit mrolescope-ai-trial-prepare prepare-trial AI_TRIAL_PREPARE_SERVICE_DESTINATION 'ReadWritePaths=/var/lib/mrolescope/releases'
check_ai_systemd_unit mrolescope-ai-plus-trial run-plus-trial AI_PLUS_TRIAL_SERVICE_DESTINATION 'ReadWritePaths=/var/lib/mrolescope/releases /var/lib/mrolescope/ai-output-audit'
check_ai_systemd_unit mrolescope-ai-plus-expand expand-plus AI_PLUS_EXPAND_SERVICE_DESTINATION 'ReadWritePaths=/var/lib/mrolescope/releases'
check_ai_systemd_unit mrolescope-ai-plus-publish publish-plus AI_PLUS_PUBLISH_SERVICE_DESTINATION 'ReadWritePaths=/var/lib/mrolescope/releases'
[[ ! -e "$unit_directory/mrolescope-ai-flash-trial.service" ]] \
  || die 'a future Flash trial systemd unit must not be shipped'
grep -Fqx 'systemctl disable --now mrolescope-ai-flash-trial.service >/dev/null 2>&1 || true' "$bootstrap" \
  || die 'bootstrap must retire the obsolete Flash trial unit without touching history'
grep -Fqx 'rm -f /etc/systemd/system/mrolescope-ai-flash-trial.service' "$bootstrap" \
  || die 'bootstrap must remove only the obsolete Flash trial unit definition'

grep -Fqx "readonly OUTPUT_AUDIT_DIR='/var/lib/mrolescope/ai-output-audit'" "$bootstrap" \
  || die 'bootstrap must define the root-only AI output audit directory'
grep -Fqx 'install -d -o root -g root -m 0700 "$SECRETS_DIR" "$RELEASE_DIR" "$OUTPUT_AUDIT_DIR"' "$bootstrap" \
  || die 'bootstrap must create the root-only AI output audit directory'

service_block() {
  local service="$1"
  awk -v service="$service" '
    $0 == "  " service ":" { in_block = 1; next }
    in_block && /^  [A-Za-z0-9][A-Za-z0-9-]*:$/ { exit }
    in_block { print }
  ' "$compose_file"
}

for service in postgres migrate api worker analysis candidate-analysis dictionary-seed dictionary-approve dictionary-prepare backfill; do
  block=$(service_block "$service")
  [[ -n "$block" ]] || die "${service} Compose service was not found"
  if grep -Fq 'MROLESCOPE_QWEN_WORKSPACE_ID' <<<"$block"; then
    die "${service} must not receive the AI Workspace ID"
  fi
done

for service in ai-trial-prepare ai-plus-expand ai-plus-publish ai-status; do
  block=$(service_block "$service")
  [[ -n "$block" ]] || die "${service} Compose service was not found"
  grep -Fqx '    profiles: [ai-analysis]' <<<"$block" \
    || die "${service} must use only the ai-analysis profile"
  grep -Fqx '    networks: [backend]' <<<"$block" \
    || die "${service} must use only the internal backend network"
  grep -Fqx '      MROLESCOPE_QWEN_WORKSPACE_ID: ${MROLESCOPE_QWEN_WORKSPACE_ID:-}' <<<"$block" \
    || die "${service} must receive the Workspace ID through the root-owned env-file variable"
  if grep -Eq '^[[:space:]]+(COS_|ports:|volumes:)|^[[:space:]]+networks:.*edge|MROLESCOPE_QWEN_API_KEY' <<<"$block"; then
    die "${service} must not receive a provider key, COS, edge-network, ports, or volumes"
  fi
done

block=$(service_block ai-output-audit-diagnostic-prepare)
[[ -n "$block" ]] || die 'ai-output-audit-diagnostic-prepare Compose service was not found'
grep -Fqx '    profiles: [ai-output-audit-diagnostic]' <<<"$block" \
  || die 'diagnostic preparation must use only the dedicated profile'
grep -Fqx '    command: ["analyze-ai", "prepare-output-audit-diagnostic"]' <<<"$block" \
  || die 'diagnostic preparation must use only its fixed worker command'
grep -Fqx '    networks: [backend]' <<<"$block" \
  || die 'diagnostic preparation must use only the internal backend network'
grep -Fqx '      MROLESCOPE_QWEN_WORKSPACE_ID: ${MROLESCOPE_QWEN_WORKSPACE_ID:-}' <<<"$block" \
  || die 'diagnostic preparation must receive the Workspace ID through the root-owned env-file variable'
if grep -Eq '^[[:space:]]+(COS_|ports:|volumes:)|^[[:space:]]+networks:.*edge|MROLESCOPE_QWEN_API_KEY' <<<"$block"; then
  die 'diagnostic preparation must not receive a provider key, COS, edge-network, ports, or volumes'
fi

for service in ai-plus-trial; do
  block=$(service_block "$service")
  [[ -n "$block" ]] || die "${service} Compose service was not found"
  grep -Fqx '    profiles: [ai-analysis]' <<<"$block" \
    || die "${service} must use only the ai-analysis profile"
  grep -Fqx '    networks: [backend, ai-edge]' <<<"$block" \
    || die "${service} must use only backend and its dedicated provider egress network"
  grep -Fqx '      MROLESCOPE_QWEN_API_KEY: ${MROLESCOPE_QWEN_API_KEY:-}' <<<"$block" \
    || die "${service} must receive the key only through the root-owned env-file variable"
  grep -Fqx '      MROLESCOPE_QWEN_WORKSPACE_ID: ${MROLESCOPE_QWEN_WORKSPACE_ID:-}' <<<"$block" \
    || die "${service} must receive the Workspace ID through the root-owned env-file variable"
  grep -Fqx '    volumes:' <<<"$block" \
    || die "${service} must receive only its fixed AI-output audit mount"
  grep -Fqx '        source: /var/lib/mrolescope/ai-output-audit' <<<"$block" \
    || die "${service} must mount only the fixed root-only AI-output audit directory"
  grep -Fqx '        target: /var/lib/mrolescope/ai-output-audit' <<<"$block" \
    || die "${service} must mount the fixed audit target"
  if grep -Eq '^[[:space:]]+(COS_|ports:)' <<<"$block"; then
    die "${service} must not receive COS or a host port"
  fi
done

block=$(service_block ai-plus-output-audit-diagnostic)
[[ -n "$block" ]] || die 'ai-plus-output-audit-diagnostic Compose service was not found'
grep -Fqx '    profiles: [ai-output-audit-diagnostic]' <<<"$block" \
  || die 'five-body diagnostic caller must use only the dedicated profile'
grep -Fqx '    command: ["analyze-ai", "run-plus-output-audit-diagnostic"]' <<<"$block" \
  || die 'five-body diagnostic caller must use only its fixed worker command'
grep -Fqx '    networks: [backend, ai-edge]' <<<"$block" \
  || die 'five-body diagnostic caller must use only backend and the dedicated provider egress network'
grep -Fqx '      MROLESCOPE_QWEN_API_KEY: ${MROLESCOPE_QWEN_API_KEY:-}' <<<"$block" \
  || die 'five-body diagnostic caller must receive the key only through the root-owned env-file variable'
grep -Fqx '      MROLESCOPE_QWEN_WORKSPACE_ID: ${MROLESCOPE_QWEN_WORKSPACE_ID:-}' <<<"$block" \
  || die 'five-body diagnostic caller must receive the Workspace ID through the root-owned env-file variable'
grep -Fqx '        source: /var/lib/mrolescope/ai-output-audit' <<<"$block" \
  || die 'five-body diagnostic caller must mount only the fixed root-only AI-output audit directory'
grep -Fqx '        target: /var/lib/mrolescope/ai-output-audit' <<<"$block" \
  || die 'five-body diagnostic caller must mount the fixed audit target'
if grep -Eq '^[[:space:]]+(COS_|ports:)' <<<"$block"; then
  die 'five-body diagnostic caller must not receive COS or a host port'
fi

for service in ai-plus-review; do
  block=$(service_block "$service")
  [[ -n "$block" ]] || die "${service} Compose service was not found"
  grep -Fqx '    profiles: [ai-review]' <<<"$block" \
    || die "${service} must use only the ai-review profile"
  grep -Fqx '    networks: [backend]' <<<"$block" \
    || die "${service} must use only the internal backend network"
  grep -Fqx '    stdin_open: true' <<<"$block" \
    || die "${service} must preserve direct-terminal input"
  grep -Fqx '    tty: true' <<<"$block" \
    || die "${service} must preserve direct-terminal output"
  grep -Fqx '      MROLESCOPE_QWEN_WORKSPACE_ID: ${MROLESCOPE_QWEN_WORKSPACE_ID:-}' <<<"$block" \
    || die "${service} must receive the Workspace ID through the root-owned env-file variable"
  if grep -Eq '^[[:space:]]+(COS_|ports:|volumes:)|^[[:space:]]+networks:.*edge|MROLESCOPE_QWEN_API_KEY' <<<"$block"; then
    die "${service} must not receive a provider key, COS, edge-network, ports, or volumes"
  fi
done

check_output_audit_terminal() {
  local service="$1" model="$2" block
  block=$(service_block "$service")
  [[ -n "$block" ]] || die "${service} service was not found"
  grep -Fqx '    profiles: [ai-output-audit]' <<<"$block" \
    || die "${service} must use the AI-output-audit profile"
  grep -Fqx "    command: [\"audit-ai\", \"${model}\"]" <<<"$block" \
    || die "${service} must run only its fixed audit worker command"
  grep -Fqx '    networks: [backend]' <<<"$block" \
    || die "${service} must use only the internal backend network"
  grep -Fqx '    stdin_open: true' <<<"$block" \
    || die "${service} must retain direct-terminal input"
  grep -Fqx '    tty: true' <<<"$block" \
    || die "${service} must retain direct-terminal output"
  grep -Fqx '        source: /var/lib/mrolescope/ai-output-audit' <<<"$block" \
    || die "${service} must mount the fixed output-audit directory"
  grep -Fqx '        target: /var/lib/mrolescope/ai-output-audit' <<<"$block" \
    || die "${service} must use the fixed output-audit target"
  grep -Fqx '        read_only: true' <<<"$block" \
    || die "${service} must mount output-audit data read-only"
  if grep -Eq '^[[:space:]]+(MROLESCOPE_QWEN_API_KEY:|COS_|ports:)|^[[:space:]]+networks:.*edge' <<<"$block"; then
    die "${service} must not receive a provider key, COS, an external network, or a host port"
  fi
}

check_output_audit_terminal ai-plus-contract-audit plus

# Future execution assets are Plus-only. Bootstrap's two explicit retirement
# commands above are intentionally the only Flash references in this check.
for path in "$helper" "$operations" "$workflow" "$compose_file"; do
  if grep -Eiq 'flash' "$path"; then
    die "future AI execution asset must not expose Flash: ${path}"
  fi
done

block=$(service_block ai-output-audit-status)
[[ -n "$block" ]] || die 'ai-output-audit-status Compose service was not found'
grep -Fqx '    profiles: [ai-output-audit]' <<<"$block" \
  || die 'output-audit status must use the AI-output-audit profile'
grep -Fqx '    command: ["audit-ai", "status"]' <<<"$block" \
  || die 'output-audit status must run only its fixed worker command'
grep -Fqx '    networks: [backend]' <<<"$block" \
  || die 'output-audit status must use only the internal backend network'
grep -Fqx '        source: /var/lib/mrolescope/ai-output-audit' <<<"$block" \
  || die 'output-audit status must mount the fixed output-audit directory'
grep -Fqx '        target: /var/lib/mrolescope/ai-output-audit' <<<"$block" \
  || die 'output-audit status must use the fixed output-audit target'
grep -Fqx '        read_only: true' <<<"$block" \
  || die 'output-audit status must mount output-audit data read-only'
if grep -Eq '^[[:space:]]+(MROLESCOPE_QWEN_API_KEY:|COS_|ports:)|^[[:space:]]+networks:.*edge|^[[:space:]]+(stdin_open:|tty:)' <<<"$block"; then
  die 'output-audit status must not receive a provider key, COS, external network, port, or terminal session'
fi

printf '%s\n' 'remote-development AI analysis check passed'
