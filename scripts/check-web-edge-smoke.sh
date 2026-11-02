#!/usr/bin/env bash
# Run only from the operator's terminal after the manual domain/hosting gate
# and an explicit release approval. It never prints JD bodies or passwords.

set -euo pipefail

hostname="${1:-}"
username="${2:-mrolescope}"

die() {
  printf 'Web Edge smoke check failed: %s\n' "$1" >&2
  exit 1
}

[[ "$hostname" =~ ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$ ]] \
  || die 'pass one approved lower-case hostname'
[[ "$username" =~ ^[A-Za-z0-9._-]{1,64}$ ]] || die 'username has an invalid format'
command -v curl >/dev/null 2>&1 || die 'curl is required'
command -v getent >/dev/null 2>&1 || die 'getent is required'

read -r -s -p "HTTP Basic password for ${username}: " password
printf '\n' >&2
trap 'unset password list_body' EXIT

http_headers="$(curl --silent --show-error --include --max-time 10 "http://${hostname}/")"
grep -Eq '^HTTP/1\.[01] 301 ' <<<"$http_headers" || die 'HTTP did not redirect to HTTPS'
grep -Fq "Location: https://${hostname}/" <<<"$http_headers" \
  || die 'HTTP redirect did not target the approved HTTPS hostname'
if grep -Eiq '(__vite|/api/|岗位)' <<<"$http_headers"; then
  die 'HTTP exposed browser assets or JD data'
fi

unauthenticated_status="$(curl --silent --output /dev/null --write-out '%{http_code}' --max-time 10 \
  "https://${hostname}/api/v1/jobs" || true)"
[[ "$unauthenticated_status" == 401 ]] || die 'unauthenticated HTTPS API was not denied'

# This authenticated request verifies the publicly trusted certificate, TLS
# hostname validation, Basic Auth, same-origin API path, and list endpoint.
list_body="$(curl --fail --silent --show-error --proto '=https' --tlsv1.2 --max-time 15 \
  --user "${username}:${password}" "https://${hostname}/api/v1/jobs?limit=1")" \
  || die 'authenticated HTTPS list request failed'
if grep -Eiq 'access-control-allow-origin' <<<"$(curl --fail --silent --show-error --include --output /dev/null --max-time 15 --user "${username}:${password}" "https://${hostname}/api/v1/jobs?limit=1")"; then
  die 'same-origin API unexpectedly returned a permissive CORS header'
fi
job_id="$(sed -nE 's/.*"id":"([^"]+)".*/\1/p' <<<"$list_body" | head -n 1)"
[[ -n "$job_id" ]] || die 'authenticated list response did not contain one opaque JD id'
curl --fail --silent --show-error --proto '=https' --tlsv1.2 --max-time 15 \
  --user "${username}:${password}" --output /dev/null \
  "https://${hostname}/api/v1/jobs/${job_id}" \
  || die 'authenticated HTTPS detail request failed'

# Direct raw-IP HTTPS has no approved SNI hostname and must not become a
# usable product route. A TLS handshake failure or any non-routable response
# is acceptable; a success/redirect/auth challenge is not.
raw_ip="$(getent ahostsv4 "$hostname" | awk 'NR == 1 { print $1 }')"
[[ -n "$raw_ip" ]] || die 'hostname has no IPv4 DNS result for raw-IP denial check'
raw_status="$(curl --insecure --silent --output /dev/null --write-out '%{http_code}' --max-time 10 "https://${raw_ip}/" || true)"
case "$raw_status" in
  000|400|403|404|421|444|495|496|497)
    ;;
  *)
    die "raw IP unexpectedly received HTTP ${raw_status}"
    ;;
esac

printf '%s\n' 'Web Edge smoke check passed'
