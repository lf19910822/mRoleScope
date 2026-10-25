#!/usr/bin/env bash
# Regression tests for the Compose network-boundary checker. Each case starts
# from the tracked Compose asset, mutates one review-session boundary, and
# confirms the checker rejects it. No Docker daemon, host, or secret is used.

set -euo pipefail

readonly repository_root="$(cd -- "$(dirname -- "$0")/.." && pwd)"
readonly checker="$repository_root/scripts/check-remote-development-compose.sh"
readonly source_compose="$repository_root/deploy/remote-development/compose.yaml"
temporary_dir="$(mktemp -d)"

cleanup() {
  rm -rf -- "$temporary_dir"
}
trap cleanup EXIT

expect_rejected() {
  local name="$1" compose_file="$2"

  if bash "$checker" "$compose_file" >"$temporary_dir/${name}.log" 2>&1; then
    cat "$temporary_dir/${name}.log" >&2
    printf 'Compose checker accepted an invalid %s review-service boundary.\n' "$name" >&2
    exit 1
  fi
}

# This is the original failure mode: the complete, valid configuration must
# pass after both terminal-only review services are added.
bash "$checker" "$source_compose"

cp "$source_compose" "$temporary_dir/wrong-profile.yaml"
sed -i '/^  ai-plus-review:/,/^  ai-flash-review:/ s/    profiles: \[ai-review\]/    profiles: [ai-analysis]/' \
  "$temporary_dir/wrong-profile.yaml"
expect_rejected wrong-profile "$temporary_dir/wrong-profile.yaml"

cp "$source_compose" "$temporary_dir/wrong-command.yaml"
sed -i '/^  ai-plus-review:/,/^  ai-flash-review:/ s/    command: \["review-ai", "plus"\]/    command: ["analyze-ai", "run-plus-trial"]/' \
  "$temporary_dir/wrong-command.yaml"
expect_rejected wrong-command "$temporary_dir/wrong-command.yaml"

cp "$source_compose" "$temporary_dir/no-stdin.yaml"
sed -i '/^  ai-plus-review:/,/^  ai-flash-review:/ { /^    stdin_open: true$/d; }' \
  "$temporary_dir/no-stdin.yaml"
expect_rejected no-stdin "$temporary_dir/no-stdin.yaml"

cp "$source_compose" "$temporary_dir/no-tty.yaml"
sed -i '/^  ai-plus-review:/,/^  ai-flash-review:/ { /^    tty: true$/d; }' \
  "$temporary_dir/no-tty.yaml"
expect_rejected no-tty "$temporary_dir/no-tty.yaml"

cp "$source_compose" "$temporary_dir/key-leak.yaml"
sed -i '/^  ai-plus-review:/,/^  ai-flash-review:/ s/      MROLESCOPE_QWEN_WORKSPACE_ID:.*/&\
      MROLESCOPE_QWEN_API_KEY: ${MROLESCOPE_QWEN_API_KEY:-}/' \
  "$temporary_dir/key-leak.yaml"
expect_rejected key-leak "$temporary_dir/key-leak.yaml"

cp "$source_compose" "$temporary_dir/audit-writable.yaml"
sed -i '/^  ai-plus-contract-audit:/,/^  ai-flash-contract-audit:/ { /^        read_only: true$/d; }' \
  "$temporary_dir/audit-writable.yaml"
expect_rejected audit-writable "$temporary_dir/audit-writable.yaml"

cp "$source_compose" "$temporary_dir/missing-output-audit-diagnostic.yaml"
sed -i '/^  ai-output-audit-diagnostic-prepare:/,/^  ai-plus-output-audit-diagnostic:/d' \
  "$temporary_dir/missing-output-audit-diagnostic.yaml"
expect_rejected missing-output-audit-diagnostic "$temporary_dir/missing-output-audit-diagnostic.yaml"

printf '%s\n' 'Compose checker review-session regression test passed'
