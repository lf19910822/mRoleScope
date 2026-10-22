#!/usr/bin/env bash
# The sync helper reads lower-case image references from the root-owned release
# record, while Compose resolves the corresponding MROLESCOPE_* variables. Keep
# that mapping explicit: Compose validates every service even for the sync
# profile, so omitting API or migration images prevents collection from starting.

set -euo pipefail

readonly sync_helper="${1:-deploy/remote-development/mrolescope-sync}"

die() {
  printf 'remote-development sync release check failed: %s\n' "$1" >&2
  exit 1
}

[[ -f "$sync_helper" ]] || die "sync helper does not exist: $sync_helper"
bash -n "$sync_helper"

load_release_body=$(awk '
  /^load_release\(\)/ { in_function = 1 }
  in_function { print }
  in_function && /^}$/ { exit }
' "$sync_helper")
[[ -n "$load_release_body" ]] || die 'load_release function was not found'
normalized_load_release_body=$(sed 's/^[[:space:]]*//' <<<"$load_release_body")

for mapping in \
  'export MROLESCOPE_API_IMAGE="$api_image"' \
  'export MROLESCOPE_MIGRATE_IMAGE="$migrate_image"' \
  'export MROLESCOPE_WORKER_IMAGE="$worker_image"'; do
  grep -Fqx "$mapping" <<<"$normalized_load_release_body" \
    || die "load_release must contain: ${mapping}"
done
