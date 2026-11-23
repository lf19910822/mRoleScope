#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
checker="$repository_root/scripts/check-remote-development-ai-evaluation.sh"
helper="$repository_root/deploy/remote-development/mrolescope-ai-evaluation"

[[ -x "$checker" ]] || { echo 'evaluation checker is missing' >&2; exit 1; }
[[ -f "$helper" ]] || { echo 'evaluation helper is missing' >&2; exit 1; }
bash -n "$helper"

if grep -Eq 'cat[[:space:]].*SECRETS|printf.*MROLESCOPE_QWEN_API_KEY|echo.*MROLESCOPE_QWEN_API_KEY' "$helper"; then
  echo 'evaluation helper may print secret values' >&2
  exit 1
fi

"$checker" "$helper" deploy/remote-development/compose.yaml deploy/remote-development/bootstrap.sh docker/Dockerfile.worker
