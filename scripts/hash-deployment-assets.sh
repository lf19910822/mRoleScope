#!/usr/bin/env bash
# Produce the stable, non-secret identity of the reviewed CVM deployment asset set.

set -euo pipefail

assets_dir="${1:-$(cd -- "$(dirname -- "$0")/../deploy/remote-development" && pwd)}"
[[ -d "$assets_dir" ]] || {
  printf 'deployment asset directory does not exist: %s\n' "$assets_dir" >&2
  exit 1
}

(
  cd "$assets_dir"
  find . -type f -print0 \
    | sort -z \
    | while IFS= read -r -d '' path; do
        printf '%s  %s\n' "$(sha256sum "$path" | awk '{print $1}')" "${path#./}"
      done
) | sha256sum | awk '{print $1}'
