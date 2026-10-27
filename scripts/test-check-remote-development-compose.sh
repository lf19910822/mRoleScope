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

# The complete, valid configuration must pass with its Plus-only direct
# terminal review and output-audit boundaries.
bash "$checker" "$source_compose"

cp "$source_compose" "$temporary_dir/wrong-profile.yaml"
sed -i '/^  ai-plus-review:/,/^  ai-plus-contract-audit:/ s/    profiles: \[ai-review\]/    profiles: [ai-analysis]/' \
  "$temporary_dir/wrong-profile.yaml"
expect_rejected wrong-profile "$temporary_dir/wrong-profile.yaml"

cp "$source_compose" "$temporary_dir/wrong-command.yaml"
sed -i '/^  ai-plus-review:/,/^  ai-plus-contract-audit:/ s/    command: \["review-ai", "plus"\]/    command: ["analyze-ai", "run-plus-trial"]/' \
  "$temporary_dir/wrong-command.yaml"
expect_rejected wrong-command "$temporary_dir/wrong-command.yaml"

cp "$source_compose" "$temporary_dir/no-stdin.yaml"
sed -i '/^  ai-plus-review:/,/^  ai-plus-contract-audit:/ { /^    stdin_open: true$/d; }' \
  "$temporary_dir/no-stdin.yaml"
expect_rejected no-stdin "$temporary_dir/no-stdin.yaml"

cp "$source_compose" "$temporary_dir/no-tty.yaml"
sed -i '/^  ai-plus-review:/,/^  ai-plus-contract-audit:/ { /^    tty: true$/d; }' \
  "$temporary_dir/no-tty.yaml"
expect_rejected no-tty "$temporary_dir/no-tty.yaml"

cp "$source_compose" "$temporary_dir/key-leak.yaml"
sed -i '/^  ai-plus-review:/,/^  ai-plus-contract-audit:/ s/      MROLESCOPE_QWEN_WORKSPACE_ID:.*/&\
      MROLESCOPE_QWEN_API_KEY: ${MROLESCOPE_QWEN_API_KEY:-}/' \
  "$temporary_dir/key-leak.yaml"
expect_rejected key-leak "$temporary_dir/key-leak.yaml"

cp "$source_compose" "$temporary_dir/audit-writable.yaml"
sed -i '/^  ai-plus-contract-audit:/,/^  ai-output-audit-status:/ { /^        read_only: true$/d; }' \
  "$temporary_dir/audit-writable.yaml"
expect_rejected audit-writable "$temporary_dir/audit-writable.yaml"

# Root-owned, mode-0700 audit artifacts are intentionally inaccessible to the
# image default user. Every fixed reader/writer must therefore explicitly run
# as container root; removing that identity would turn a safe storage failure
# into a zero-provider-call diagnostic failure.
cp "$source_compose" "$temporary_dir/audit-reader-not-root.yaml"
sed -i '/^  ai-plus-contract-audit:/,/^  ai-output-audit-status:/ { /^    user: "0:0"$/d; }' \
  "$temporary_dir/audit-reader-not-root.yaml"
expect_rejected audit-reader-not-root "$temporary_dir/audit-reader-not-root.yaml"

cp "$source_compose" "$temporary_dir/audit-writer-not-root.yaml"
sed -i '/^  ai-plus-output-audit-diagnostic:/,/^  ai-plus-trial:/ { /^    user: "0:0"$/d; }' \
  "$temporary_dir/audit-writer-not-root.yaml"
expect_rejected audit-writer-not-root "$temporary_dir/audit-writer-not-root.yaml"

cp "$source_compose" "$temporary_dir/missing-output-audit-diagnostic.yaml"
sed -i '/^  ai-output-audit-diagnostic-prepare:/,/^  ai-plus-output-audit-diagnostic:/d' \
  "$temporary_dir/missing-output-audit-diagnostic.yaml"
expect_rejected missing-output-audit-diagnostic "$temporary_dir/missing-output-audit-diagnostic.yaml"

printf '%s\n' 'Compose checker review-session regression test passed'
