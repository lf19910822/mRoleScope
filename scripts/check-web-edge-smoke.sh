#!/usr/bin/env bash
# Run only from the operator's terminal after the manual domain/hosting gate
# and an explicit release approval. It never accepts or prints the invite code.

set -euo pipefail

hostname="${1:-}"
readonly cookie_jar="$(mktemp)"
invite_code=''
trap 'rm -f -- "$cookie_jar"; unset invite_code list_body' EXIT

die() {
  printf 'Web Edge smoke check failed: %s\n' "$1" >&2
  exit 1
}

[[ "$hostname" =~ ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$ ]] \
  || die 'pass one approved lower-case hostname'
command -v curl >/dev/null 2>&1 || die 'curl is required'
command -v getent >/dev/null 2>&1 || die 'getent is required'

read -r -s -p "Invite code for ${hostname}: " invite_code
printf '\n' >&2
[[ -n "$invite_code" ]] || die 'invite code cannot be empty'

http_headers="$(curl --silent --show-error --include --max-time 10 "http://${hostname}/")"
grep -Eq '^HTTP/1\.[01] 301 ' <<<"$http_headers" || die 'HTTP did not redirect to HTTPS'
grep -Fq "Location: https://${hostname}/" <<<"$http_headers" \
  || die 'HTTP redirect did not target the approved HTTPS hostname'
if grep -Eiq '(__vite|/api/|岗位)' <<<"$http_headers"; then
  die 'HTTP exposed browser assets or JD data'
fi

access_status="$(curl --silent --output /dev/null --write-out '%{http_code}' --max-time 10 \
  "https://${hostname}/access" || true)"
[[ "$access_status" == 200 ]] || die 'invite page was not available over HTTPS'

unauthenticated_status="$(curl --silent --output /dev/null --write-out '%{http_code}' --max-time 10 \
  "https://${hostname}/api/v1/jobs" || true)"
[[ "$unauthenticated_status" == 401 ]] || die 'unauthenticated HTTPS API was not denied'

[[ "$invite_code" != *'"'* && "$invite_code" != *'\\'* && "$invite_code" != *$'\n'* && "$invite_code" != *$'\r'* ]] \
  || die 'invite code contains unsupported control or JSON delimiter characters'

printf '{"invite_code":"%s"}' "$invite_code" | \
curl --fail --silent --show-error --proto '=https' --tlsv1.2 --max-time 15 \
  --cookie-jar "$cookie_jar" --data-binary @- \
  --header 'Content-Type: application/json' \
  --request POST "https://${hostname}/api/v1/access/verify" \
  --dump-header "$cookie_jar.headers" --output /dev/null \
  || die 'invite verification failed'
grep -Fqi 'Secure' "$cookie_jar.headers" || die 'access cookie is not Secure'
grep -Fqi 'HttpOnly' "$cookie_jar.headers" || die 'access cookie is not HttpOnly'
grep -Fqi 'SameSite=Lax' "$cookie_jar.headers" || die 'access cookie does not use SameSite=Lax'
grep -Fqi 'Max-Age=259200' "$cookie_jar.headers" || die 'access cookie lifetime is not three days'

list_body="$(curl --fail --silent --show-error --proto '=https' --tlsv1.2 --max-time 15 \
  --cookie "$cookie_jar" "https://${hostname}/api/v1/jobs?limit=1")" \
  || die 'authorized HTTPS list request failed'
if grep -Eiq 'access-control-allow-origin' <<<"$(curl --fail --silent --show-error --include --output /dev/null --max-time 15 --cookie "$cookie_jar" "https://${hostname}/api/v1/jobs?limit=1")"; then
  die 'same-origin API unexpectedly returned a permissive CORS header'
fi
job_id="$(sed -nE 's/.*"id":"([^"]+)".*/\1/p' <<<"$list_body" | head -n 1)"
[[ -n "$job_id" ]] || die 'authorized list response did not contain one opaque JD id'
curl --fail --silent --show-error --proto '=https' --tlsv1.2 --max-time 15 \
  --cookie "$cookie_jar" --output /dev/null \
  "https://${hostname}/api/v1/jobs/${job_id}" \
  || die 'authorized HTTPS detail request failed'

expired_status="$(curl --silent --output /dev/null --write-out '%{http_code}' --max-time 10 \
  --cookie 'mrolescope_access=expired' "https://${hostname}/" || true)"
[[ "$expired_status" == 302 ]] || die 'invalid/expired access cookie was not returned to the invite flow'

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
