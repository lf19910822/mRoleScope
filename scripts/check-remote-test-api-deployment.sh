#!/usr/bin/env bash
# Check the fixed boundary for the loopback-only candidate API deployment.

set -euo pipefail

readonly compose_file="${1:-deploy/remote-development/test-api.compose.yaml}"
readonly helper="${2:-deploy/remote-development/mrolescope-test-api-deploy}"
readonly bootstrap="${3:-deploy/remote-development/bootstrap.sh}"

die() {
  printf 'remote test API deployment check failed: %s\n' "$1" >&2
  exit 1
}

for path in "$compose_file" "$helper" "$bootstrap"; do
  [[ -f "$path" ]] || die "required file does not exist: ${path}"
done

bash -n "$helper" "$bootstrap"

service_block=$(awk '
  $0 == "  api-test:" { in_block = 1; next }
  in_block && /^  [A-Za-z0-9][A-Za-z0-9-]*:$/ { exit }
  in_block { print }
' "$compose_file")
[[ -n "$service_block" ]] || die 'test Compose service api-test was not found'
grep -Fqx '    image: ${MROLESCOPE_TEST_API_IMAGE}' <<<"$service_block" \
  || die 'test API must receive its image only through MROLESCOPE_TEST_API_IMAGE'
grep -Fqx '      APP_ENV: remote-test-api' <<<"$service_block" \
  || die 'test API must identify itself as remote-test-api'
grep -Fqx '      - "127.0.0.1:18080:8080"' <<<"$service_block" \
  || die 'test API must bind only the fixed loopback port'
grep -Fqx '    networks: [formal-backend, formal-edge, formal-ai-edge]' <<<"$service_block" \
  || die 'test API must join only the formal data and egress networks'
grep -Fqx '    read_only: true' <<<"$service_block" \
  || die 'test API root filesystem must remain read-only'
if grep -Eq '^[[:space:]]+(depends_on:|profiles:|volumes:)|^[[:space:]]+(postgres|migrate|worker|web-edge):' "$compose_file"; then
  die 'test Compose must not declare or operate data, worker, or Web Edge services'
fi
[[ "$(grep -Fxc '    ports:' "$compose_file")" -eq 1 ]] \
  || die 'test Compose must publish exactly one host port'
if grep -Eq '(^|[^0-9])8080:8080|0\.0\.0\.0|"80:80"|"443:443"' "$compose_file"; then
  die 'test Compose must not expose API or Web Edge ports publicly'
fi
for network in \
  '    name: mrolescope-remote-development_backend' \
  '    name: mrolescope-remote-development_edge' \
  '    name: mrolescope-remote-development_ai-edge'; do
  grep -Fqx "$network" "$compose_file" || die "missing required formal network: ${network}"
done
[[ "$(grep -Fxc '    external: true' "$compose_file")" -eq 3 ]] \
  || die 'all test API networks must be existing formal networks'

grep -Fqx "readonly COMPOSE_FILE='/opt/mrolescope/remote-test-api.compose.yaml'" "$helper" \
  || die 'test helper must use only the fixed test Compose path'
grep -Fqx "readonly PROJECT_NAME='mrolescope-remote-test-api'" "$helper" \
  || die 'test helper must use an isolated Compose project name'
grep -Fqx "readonly RELEASE_DIR='/var/lib/mrolescope/test-api-releases'" "$helper" \
  || die 'test helper must keep an isolated release directory'
grep -Fqx "[[ \"\$#\" -eq 1 ]] || die 'expected one API image digest'" "$helper" \
  || die 'test helper must accept exactly one API digest'
grep -Fqx 'compose pull api-test' "$helper" \
  || die 'test helper must pull only the test API image'
grep -Fq 'compose up -d --no-deps api-test' "$helper" \
  || die 'test helper must start only the test API service'
grep -Fq "[[ \"\$binding\" == '127.0.0.1:18080' ]]" "$helper" \
  || die 'test helper must verify the running test API loopback binding'
grep -Fq 'compose stop api-test >/dev/null 2>&1 || true' "$helper" \
  || die 'test helper must stop only an unverified test API service'
if grep -Eq 'compose .*(postgres|migrate|worker|web-edge| api($| ))' "$helper"; then
  die 'test helper must not operate formal services'
fi

grep -Fqx "readonly TEST_API_COMPOSE_DESTINATION='/opt/mrolescope/remote-test-api.compose.yaml'" "$bootstrap" \
  || die 'bootstrap must define the test Compose destination'
grep -Fqx "readonly TEST_API_HELPER_DESTINATION='/usr/local/sbin/mrolescope-test-api-deploy'" "$bootstrap" \
  || die 'bootstrap must define the test helper destination'
grep -Fqx "readonly TEST_API_RELEASE_DIR='/var/lib/mrolescope/test-api-releases'" "$bootstrap" \
  || die 'bootstrap must define the test release directory'
grep -Fqx "download_asset 'deploy/remote-development/test-api.compose.yaml' \"\$staging_dir/test-api.compose.yaml\"" "$bootstrap" \
  || die 'bootstrap must stage the test Compose asset'
grep -Fqx "download_asset 'deploy/remote-development/mrolescope-test-api-deploy' \"\$staging_dir/mrolescope-test-api-deploy\"" "$bootstrap" \
  || die 'bootstrap must stage the test deployment helper'
grep -Fq 'mrolescope-deploy ALL=(root) NOPASSWD: /usr/local/sbin/mrolescope-deploy *, /usr/local/sbin/mrolescope-test-api-deploy *, /usr/local/sbin/mrolescope-operations *' "$bootstrap" \
  || die 'bootstrap must allow the deployment account to run only the fixed test helper'

printf '%s\n' 'remote test API deployment check passed'
