#!/usr/bin/env bash
# Exercise the root deployment helper's image allowlist with a fully local
# mock Docker CLI. This proves a fourth immutable Web Edge digest is required
# without pulling images, touching /opt, or exposing any listener.

set -euo pipefail

readonly repository_root="$(cd -- "$(dirname -- "$0")/.." && pwd)"
readonly source_helper="$repository_root/deploy/remote-development/mrolescope-deploy"
temporary_dir="$(mktemp -d)"
readonly namespace='mrolescopeverify'
readonly revision='0123456789abcdef0123456789abcdef01234567'
readonly api_image="docker.io/${namespace}/mrolescope-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
readonly migrate_image="docker.io/${namespace}/mrolescope-migrate@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
readonly worker_image="docker.io/${namespace}/mrolescope-worker@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
readonly web_edge_image="docker.io/${namespace}/mrolescope-web-edge@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
readonly replacement_web_edge_image="docker.io/${namespace}/mrolescope-web-edge@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"

cleanup() {
  rm -rf -- "$temporary_dir"
}
trap cleanup EXIT

die() {
  printf 'Web Edge deployment helper test failed: %s\n' "$1" >&2
  exit 1
}

mkdir -p "$temporary_dir/bin" "$temporary_dir/releases"
printf '%s\n' 'services: {}' >"$temporary_dir/compose.yaml"
printf 'POSTGRES_DB=test\nPOSTGRES_USER=test\nPOSTGRES_PASSWORD=test\nDATABASE_URL=postgres://test:test@postgres/test\n' >"$temporary_dir/runtime.env"
printf 'DOCKERHUB_NAMESPACE=%s\n' "$namespace" >"$temporary_dir/deploy.conf"
chmod 0644 "$temporary_dir/compose.yaml" "$temporary_dir/deploy.conf"
chmod 0600 "$temporary_dir/runtime.env"
chmod 0700 "$temporary_dir/releases"

cat >"$temporary_dir/bin/docker" <<MOCK
#!/usr/bin/env bash
set -euo pipefail
case "\$1" in
  compose)
    if [[ " \$* " == *' web-edge '* ]]; then
      printf '%s\\n' "\${MROLESCOPE_WEB_EDGE_IMAGE:-}" >>"\${MROLESCOPE_TEST_DOCKER_LOG}"
    fi
    if [[ " \$* " == *' ps -q api '* ]]; then
      printf '%s\\n' api-test-container
    fi
    ;;
  image)
    [[ "\$2" == inspect ]] || exit 1
    printf '%s\\n' '$revision'
    ;;
  inspect)
    printf '%s\\n' healthy
    ;;
  *)
    exit 1
    ;;
esac
MOCK
chmod 0755 "$temporary_dir/bin/docker"

cat >"$temporary_dir/bin/curl" <<MOCK
#!/usr/bin/env bash
set -euo pipefail
if [[ "\${MROLESCOPE_WEB_EDGE_IMAGE:-}" == '$web_edge_image' ]]; then
  printf '%s' 401
else
  printf '%s' 000
fi
MOCK
chmod 0755 "$temporary_dir/bin/curl"

# The real helper must retain its root-only and caller-identity guards. This
# unit test exercises its image allowlist under an ordinary CI user, so the
# isolated copy replaces process-identity and root-ownership guards; it
# continues to run all image-input, release-record, rollback, and readiness
# logic.
grep -Fqx '[[ "${EUID}" -eq 0 ]] || die "must be invoked through sudo"' "$source_helper" \
  || die 'deployment helper no longer has its root-only guard'
grep -Fqx 'if [[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != "mrolescope-deploy" ]]; then' "$source_helper" \
  || die 'deployment helper no longer has its allowed-caller guard'
grep -Fqx '  [[ "$metadata" == "root:root:${expected_mode}" ]]' "$source_helper" \
  || die 'deployment helper no longer checks root file ownership and mode'
grep -Fqx '[[ "$(stat -c '\''%U:%G:%a'\'' "$RELEASE_DIR")" == '\''root:root:700'\'' ]] || die "release directory must be root:root mode 0700"' "$source_helper" \
  || die 'deployment helper no longer checks release-directory ownership and mode'

# Render a test-only copy with its fixed production paths redirected to the
# temporary directory. The source helper's identity and root-ownership guards
# are asserted above and replaced below because this test has no real sudo
# service account or root-owned fixture files.
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

run_helper() {
  env -u SUDO_USER "$temporary_dir/mrolescope-deploy" "$@"
}

export MROLESCOPE_TEST_DOCKER_LOG="$temporary_dir/docker.log"
run_helper "$api_image" "$migrate_image" "$worker_image" "$web_edge_image" >/dev/null
grep -Fqx "web_edge_image=${web_edge_image}" "$temporary_dir/releases/current.env" \
  || die 'successful release did not record its Web Edge digest'

# With the manual gate enabled, a failed new Edge must restore only the prior
# verified Edge digest. The mocked curl returns 000 for the candidate then 401
# for the former digest, so this tests the independent rollback path without a
# real listener or hostname.
printf 'POSTGRES_DB=test\nPOSTGRES_USER=test\nPOSTGRES_PASSWORD=test\nDATABASE_URL=postgres://test:test@postgres/test\nMROLESCOPE_WEB_EDGE_ENABLED=true\nMROLESCOPE_WEB_HOSTNAME=jobs.example.test\nMROLESCOPE_WEB_AUTH_BCRYPT_HASH='"'"'\$2b\$05\$lU6JnhJzxffWpA4E/ips0uEq2FHVSvClisOk.iZC5QrLSaeIYb3BW'"'"'\n' >"$temporary_dir/runtime.env"
chmod 0600 "$temporary_dir/runtime.env"
: >"$MROLESCOPE_TEST_DOCKER_LOG"
if run_helper "$api_image" "$migrate_image" "$worker_image" "$replacement_web_edge_image" >"$temporary_dir/rollback.log" 2>&1; then
  die 'a Web Edge readiness failure was accepted as a successful release'
fi
grep -Fqx "$web_edge_image" "$MROLESCOPE_TEST_DOCKER_LOG" \
  || die 'failed Web Edge did not restore the prior verified Edge digest'

expect_rejected() {
  local name="$1"
  shift
  if run_helper "$@" >"$temporary_dir/${name}.log" 2>&1; then
    cat "$temporary_dir/${name}.log" >&2
    die "helper accepted invalid ${name} input"
  fi
}

expect_rejected omitted-web-edge "$api_image" "$migrate_image" "$worker_image"
expect_rejected mutable-web-edge "$api_image" "$migrate_image" "$worker_image" "docker.io/${namespace}/mrolescope-web-edge:latest"
expect_rejected foreign-web-edge "$api_image" "$migrate_image" "$worker_image" 'docker.io/foreign/mrolescope-web-edge@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd'

printf '%s\n' 'Web Edge deployment helper allowlist test passed'
