#!/usr/bin/env bash
# Prove that a successful deployment removes only old, unreferenced images in
# its own Docker Hub namespace while retaining the current and prior release.

set -euo pipefail

readonly repository_root="$(cd -- "$(dirname -- "$0")/.." && pwd)"
readonly source_helper="$repository_root/deploy/remote-development/mrolescope-deploy"
temporary_dir="$(mktemp -d)"
readonly namespace='mrolescopecleanup'
readonly revision='0123456789abcdef0123456789abcdef01234567'
readonly previous_api="docker.io/${namespace}/mrolescope-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
readonly previous_migrate="docker.io/${namespace}/mrolescope-migrate@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
readonly previous_worker="docker.io/${namespace}/mrolescope-worker@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
readonly previous_web_edge="docker.io/${namespace}/mrolescope-web-edge@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
readonly current_api="docker.io/${namespace}/mrolescope-api@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
readonly current_migrate="docker.io/${namespace}/mrolescope-migrate@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
readonly current_worker="docker.io/${namespace}/mrolescope-worker@sha256:9999999999999999999999999999999999999999999999999999999999999999"
readonly current_web_edge="docker.io/${namespace}/mrolescope-web-edge@sha256:8888888888888888888888888888888888888888888888888888888888888888"
readonly stale_api="docker.io/${namespace}/mrolescope-api@sha256:1111111111111111111111111111111111111111111111111111111111111111"
readonly stale_migrate="docker.io/${namespace}/mrolescope-migrate@sha256:2222222222222222222222222222222222222222222222222222222222222222"
readonly stale_worker="docker.io/${namespace}/mrolescope-worker@sha256:3333333333333333333333333333333333333333333333333333333333333333"
readonly stale_web_edge="docker.io/${namespace}/mrolescope-web-edge@sha256:4444444444444444444444444444444444444444444444444444444444444444"

cleanup() {
  rm -rf -- "$temporary_dir"
}
trap cleanup EXIT

die() {
  printf 'Release-image cleanup test failed: %s\n' "$1" >&2
  exit 1
}

mkdir -p "$temporary_dir/bin" "$temporary_dir/releases"
printf '%s\n' 'services: {}' >"$temporary_dir/compose.yaml"
printf 'POSTGRES_DB=test\nPOSTGRES_USER=test\nPOSTGRES_PASSWORD=test\nDATABASE_URL=postgres://test:test@postgres/test\n' >"$temporary_dir/runtime.env"
printf 'DOCKERHUB_NAMESPACE=%s\n' "$namespace" >"$temporary_dir/deploy.conf"
printf 'api_image=%s\nmigrate_image=%s\nworker_image=%s\nweb_edge_image=%s\nrevision=%s\nverified_at=2026-11-06T00:00:00Z\n' \
  "$previous_api" "$previous_migrate" "$previous_worker" "$previous_web_edge" "$revision" \
  >"$temporary_dir/releases/current.env"
chmod 0644 "$temporary_dir/compose.yaml" "$temporary_dir/deploy.conf"
chmod 0600 "$temporary_dir/runtime.env" "$temporary_dir/releases/current.env"
chmod 0700 "$temporary_dir/releases"

cat >"$temporary_dir/bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail

case "$1" in
  compose)
    if [[ " $* " == *' ps -q api '* ]]; then
      printf '%s\n' api-test-container
    fi
    ;;
  image)
    case "$2" in
      inspect)
        if [[ " $* " == *'{{.Id}}'* ]]; then
          case "${!#}" in
            *3333333333333333333333333333333333333333333333333333333333333333)
              printf '%s\n' 'sha256:stale-worker-image'
              ;;
            *)
              printf '%s\n' "sha256:${!#}"
              ;;
          esac
        else
          printf '%s\n' "$MROLESCOPE_TEST_REVISION"
        fi
        ;;
      ls)
        for row in \
          "mrolescopecleanup/mrolescope-api sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" \
          "mrolescopecleanup/mrolescope-migrate sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" \
          "mrolescopecleanup/mrolescope-worker sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc" \
          "mrolescopecleanup/mrolescope-web-edge sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd" \
          "mrolescopecleanup/mrolescope-api sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee" \
          "mrolescopecleanup/mrolescope-migrate sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff" \
          "mrolescopecleanup/mrolescope-worker sha256:9999999999999999999999999999999999999999999999999999999999999999" \
          "mrolescopecleanup/mrolescope-web-edge sha256:8888888888888888888888888888888888888888888888888888888888888888" \
          "mrolescopecleanup/mrolescope-api sha256:1111111111111111111111111111111111111111111111111111111111111111" \
          "mrolescopecleanup/mrolescope-migrate sha256:2222222222222222222222222222222222222222222222222222222222222222" \
          "mrolescopecleanup/mrolescope-worker sha256:3333333333333333333333333333333333333333333333333333333333333333" \
          "mrolescopecleanup/mrolescope-web-edge sha256:4444444444444444444444444444444444444444444444444444444444444444" \
          "foreign/workload sha256:5555555555555555555555555555555555555555555555555555555555555555"; do
          printf '%s\n' "$row"
        done
        ;;
      rm)
        printf '%s\n' "${!#}" >>"$MROLESCOPE_TEST_DOCKER_LOG"
        ;;
      *)
        exit 1
        ;;
    esac
    ;;
  container)
    case "$2" in
      ls)
        printf '%s\n' stale-worker-container
        ;;
      inspect)
        printf '%s\n' 'sha256:stale-worker-image'
        ;;
      *)
        exit 1
        ;;
    esac
    ;;
  inspect)
    printf '%s\n' healthy
    ;;
  *)
    exit 1
    ;;
esac
MOCK
chmod 0755 "$temporary_dir/bin/docker"

grep -Fqx '[[ "${EUID}" -eq 0 ]] || die "must be invoked through sudo"' "$source_helper" \
  || die 'deployment helper no longer has its root-only guard'
grep -Fqx 'if [[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != "mrolescope-deploy" ]]; then' "$source_helper" \
  || die 'deployment helper no longer has its allowed-caller guard'

sed \
  -e "s|/opt/mrolescope/remote-development.compose.yaml|$temporary_dir/compose.yaml|" \
  -e "s|/etc/mrolescope/secrets/remote-development.env|$temporary_dir/runtime.env|" \
  -e "s|/etc/mrolescope/remote-development.conf|$temporary_dir/deploy.conf|" \
  -e "s|/var/lib/mrolescope/releases|$temporary_dir/releases|g" \
  -e "s|PATH='/usr/sbin:/usr/bin:/sbin:/bin'|PATH='$temporary_dir/bin:/usr/sbin:/usr/bin:/sbin:/bin'|" \
  -e 's/attempt <= 20/attempt <= 1/g' \
  -e 's/^\[\[ "${EUID}" -eq 0 \]\] || die "must be invoked through sudo"$/true # test-only identity bypass/' \
  -e 's/^if \[\[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != "mrolescope-deploy" \]\]; then$/if false; then # test-only identity bypass/' \
  -e '/^is_root_owned_mode() {$/,/^}$/c\is_root_owned_mode() { return 0; } # test-only ownership bypass' \
  -e 's/^\[\[ "$(stat -c '\''%U:%G:%a'\'' "$RELEASE_DIR")" == '\''root:root:700'\'' \]\] || die "release directory must be root:root mode 0700"$/true # test-only release ownership bypass/' \
  "$source_helper" >"$temporary_dir/mrolescope-deploy"
chmod 0750 "$temporary_dir/mrolescope-deploy"

export MROLESCOPE_TEST_DOCKER_LOG="$temporary_dir/docker.log"
export MROLESCOPE_TEST_REVISION="$revision"
env -u SUDO_USER "$temporary_dir/mrolescope-deploy" \
  "$current_api" "$current_migrate" "$current_worker" "$current_web_edge" >/dev/null

for expected in "$stale_api" "$stale_migrate" "$stale_web_edge"; do
  grep -Fqx "$expected" "$MROLESCOPE_TEST_DOCKER_LOG" \
    || die "stale unreferenced image was not removed: $expected"
done
if grep -Fqx "$stale_worker" "$MROLESCOPE_TEST_DOCKER_LOG"; then
  die 'image referenced by a container was removed'
fi
for retained in "$previous_api" "$previous_migrate" "$previous_worker" "$previous_web_edge" \
  "$current_api" "$current_migrate" "$current_worker" "$current_web_edge"; do
  if grep -Fqx "$retained" "$MROLESCOPE_TEST_DOCKER_LOG"; then
    die "retained release image was removed: $retained"
  fi
done
if grep -Fq 'foreign/workload' "$MROLESCOPE_TEST_DOCKER_LOG"; then
  die 'foreign Docker image was considered for removal'
fi

printf '%s\n' 'Release-image cleanup deployment helper test passed'
