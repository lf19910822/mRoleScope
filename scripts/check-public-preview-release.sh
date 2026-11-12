#!/usr/bin/env bash
# Static release gate for the invite-gated public preview. It checks only
# tracked assets and never reads runtime secrets or contacts a host.

set -euo pipefail

readonly repository_root="$(cd -- "$(dirname -- "$0")/.." && pwd)"
readonly compose_file="$repository_root/deploy/remote-development/compose.yaml"
readonly nginx_template="$repository_root/docker/web-edge/nginx.conf.template"
readonly entrypoint="$repository_root/docker/web-edge/entrypoint.sh"
readonly deploy_helper="$repository_root/deploy/remote-development/mrolescope-deploy"

die() {
  printf 'public preview release check failed: %s\n' "$1" >&2
  exit 1
}

grep -Fqx '      MROLESCOPE_INVITE_BCRYPT_HASH: ${MROLESCOPE_INVITE_BCRYPT_HASH:-}' "$compose_file" \
  || die 'API Compose service must receive the invite hash from runtime configuration'
grep -Fqx '      MROLESCOPE_SESSION_SIGNING_SECRET: ${MROLESCOPE_SESSION_SIGNING_SECRET:-}' "$compose_file" \
  || die 'API Compose service must receive the session signing secret from runtime configuration'
grep -Fq 'auth_request /_mrolescope_auth;' "$nginx_template" \
  || die 'Web Edge must use Nginx auth_request'
grep -Fq 'location = /access' "$nginx_template" \
  || die 'Web Edge must expose the exact invite page location'
grep -Fq 'location = /api/v1/access/verify' "$nginx_template" \
  || die 'Web Edge must expose the exact verification location'
grep -Fq 'limit_req_zone $binary_remote_addr zone=mrolescope_verify' "$nginx_template" \
  || die 'Web Edge must rate-limit verification requests'
grep -Fq 'limit_req_zone $binary_remote_addr zone=mrolescope_api' "$nginx_template" \
  || die 'Web Edge must rate-limit business API requests'
grep -Fq 'gzip on;' "$nginx_template" \
  || die 'Web Edge must compress browser assets'
grep -Fq 'location ^~ /assets/' "$nginx_template" \
  || die 'Web Edge must isolate hashed browser assets for caching'
grep -Fq 'Cache-Control "private, max-age=31536000, immutable"' "$nginx_template" \
  || die 'hashed browser assets must be cached immutably'
grep -Fq 'Cache-Control "no-store"' "$nginx_template" \
  || die 'invite and business responses must retain no-store protection'
if grep -Eiq 'auth_basic|WEB_AUTH_BCRYPT_HASH|\$http_authorization|\$request_body' "$nginx_template" "$entrypoint"; then
  die 'Web Edge assets contain a legacy auth or sensitive request logging contract'
fi
grep -Fq 'MROLESCOPE_INVITE_BCRYPT_HASH' "$deploy_helper" \
  && grep -Fq 'MROLESCOPE_SESSION_SIGNING_SECRET' "$deploy_helper" \
  || die 'deployment helper must require both access runtime values'
grep -Fq 'if [[ "$status" == 302 ]]' "$deploy_helper" \
  || die 'deployment helper must probe the invite redirect boundary'
if grep -Eiq 'MROLESCOPE_INVITE_BCRYPT_HASH|MROLESCOPE_SESSION_SIGNING_SECRET' "$nginx_template" "$entrypoint"; then
  die 'invite/session secrets must not enter Web Edge assets'
fi
printf '%s\n' 'public preview release check passed'
