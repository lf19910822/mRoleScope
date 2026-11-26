#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
checker="$repository_root/scripts/check-remote-development-ai-evaluation.sh"
helper="$repository_root/deploy/remote-development/mrolescope-ai-evaluation"
temporary_dir="$(mktemp -d)"

cleanup() {
  rm -rf -- "$temporary_dir"
}
trap cleanup EXIT

[[ -x "$checker" ]] || { echo 'evaluation checker is missing' >&2; exit 1; }
[[ -f "$helper" ]] || { echo 'evaluation helper is missing' >&2; exit 1; }
bash -n "$helper"

if grep -Eq 'cat[[:space:]].*SECRETS|printf.*MROLESCOPE_QWEN_API_KEY|echo.*MROLESCOPE_QWEN_API_KEY' "$helper"; then
  echo 'evaluation helper may print secret values' >&2
  exit 1
fi

"$checker" "$helper" deploy/remote-development/compose.yaml deploy/remote-development/bootstrap.sh docker/Dockerfile.worker

for variable in \
  MROLESCOPE_LANGFUSE_EVALUATION_ENABLED \
  MROLESCOPE_LANGFUSE_BASE_URL \
  MROLESCOPE_LANGFUSE_PROJECT_ID \
  MROLESCOPE_LANGFUSE_PUBLIC_KEY \
  MROLESCOPE_LANGFUSE_SECRET_KEY; do
  temporary_compose="$temporary_dir/missing-${variable}.yaml"
  cp deploy/remote-development/compose.yaml "$temporary_compose"
  sed -i "/^      ${variable}:/d" "$temporary_compose"
  if "$checker" "$helper" "$temporary_compose" deploy/remote-development/bootstrap.sh docker/Dockerfile.worker >"$temporary_dir/${variable}.log" 2>&1; then
    cat "$temporary_dir/${variable}.log" >&2
    echo "evaluation checker accepted missing ${variable}" >&2
    exit 1
  fi
done

echo 'AI evaluation Langfuse wiring regression tests passed'
