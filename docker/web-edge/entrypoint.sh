#!/bin/sh
# Render only the hostname into the Nginx configuration. The Basic-Auth hash
# remains in an ephemeral htpasswd file rather than becoming part of an image
# layer or a static configuration asset.

set -eu

die() {
  printf '%s\n' "mrolescope web edge: $*" >&2
  exit 1
}

hostname="${MROLESCOPE_WEB_HOSTNAME:-}"
bcrypt_hash="${MROLESCOPE_WEB_AUTH_BCRYPT_HASH:-}"

printf '%s' "$hostname" | grep -Eq \
  '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$' \
  || die 'MROLESCOPE_WEB_HOSTNAME must be one lower-case DNS hostname'
printf '%s' "$bcrypt_hash" | grep -Eq '^\$2[aby]\$[0-3][0-9]\$[./A-Za-z0-9]{53}$' \
  || die 'MROLESCOPE_WEB_AUTH_BCRYPT_HASH must be a bcrypt password hash'
[ -f /etc/nginx/tls/fullchain.pem ] || die 'TLS full chain is unavailable'
[ -f /etc/nginx/tls/privkey.pem ] || die 'TLS private key is unavailable'

install -d -m 0755 /run/nginx
printf 'mrolescope:%s\n' "$bcrypt_hash" >/run/nginx/mrolescope.htpasswd
chown nginx:nginx /run/nginx/mrolescope.htpasswd
chmod 0400 /run/nginx/mrolescope.htpasswd

# Explicitly limit substitution to the hostname. This leaves all Nginx `$...`
# variables intact and makes the environment hash unnecessary after startup.
runtime_config='/run/nginx/mrolescope.conf'
envsubst '${MROLESCOPE_WEB_HOSTNAME}' \
  </etc/nginx/nginx.conf.template >"$runtime_config"
unset MROLESCOPE_WEB_AUTH_BCRYPT_HASH

nginx -t -c "$runtime_config"
exec nginx -c "$runtime_config" -g 'daemon off;'
