#!/bin/sh
# Render only the hostname into the Nginx configuration. Invite and session
# secrets belong to the private API runtime and are never passed to this image.

set -eu

die() {
  printf '%s\n' "mrolescope web edge: $*" >&2
  exit 1
}

hostname="${MROLESCOPE_WEB_HOSTNAME:-}"

printf '%s' "$hostname" | grep -Eq \
  '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$' \
  || die 'MROLESCOPE_WEB_HOSTNAME must be one lower-case DNS hostname'
[ -f /etc/nginx/tls/fullchain.pem ] || die 'TLS full chain is unavailable'
[ -f /etc/nginx/tls/privkey.pem ] || die 'TLS private key is unavailable'

install -d -m 0755 /run/nginx

# Explicitly limit substitution to the hostname. This leaves all Nginx `$...`
# variables intact and makes the environment hash unnecessary after startup.
runtime_config='/run/nginx/mrolescope.conf'
envsubst '${MROLESCOPE_WEB_HOSTNAME}' \
  </etc/nginx/nginx.conf.template >"$runtime_config"

nginx -t -c "$runtime_config"
exec nginx -c "$runtime_config" -g 'daemon off;'
