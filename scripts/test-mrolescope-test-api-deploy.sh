#!/usr/bin/env bash
# Exercise the loopback test API helper with a local mock Docker CLI. The test
# never reads a real runtime configuration or starts a container.

set -euo pipefail

readonly repository_root="$(cd -- "$(dirname -- "$0")/.." && pwd)"
readonly source_helper="$repository_root/deploy/remote-development/mrolescope-test-api-deploy"
temporary_dir="$(mktemp -d)"
readonly namespace='mrolescopeverify'
readonly revision='0123456789abcdef0123456789abcdef01234567'
readonly api_image="docker.io/${namespace}/mrolescope-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
readonly replacement_api_image="docker.io/${namespace}/mrolescope-api@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

cleanup() {
  rm -rf -- "$temporary_dir"
}
trap cleanup EXIT

die() {
  printf 'test API deployment helper test failed: %s\n' "$1" >&2
  exit 1
}

mkdir -p "$temporary_dir/bin" "$temporary_dir/releases"
printf '%s\n' 'services: {}' >"$temporary_dir/compose.yaml"
printf 'DATABASE_URL=postgres://test:test@postgres/test\nMROLESCOPE_INVITE_BCRYPT_HASH='"'"'\$2b\$05\$lU6JnhJzxffWpA4E/ips0uEq2FHVSvClisOk.iZC5QrLSaeIYb3BW'"'"'\nMROLESCOPE_SESSION_SIGNING_SECRET='"'"'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'"'"'\n' >"$temporary_dir/runtime.env"
printf 'DOCKERHUB_NAMESPACE=%s\n' "$namespace" >"$temporary_dir/deploy.conf"
chmod 0644 "$temporary_dir/compose.yaml" "$temporary_dir/deploy.conf"
chmod 0600 "$temporary_dir/runtime.env"
chmod 0700 "$temporary_dir/releases"

cat >"$temporary_dir/bin/docker" <<MOCK
#!/usr/bin/env bash
set -euo pipefail
case "\$1" in
  network)
    [[ "\$2" == inspect ]] || exit 1
    ;;
  compose)
    printf 'compose %s\n' "\$*" >>"\${MROLESCOPE_TEST_DOCKER_LOG}"
    if [[ "\${MROLESCOPE_TEST_PULL_FAIL:-false}" == true && " \$* " == *' pull '* ]]; then
      printf '%s\n' 'simulated Compose pull failure' >&2
      exit 17
    fi
    if [[ " \$* " == *' up '* ]]; then
      printf '%s\n' "\${MROLESCOPE_TEST_API_IMAGE:-}" >>"\${MROLESCOPE_TEST_DOCKER_LOG}"
    fi
    if [[ " \$* " == *' ps -q api-test '* ]]; then
      printf '%s\n' api-test-container
    fi
    ;;
  image)
    [[ "\$2" == inspect ]] || exit 1
    printf '%s\n' '$revision'
    ;;
  inspect)
    if [[ "\$*" == *'.NetworkSettings.Ports'* ]]; then
      printf '%s\n' "\${MROLESCOPE_TEST_PORT_BINDING:-127.0.0.1:18080}"
      exit 0
    fi
    if [[ "\${MROLESCOPE_TEST_UNHEALTHY_IMAGE:-}" == "\${MROLESCOPE_TEST_API_IMAGE:-}" ]]; then
      printf '%s\n' unhealthy
    else
      printf '%s\n' healthy
    fi
    ;;
  *)
    exit 1
    ;;
esac
MOCK
chmod 0755 "$temporary_dir/bin/docker"

grep -Fqx "[[ \"\${EUID}\" -eq 0 ]] || die 'must be invoked through sudo'" "$source_helper" \
  || die 'test helper no longer has the root-only guard'
grep -Fqx 'if [[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != '\''mrolescope-deploy'\'' ]]; then' "$source_helper" \
  || die 'test helper no longer has the allowed-caller guard'
grep -Fqx '  [[ "$metadata" == "root:root:${expected_mode}" ]]' "$source_helper" \
  || die 'test helper no longer verifies root-owned file modes'

sed \
  -e "s|/opt/mrolescope/remote-test-api.compose.yaml|$temporary_dir/compose.yaml|" \
  -e "s|/etc/mrolescope/secrets/remote-development.env|$temporary_dir/runtime.env|" \
  -e "s|/etc/mrolescope/remote-development.conf|$temporary_dir/deploy.conf|" \
  -e "s|/var/lib/mrolescope/test-api-releases|$temporary_dir/releases|g" \
  -e "s|PATH='/usr/sbin:/usr/bin:/sbin:/bin'|PATH='$temporary_dir/bin:/usr/sbin:/usr/bin:/sbin:/bin'|" \
  -e 's/attempt <= 20/attempt <= 1/g' \
  -e 's/sleep 2/true/g' \
  -e 's/^\[\[ "${EUID}" -eq 0 \]\] || die '\''must be invoked through sudo'\''$/true # test-only identity bypass/' \
  -e 's/^if \[\[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != '\''mrolescope-deploy'\'' \]\]; then$/if false; then # test-only identity bypass/' \
  -e '/^is_root_owned_mode() {$/,/^}$/c\is_root_owned_mode() { return 0; } # test-only ownership bypass' \
  -e 's/^\[\[ "$(stat -c '\''%U:%G:%a'\'' "$RELEASE_DIR")" == '\''root:root:700'\'' \]\] || die '\''test API release directory must be root:root mode 0700'\''$/true # test-only release ownership bypass/' \
  "$source_helper" >"$temporary_dir/mrolescope-test-api-deploy"
chmod 0750 "$temporary_dir/mrolescope-test-api-deploy"

run_helper() {
  env -u SUDO_USER "$temporary_dir/mrolescope-test-api-deploy" "$@"
}

export MROLESCOPE_TEST_DOCKER_LOG="$temporary_dir/docker.log"
run_helper "$api_image" >/dev/null
grep -Fqx "api_image=${api_image}" "$temporary_dir/releases/current.env" \
  || die 'successful test API release did not record its API digest'
if grep -Eq 'compose .* (postgres|migrate|worker|web-edge| api($| ))' "$MROLESCOPE_TEST_DOCKER_LOG"; then
  die 'test helper operated a formal service'
fi

expect_rejected() {
  local name="$1"
  shift
  if run_helper "$@" >"$temporary_dir/${name}.log" 2>&1; then
    cat "$temporary_dir/${name}.log" >&2
    die "helper accepted invalid ${name} input"
  fi
}

expect_rejected omitted-api
expect_rejected mutable-api "docker.io/${namespace}/mrolescope-api:latest"
expect_rejected foreign-api 'docker.io/foreign/mrolescope-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'

export MROLESCOPE_TEST_UNHEALTHY_IMAGE="$replacement_api_image"
: >"$MROLESCOPE_TEST_DOCKER_LOG"
if run_helper "$replacement_api_image" >"$temporary_dir/rollback.log" 2>&1; then
  die 'an unhealthy test API was accepted as a successful release'
fi
grep -Fqx "$replacement_api_image" "$MROLESCOPE_TEST_DOCKER_LOG" \
  || die 'failed test API was not started before health validation'
grep -Fqx "$api_image" "$MROLESCOPE_TEST_DOCKER_LOG" \
  || die 'failed test API did not restore the prior verified test digest'
grep -Fqx "api_image=${api_image}" "$temporary_dir/releases/current.env" \
  || die 'failed test API replaced the verified test release record'
unset MROLESCOPE_TEST_UNHEALTHY_IMAGE

export MROLESCOPE_TEST_PORT_BINDING='0.0.0.0:18080'
: >"$MROLESCOPE_TEST_DOCKER_LOG"
if run_helper "$replacement_api_image" >"$temporary_dir/public-port.log" 2>&1; then
  die 'a publicly bound test API was accepted as a successful release'
fi
grep -Fqx "$replacement_api_image" "$MROLESCOPE_TEST_DOCKER_LOG" \
  || die 'publicly bound test API was not started before port validation'
grep -Fqx "$api_image" "$MROLESCOPE_TEST_DOCKER_LOG" \
  || die 'publicly bound test API did not restore the prior verified test digest'
grep -Fqx "api_image=${api_image}" "$temporary_dir/releases/current.env" \
  || die 'publicly bound test API replaced the verified test release record'
unset MROLESCOPE_TEST_PORT_BINDING

printf '%s\n' 'test API deployment helper test passed'
