#!/usr/bin/env bash
# Exercise the Web Edge from outside the container.  The test deliberately
# proves the negative boundary first: HTTP, unauthenticated HTTPS, and an
# unknown Host must not reveal browser assets or API data.

set -euo pipefail

readonly repository_root="$(cd -- "$(dirname -- "$0")/.." && pwd)"
readonly image_tag='mrolescope-web-edge-container-test:local'
readonly container_name="mrolescope-web-edge-test-$$"
readonly hostname='jobs.example.test'
readonly username='mrolescope'
readonly password='local-test-password'
# bcrypt for local-test-password only; it is intentionally neither a runtime
# secret nor a production credential.
readonly bcrypt_hash='$2b$05$lU6JnhJzxffWpA4E/ips0uEq2FHVSvClisOk.iZC5QrLSaeIYb3BW'
temporary_dir="$(mktemp -d)"

cleanup() {
  docker rm -f "$container_name" >/dev/null 2>&1 || true
  rm -rf -- "$temporary_dir"
}
trap cleanup EXIT

die() {
  printf 'Web Edge container test failed: %s\n' "$1" >&2
  exit 1
}

command -v docker >/dev/null 2>&1 || die 'docker is required'
command -v curl >/dev/null 2>&1 || die 'curl is required'
command -v openssl >/dev/null 2>&1 || die 'openssl is required'

mkdir -p "$temporary_dir/tls" "$temporary_dir/acme"
openssl req -x509 -newkey rsa:2048 -nodes -sha256 -days 1 \
  -subj "/CN=${hostname}" \
  -keyout "$temporary_dir/tls/privkey.pem" \
  -out "$temporary_dir/tls/fullchain.pem" >/dev/null 2>&1

docker build --tag "$image_tag" --file "$repository_root/docker/Dockerfile.web-edge" "$repository_root"
if docker image inspect "$image_tag" | grep -Fq "$bcrypt_hash" \
  || docker history --no-trunc "$image_tag" | grep -Fq "$bcrypt_hash"; then
  die 'runtime bcrypt hash was retained in image metadata or a layer command'
fi
docker run --detach --name "$container_name" \
  --publish '127.0.0.1::80' --publish '127.0.0.1::443' \
  --add-host 'api:127.0.0.1' \
  --env "MROLESCOPE_WEB_HOSTNAME=${hostname}" \
  --env "MROLESCOPE_WEB_AUTH_BCRYPT_HASH=${bcrypt_hash}" \
  --mount "type=bind,src=$temporary_dir/tls,dst=/etc/nginx/tls,readonly" \
  --mount "type=bind,src=$temporary_dir/acme,dst=/usr/share/nginx/acme-challenge,readonly" \
  "$image_tag" >/dev/null

for attempt in $(seq 1 30); do
  if docker inspect --format '{{.State.Running}}' "$container_name" 2>/dev/null | grep -qx true; then
    break
  fi
  sleep 1
done
docker inspect --format '{{.State.Running}}' "$container_name" | grep -qx true \
  || {
    docker logs "$container_name" >&2 || true
    die 'container did not stay running'
  }

http_port="$(docker port "$container_name" 80/tcp | sed -nE 's/.*:([0-9]+)$/\1/p')"
https_port="$(docker port "$container_name" 443/tcp | sed -nE 's/.*:([0-9]+)$/\1/p')"
[[ "$http_port" =~ ^[0-9]+$ && "$https_port" =~ ^[0-9]+$ ]] || die 'published ports were not discoverable'

# Give Nginx a moment to bind after its configuration validation. The domain's
# HTTP endpoint may only redirect (or serve an ACME challenge);
# it must not serve the SPA or proxy its API before TLS authentication.
for attempt in $(seq 1 10); do
  if curl --silent --output /dev/null --max-time 1 \
    --resolve "${hostname}:${http_port}:127.0.0.1" \
    "http://${hostname}:${http_port}/"; then
    break
  fi
  sleep 1
done
if ! docker inspect --format '{{.State.Running}}' "$container_name" | grep -qx true; then
  docker logs "$container_name" >&2 || true
  die 'container stopped before accepting HTTP connections'
fi

# it must not serve the SPA or proxy its API before TLS authentication.
http_headers="$(curl --silent --show-error --include --max-time 5 \
  --resolve "${hostname}:${http_port}:127.0.0.1" \
  "http://${hostname}:${http_port}/")"
grep -Eq '^HTTP/1\.[01] 301 ' <<<"$http_headers" || die 'HTTP did not redirect to HTTPS'
if grep -Eiq '(__vite|/api/|岗位)' <<<"$http_headers"; then
  die 'HTTP redirect exposed browser assets or JD data'
fi

unauthenticated_root="$(curl --insecure --silent --show-error --include --max-time 5 \
  --resolve "${hostname}:${https_port}:127.0.0.1" \
  "https://${hostname}:${https_port}/")"
grep -Eq '^HTTP/1\.[01] 401 ' <<<"$unauthenticated_root" || die 'unauthenticated HTTPS root was not denied'
if grep -Eiq '(__vite|岗位)' <<<"$unauthenticated_root"; then
  die 'unauthenticated HTTPS root exposed browser assets'
fi

unauthenticated_api="$(curl --insecure --silent --show-error --include --max-time 5 \
  --resolve "${hostname}:${https_port}:127.0.0.1" \
  "https://${hostname}:${https_port}/api/v1/jobs")"
grep -Eq '^HTTP/1\.[01] 401 ' <<<"$unauthenticated_api" || die 'unauthenticated HTTPS API was not denied'
if grep -Eiq '(job_id|"title"|岗位)' <<<"$unauthenticated_api"; then
  die 'unauthenticated HTTPS API exposed JD data'
fi

unknown_host="$(curl --insecure --silent --show-error --include --max-time 5 \
  --resolve "not-the-approved-host.example:${https_port}:127.0.0.1" \
  "https://not-the-approved-host.example:${https_port}/" || true)"
if grep -Eq '^HTTP/1\.[01] (200|301|302|401) ' <<<"$unknown_host"; then
  die 'unknown HTTPS Host received a routable response'
fi

authenticated_root="$(curl --insecure --silent --show-error --include --max-time 5 \
  --user "${username}:${password}" --resolve "${hostname}:${https_port}:127.0.0.1" \
  "https://${hostname}:${https_port}/")"
grep -Eq '^HTTP/1\.[01] 200 ' <<<"$authenticated_root" || die 'authenticated HTTPS root was not served'

# A short authenticated burst must be bounded at the edge too. Sending these
# in parallel avoids coupling this check to client-side request pacing.
rate_results="$temporary_dir/rate-results"
for attempt in $(seq 1 32); do
  curl --insecure --silent --max-time 5 --output /dev/null --write-out '%{http_code}\n' \
    --user "${username}:${password}" --resolve "${hostname}:${https_port}:127.0.0.1" \
    "https://${hostname}:${https_port}/" >>"$rate_results" &
done
wait
grep -qx '429' "$rate_results" || die 'edge rate limit did not bound an authenticated burst'

printf '%s\n' 'Web Edge container boundary test passed'
