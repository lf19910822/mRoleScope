#!/usr/bin/env bash
# Regression test for the AI deployment static checker. It uses a copied,
# deliberately incomplete bootstrap asset and never reads secrets, contacts a
# host, or starts Docker.

set -euo pipefail

readonly repository_root="$(cd -- "$(dirname -- "$0")/.." && pwd)"
readonly checker="$repository_root/scripts/check-remote-development-ai-analysis.sh"
readonly source_dir="$repository_root/deploy/remote-development"
temporary_dir="$(mktemp -d)"

cleanup() {
  rm -rf -- "$temporary_dir"
}
trap cleanup EXIT

copied_dir="$temporary_dir/remote-development"
mkdir -p "$copied_dir"
cp "$source_dir/bootstrap.sh" "$copied_dir/bootstrap.sh"
cp "$source_dir/compose.yaml" "$copied_dir/compose.yaml"
for unit in \
  mrolescope-ai-trial-prepare.service \
  mrolescope-ai-plus-trial.service \
  mrolescope-ai-flash-trial.service \
  mrolescope-ai-plus-expand.service \
  mrolescope-ai-plus-publish.service; do
  cp "$source_dir/$unit" "$copied_dir/$unit"
done

sed -i "/download_asset 'deploy\/remote-development\/mrolescope-ai-plus-trial.service'/d" \
  "$copied_dir/bootstrap.sh"

if bash "$checker" \
  "$source_dir/mrolescope-ai-analysis" \
  "$source_dir/mrolescope-operations" \
  "$repository_root/.github/workflows/remote-development-operations.yml" \
  "$copied_dir/bootstrap.sh" \
  "$source_dir/compose.yaml" >"$temporary_dir/checker.log" 2>&1; then
  cat "$temporary_dir/checker.log" >&2
  printf '%s\n' 'AI deployment checker accepted a bootstrap without the Plus trial unit download.' >&2
  exit 1
fi

cp "$source_dir/mrolescope-ai-analysis" "$copied_dir/weak-audit-operator-helper"
sed -i 's/\[\[ "${SUDO_USER:-}" == "$AUDIT_OPERATOR" \]\]/true/' "$copied_dir/weak-audit-operator-helper"

if bash "$checker" \
  "$copied_dir/weak-audit-operator-helper" \
  "$source_dir/mrolescope-operations" \
  "$repository_root/.github/workflows/remote-development-operations.yml" \
  "$source_dir/bootstrap.sh" \
  "$source_dir/compose.yaml" >"$temporary_dir/weak-audit-operator-helper.log" 2>&1; then
  cat "$temporary_dir/weak-audit-operator-helper.log" >&2
  printf '%s\n' 'AI deployment checker accepted a helper without FAN operator binding.' >&2
  exit 1
fi

sed -i '/MROLESCOPE_QWEN_WORKSPACE_ID/d' "$copied_dir/compose.yaml"

if bash "$checker" \
  "$source_dir/mrolescope-ai-analysis" \
  "$source_dir/mrolescope-operations" \
  "$repository_root/.github/workflows/remote-development-operations.yml" \
  "$source_dir/bootstrap.sh" \
  "$copied_dir/compose.yaml" >"$temporary_dir/workspace-checker.log" 2>&1; then
  cat "$temporary_dir/workspace-checker.log" >&2
  printf '%s\n' 'AI deployment checker accepted a Compose file without Workspace ID injection.' >&2
  exit 1
fi

cp "$source_dir/compose.yaml" "$copied_dir/leaky-compose.yaml"
sed -i '/^  analysis:/,/^  ai-trial-prepare:/ {
/^      SYNC_TIMEOUT:/a\
      MROLESCOPE_QWEN_WORKSPACE_ID: ${MROLESCOPE_QWEN_WORKSPACE_ID:-}
}' "$copied_dir/leaky-compose.yaml"

if bash "$checker" \
  "$source_dir/mrolescope-ai-analysis" \
  "$source_dir/mrolescope-operations" \
  "$repository_root/.github/workflows/remote-development-operations.yml" \
  "$source_dir/bootstrap.sh" \
  "$copied_dir/leaky-compose.yaml" >"$temporary_dir/leaky-workspace-checker.log" 2>&1; then
  cat "$temporary_dir/leaky-workspace-checker.log" >&2
  printf '%s\n' 'AI deployment checker accepted Workspace ID injection into deterministic analysis.' >&2
  exit 1
fi

cp "$source_dir/mrolescope-ai-analysis" "$copied_dir/weak-workspace-helper"
sed -i 's/\[\[ "$workspace_id" =~ $QWEN_WORKSPACE_ID_PATTERN \]\]/true/' "$copied_dir/weak-workspace-helper"

if bash "$checker" \
  "$copied_dir/weak-workspace-helper" \
  "$source_dir/mrolescope-operations" \
  "$repository_root/.github/workflows/remote-development-operations.yml" \
  "$source_dir/bootstrap.sh" \
  "$source_dir/compose.yaml" >"$temporary_dir/weak-workspace-helper.log" 2>&1; then
  cat "$temporary_dir/weak-workspace-helper.log" >&2
  printf '%s\n' 'AI deployment checker accepted a helper without Workspace ID format validation.' >&2
  exit 1
fi

cp "$repository_root/internal/jdanalysis/ai_qwen_client.go" "$copied_dir/legacy-qwen-client.go"
printf '%s\n' '// dashscope.aliyuncs.com must never return as a production endpoint.' >>"$copied_dir/legacy-qwen-client.go"

if bash "$checker" \
  "$source_dir/mrolescope-ai-analysis" \
  "$source_dir/mrolescope-operations" \
  "$repository_root/.github/workflows/remote-development-operations.yml" \
  "$source_dir/bootstrap.sh" \
  "$source_dir/compose.yaml" \
  "$copied_dir/legacy-qwen-client.go" >"$temporary_dir/legacy-endpoint-checker.log" 2>&1; then
  cat "$temporary_dir/legacy-endpoint-checker.log" >&2
  printf '%s\n' 'AI deployment checker accepted the legacy DashScope endpoint.' >&2
  exit 1
fi

cp "$source_dir/compose.yaml" "$copied_dir/no-review-tty-compose.yaml"
sed -i '/^  ai-plus-review:/,/^  ai-flash-review:/ {
/^    tty: true$/d
}' "$copied_dir/no-review-tty-compose.yaml"

if bash "$checker" \
  "$source_dir/mrolescope-ai-analysis" \
  "$source_dir/mrolescope-operations" \
  "$repository_root/.github/workflows/remote-development-operations.yml" \
  "$source_dir/bootstrap.sh" \
  "$copied_dir/no-review-tty-compose.yaml" >"$temporary_dir/no-review-tty.log" 2>&1; then
  cat "$temporary_dir/no-review-tty.log" >&2
  printf '%s\n' 'AI deployment checker accepted a terminal review service without TTY output.' >&2
  exit 1
fi

cp "$source_dir/mrolescope-operations" "$copied_dir/review-leaking-operations"
printf '%s\n' '# review-plus' >>"$copied_dir/review-leaking-operations"

if bash "$checker" \
  "$source_dir/mrolescope-ai-analysis" \
  "$copied_dir/review-leaking-operations" \
  "$repository_root/.github/workflows/remote-development-operations.yml" \
  "$source_dir/bootstrap.sh" \
  "$source_dir/compose.yaml" >"$temporary_dir/review-leaking-operations.log" 2>&1; then
  cat "$temporary_dir/review-leaking-operations.log" >&2
  printf '%s\n' 'AI deployment checker accepted a terminal review action exposed through operations.' >&2
  exit 1
fi

cp "$source_dir/compose.yaml" "$copied_dir/no-output-audit-diagnostic-compose.yaml"
sed -i '/^  ai-plus-output-audit-diagnostic:/,/^  ai-plus-trial:/ { /^  ai-plus-trial:/!d; }' \
  "$copied_dir/no-output-audit-diagnostic-compose.yaml"

if bash "$checker" \
  "$source_dir/mrolescope-ai-analysis" \
  "$source_dir/mrolescope-operations" \
  "$repository_root/.github/workflows/remote-development-operations.yml" \
  "$source_dir/bootstrap.sh" \
  "$copied_dir/no-output-audit-diagnostic-compose.yaml" >"$temporary_dir/no-output-audit-diagnostic.log" 2>&1; then
  cat "$temporary_dir/no-output-audit-diagnostic.log" >&2
  printf '%s\n' 'AI deployment checker accepted a missing five-body output-audit diagnostic caller.' >&2
  exit 1
fi

# compose is a shell function in the root-owned helper. `exec compose ...`
# therefore looks for an external executable named compose and breaks the
# direct-TTY audit path before it can display a captured response.
cp "$source_dir/mrolescope-ai-analysis" "$copied_dir/exec-compose-audit-helper"
sed -i 's|^    compose --profile ai-output-audit run --rm --no-deps ai-plus-contract-audit$|    exec compose --profile ai-output-audit run --rm --no-deps ai-plus-contract-audit|' \
  "$copied_dir/exec-compose-audit-helper"

if bash "$checker" \
  "$copied_dir/exec-compose-audit-helper" \
  "$source_dir/mrolescope-operations" \
  "$repository_root/.github/workflows/remote-development-operations.yml" \
  "$source_dir/bootstrap.sh" \
  "$source_dir/compose.yaml" >"$temporary_dir/exec-compose-audit-helper.log" 2>&1; then
  cat "$temporary_dir/exec-compose-audit-helper.log" >&2
  printf '%s\n' 'AI deployment checker accepted an audit helper that execs the compose shell function.' >&2
  exit 1
fi

printf '%s\n' 'AI deployment checker regression test passed'
