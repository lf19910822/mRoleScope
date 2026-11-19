#!/usr/bin/env bash
# Exercise the local release orchestrator without building, pushing, or
# contacting a real CVM. The mocked Docker metadata intentionally mirrors the
# digest form produced after a normal `docker push`.

set -euo pipefail

readonly repository_root="$(cd -- "$(dirname -- "$0")/.." && pwd)"
readonly release_script="$repository_root/scripts/release-remote-development.sh"
readonly revision='0123456789abcdef0123456789abcdef01234567'
readonly pinned_postgres_image='postgres:16.4-alpine@sha256:5660c2cbfea50c7a9127d17dc4e48543eedd3d7a41a595a2dfa572471e37e64c'
temporary_dir="$(mktemp -d)"

cleanup() {
  rm -rf -- "$temporary_dir"
}
trap cleanup EXIT

die() {
  printf 'local release orchestrator test failed: %s\n' "$1" >&2
  exit 1
}

mkdir -p "$temporary_dir/bin"
cat >"$temporary_dir/bin/git" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
args=("$@")
last="${args[$(( ${#args[@]} - 1 ))]}"
case "$last" in
  main)
    exit 0
    ;;
  --porcelain)
    [[ "${MROLESCOPE_TEST_DIRTY:-}" != true ]] || printf '%s\n' ' M simulated-change'
    exit 0
    ;;
  --show-current)
    printf '%s\n' "${MROLESCOPE_TEST_BRANCH:-main}"
    ;;
  HEAD|refs/remotes/origin/main)
    printf '%s\n' 0123456789abcdef0123456789abcdef01234567
    ;;
  *)
    exit 0
    ;;
esac
MOCK
cat >"$temporary_dir/bin/make" <<'MOCK'
#!/usr/bin/env bash
printf 'make %s\n' "$*" >>"$MROLESCOPE_RELEASE_TEST_LOG"
MOCK
cat >"$temporary_dir/bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf 'docker %s\n' "$*" >>"$MROLESCOPE_RELEASE_TEST_LOG"
case "$1" in
  build|push|version)
    printf 'simulated docker progress for %s\n' "$1"
    exit 0
    ;;
  image)
    if [[ "$2" != inspect ]]; then
      exit 1
    fi
    target="${!#}"
    if [[ "$*" == *'.Config.Labels'* ]]; then
      printf '%s\n' "${MROLESCOPE_TEST_LABEL:-0123456789abcdef0123456789abcdef01234567}"
      exit 0
    fi
    case "$target" in
      *mrolescope-api:*) printf '%s\n' 'daud1991/mrolescope-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' ;;
      *mrolescope-migrate:*) printf '%s\n' 'daud1991/mrolescope-migrate@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb' ;;
      *mrolescope-worker:*) printf '%s\n' 'daud1991/mrolescope-worker@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc' ;;
      *mrolescope-web-edge:*) printf '%s\n' 'daud1991/mrolescope-web-edge@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd' ;;
      *) exit 1 ;;
    esac
    ;;
  *)
    exit 1
    ;;
esac
MOCK
cat >"$temporary_dir/bin/ssh" <<'MOCK'
#!/usr/bin/env bash
printf 'ssh %s\n' "$*" >>"$MROLESCOPE_RELEASE_TEST_LOG"
remote_command="${!#}"
if [[ "${MROLESCOPE_TEST_REMOTE_CACHE_HIT:-}" == true && "$remote_command" == *"asset_marker="* ]]; then
  marker="${MROLESCOPE_TEST_ASSET_MARKER:?}"
  mkdir -p "$(dirname "$marker")"
  printf '%s\n' eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee >"$marker"
  chmod 0644 "$marker"
  rewritten="${remote_command//\/opt\/mrolescope\/remote-development.assets.sha256/$marker}"
  bash -c "$rewritten"
  : >"${MROLESCOPE_TEST_CACHE_HIT_FLAG:?}"
fi
MOCK
cat >"$temporary_dir/bin/sha256sum" <<'MOCK'
#!/usr/bin/env bash
if [[ "$#" -eq 0 ]]; then
  cat >/dev/null
  printf '%s  -\n' 'eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee'
else
  printf '%s  %s\n' 'eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee' "$1"
fi
MOCK
chmod 0755 "$temporary_dir/bin/git" "$temporary_dir/bin/make" "$temporary_dir/bin/docker" \
  "$temporary_dir/bin/ssh" "$temporary_dir/bin/sha256sum"

export PATH="$temporary_dir/bin:$PATH"
export MROLESCOPE_RELEASE_TEST_LOG="$temporary_dir/commands.log"

"$release_script" --dry-run >/dev/null
[[ ! -e "$temporary_dir/commands.log" ]] || die 'dry-run performed a build, push, verification, or SSH operation'

if MROLESCOPE_TEST_BRANCH=feature "$release_script" --dry-run >/dev/null 2>&1; then
  die 'a non-main checkout unexpectedly passed release preflight'
fi
if MROLESCOPE_TEST_DIRTY=true "$release_script" --dry-run >/dev/null 2>&1; then
  die 'a dirty checkout unexpectedly passed release preflight'
fi

"$release_script" --deploy >"$temporary_dir/release-output.log"
[[ "$(grep -c '^docker build ' "$MROLESCOPE_RELEASE_TEST_LOG")" -eq 4 ]] \
  || die 'successful release did not build exactly four images'
[[ "$(grep -c '^docker push ' "$MROLESCOPE_RELEASE_TEST_LOG")" -eq 4 ]] \
  || die 'successful release did not push exactly four images'
grep -Fqx "make -C ${repository_root} remote-release-verify" "$MROLESCOPE_RELEASE_TEST_LOG" \
  || die 'successful release skipped the local release verification target'
grep -Fq 'sudo -u mrolescope-deploy sudo -n /usr/local/sbin/mrolescope-deploy' "$MROLESCOPE_RELEASE_TEST_LOG" \
  || die 'successful release did not use the restricted CVM deployment helper'
grep -Fq 'mrolescope-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' "$MROLESCOPE_RELEASE_TEST_LOG" \
  || die 'successful release did not pass immutable API digest to CVM'
grep -Fq '[[ "$actual" == '\''eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee'\'' ]]' "$MROLESCOPE_RELEASE_TEST_LOG" \
  || die 'successful release did not verify the downloaded bootstrap against the reviewed checksum'
grep -Fq "timeout 120 docker pull '$pinned_postgres_image'" "$MROLESCOPE_RELEASE_TEST_LOG" \
  || die 'successful release did not preflight the exact pinned PostgreSQL pull through the CVM Docker daemon'
grep -Fq 'docker info --format' "$MROLESCOPE_RELEASE_TEST_LOG" \
  || die 'successful release did not report the CVM Docker daemon registry mirror context'
grep -Fq "asset_marker='/opt/mrolescope/remote-development.assets.sha256'" "$MROLESCOPE_RELEASE_TEST_LOG" \
  || die 'successful release did not compare the remote deployment asset fingerprint'
grep -Fq "'eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee'" "$MROLESCOPE_RELEASE_TEST_LOG" \
  || die 'successful release did not pass the asset fingerprint to bootstrap'
for phase in preflight quality-gate image-publication asset-refresh remote-deployment total; do
  grep -Fq "release timing: phase=${phase} elapsed_ms=" "$temporary_dir/release-output.log" \
    || die "release output omitted timing for ${phase}"
done
if grep -Fq 'registry-1.docker.io' "$MROLESCOPE_RELEASE_TEST_LOG"; then
  die 'release network preflight introduced a direct official Registry probe'
fi
preflight_line=$(grep -n "timeout 120 docker pull '$pinned_postgres_image'" "$MROLESCOPE_RELEASE_TEST_LOG" | head -n 1 | cut -d: -f1)
quality_gate_line=$(grep -n "^make -C ${repository_root} remote-release-verify$" "$MROLESCOPE_RELEASE_TEST_LOG" | head -n 1 | cut -d: -f1)
build_line=$(grep -n '^docker build ' "$MROLESCOPE_RELEASE_TEST_LOG" | head -n 1 | cut -d: -f1)
[[ -n "$preflight_line" && -n "$quality_gate_line" && -n "$build_line" \
  && "$preflight_line" -lt "$quality_gate_line" && "$quality_gate_line" -lt "$build_line" ]] \
  || die 'release ordering did not put CVM Docker pull preflight before quality gate and image build'

rm -f -- "$MROLESCOPE_RELEASE_TEST_LOG"
export MROLESCOPE_TEST_REMOTE_CACHE_HIT=true
export MROLESCOPE_TEST_ASSET_MARKER="$temporary_dir/remote-development.assets.sha256"
export MROLESCOPE_TEST_CACHE_HIT_FLAG="$temporary_dir/cache-hit"
"$release_script" --deploy >"$temporary_dir/cache-hit-output.log"
[[ -e "$MROLESCOPE_TEST_CACHE_HIT_FLAG" ]] || die 'matching asset fingerprint did not execute the remote cache-hit branch'
grep -Fq 'deployment assets unchanged; bootstrap download skipped.' "$temporary_dir/cache-hit-output.log" \
  || die 'cache-hit release did not report skipped bootstrap downloads'
unset MROLESCOPE_TEST_REMOTE_CACHE_HIT MROLESCOPE_TEST_ASSET_MARKER MROLESCOPE_TEST_CACHE_HIT_FLAG

rm -f -- "$MROLESCOPE_RELEASE_TEST_LOG"
if MROLESCOPE_TEST_LABEL=not-the-reviewed-revision "$release_script" --deploy >/dev/null 2>&1; then
  die 'mismatched image revision label unexpectedly reached a release'
fi
if [[ -e "$MROLESCOPE_RELEASE_TEST_LOG" ]] && grep -q '^docker push ' "$MROLESCOPE_RELEASE_TEST_LOG"; then
  die 'mismatched image revision label pushed an image'
fi
if [[ -e "$MROLESCOPE_RELEASE_TEST_LOG" ]] && grep -q 'mrolescope-deploy' "$MROLESCOPE_RELEASE_TEST_LOG"; then
  die 'mismatched image revision label reached the CVM deployment helper'
fi

printf '%s\n' 'Local release orchestrator test passed'
