#!/usr/bin/env bash
# Static and shell-level checks for the root-controlled ACME lifecycle. The
# test never contacts an ACME server and never reads a runtime secret.

set -euo pipefail

readonly repository_root="$(cd -- "$(dirname -- "$0")/.." && pwd)"
readonly issue="$repository_root/deploy/remote-development/mrolescope-cert-issue"
readonly renew="$repository_root/deploy/remote-development/mrolescope-cert-renew"
readonly service="$repository_root/deploy/remote-development/mrolescope-cert-renew.service"
readonly timer="$repository_root/deploy/remote-development/mrolescope-cert-renew.timer"
readonly bootstrap="$repository_root/deploy/remote-development/bootstrap.sh"

die() {
  printf 'certificate operations check failed: %s\n' "$1" >&2
  exit 1
}

for path in "$issue" "$renew" "$service" "$timer" "$bootstrap"; do
  [[ -f "$path" ]] || die "missing asset: $path"
done
bash -n "$issue" "$renew" "$bootstrap"
grep -Fqx '  --standalone \' "$issue" || die 'initial issuance must use Certbot standalone HTTP-01'
grep -Fqx '  --http-01-port 80 \' "$issue" || die 'initial issuance must reserve TCP 80'
grep -Fq 'stop web-edge' "$issue" || die 'initial issuance must stop Web Edge first'
grep -Fq 'MROLESCOPE_WEB_HOSTNAME' "$issue" || die 'initial issuance must use the approved runtime hostname'
grep -Fq -- '--webroot' "$renew" || die 'renewal must use Certbot webroot mode'
grep -Fq -- '--webroot-path "$ACME_CHALLENGE_DIR"' "$renew" || die 'renewal must use the fixed challenge directory'
grep -Fq 'previous certificate restored' "$renew" || die 'renewal must describe the fallback path'
grep -Fq 'nginx -s reload' "$renew" || die 'renewal must reload Web Edge after replacement'
grep -Fq 'ReadWritePaths=/etc/mrolescope/tls /var/lib/mrolescope/acme /var/lib/mrolescope/acme-challenge' "$service" \
  || die 'renewal service write paths are not fixed'
grep -Fq 'Unit=mrolescope-cert-renew.service' "$timer" || die 'timer does not call the renewal service'
grep -Fq 'OnCalendar=' "$timer" || die 'timer has no schedule'
if grep -Eiq 'MROLESCOPE_INVITE_BCRYPT_HASH|MROLESCOPE_SESSION_SIGNING_SECRET|invite_code' "$issue" "$renew" "$service" "$timer"; then
  die 'certificate assets must not receive invite or session secrets'
fi
grep -Fq "mrolescope-cert-renew.timer" "$bootstrap" || die 'bootstrap does not install the renewal timer'
grep -Fq "mrolescope-cert-issue" "$bootstrap" || die 'bootstrap does not install the issuance helper'
printf '%s\n' 'certificate operations check passed'
