#!/usr/bin/env bash
# Verify that deployment asset fingerprints are stable and content/path sensitive.

set -euo pipefail

readonly repository_root="$(cd -- "$(dirname -- "$0")/.." && pwd)"
readonly hasher="$repository_root/scripts/hash-deployment-assets.sh"
temporary_dir="$(mktemp -d)"
cleanup() { rm -rf -- "$temporary_dir"; }
trap cleanup EXIT

mkdir -p "$temporary_dir/assets/nested"
printf '%s\n' alpha >"$temporary_dir/assets/a"
printf '%s\n' beta >"$temporary_dir/assets/nested/b"

first="$(bash "$hasher" "$temporary_dir/assets")"
second="$(bash "$hasher" "$temporary_dir/assets")"
[[ "$first" =~ ^[a-f0-9]{64}$ && "$first" == "$second" ]] || {
  printf '%s\n' 'fingerprint is not stable or has an invalid format' >&2
  exit 1
}

printf '%s\n' changed >"$temporary_dir/assets/a"
changed_content="$(bash "$hasher" "$temporary_dir/assets")"
[[ "$changed_content" != "$first" ]] || {
  printf '%s\n' 'content changes did not alter the fingerprint' >&2
  exit 1
}

mv "$temporary_dir/assets/nested/b" "$temporary_dir/assets/nested/c"
changed_path="$(bash "$hasher" "$temporary_dir/assets")"
[[ "$changed_path" != "$changed_content" ]] || {
  printf '%s\n' 'path changes did not alter the fingerprint' >&2
  exit 1
}

printf '%s\n' 'deployment asset fingerprint test passed'
