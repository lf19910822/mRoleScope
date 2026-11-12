#!/usr/bin/env bash
# Exercise the Web Edge from outside the container.  The test deliberately
# proves the negative boundary first: HTTP, unauthenticated HTTPS, and an
# unknown Host must not reveal browser assets or API data.

set -euo pipefail

readonly repository_root="$(cd -- "$(dirname -- "$0")/.." && pwd)"
readonly image_tag='mrolescope-web-edge-container-test:local'
readonly container_name="mrolescope-web-edge-test-$$"
readonly hostname='jobs.example.test'
temporary_dir="$(mktemp -d)"
mock_pid=''
mock_port=18081

cleanup() {
  if [[ -n "$mock_pid" ]]; then
    kill "$mock_pid" >/dev/null 2>&1 || true
  fi
  docker rm -f "$container_name" >/dev/null 2>&1 || true
  rm -rf -- "$temporary_dir"
}
trap cleanup EXIT

die() {
  if [[ -n "${container_name:-}" ]] && docker inspect "$container_name" >/dev/null 2>&1; then
    docker logs "$container_name" >&2 || true
  fi
  printf 'Web Edge container test failed: %s\n' "$1" >&2
  exit 1
}

command -v docker >/dev/null 2>&1 || die 'docker is required'
command -v curl >/dev/null 2>&1 || die 'curl is required'
command -v openssl >/dev/null 2>&1 || die 'openssl is required'
command -v python3 >/dev/null 2>&1 || die 'python3 is required'
command -v nc >/dev/null 2>&1 || die 'nc is required'

mkdir -p "$temporary_dir/tls" "$temporary_dir/acme"
openssl req -x509 -newkey rsa:2048 -nodes -sha256 -days 1 \
  -subj "/CN=${hostname}" \
  -keyout "$temporary_dir/tls/privkey.pem" \
  -out "$temporary_dir/tls/fullchain.pem" >/dev/null 2>&1

while nc -z 127.0.0.1 "$mock_port" 2>/dev/null; do
  mock_port=$((mock_port + 1))
done
python3 "$repository_root/scripts/test-web-edge-mock-api.py" "$mock_port" >/dev/null 2>&1 &
mock_pid=$!
for attempt in $(seq 1 10); do
  if nc -z 127.0.0.1 "$mock_port" 2>/dev/null; then
    break
  fi
  sleep 1
done
nc -z 127.0.0.1 "$mock_port" 2>/dev/null || die 'mock API did not start'
sed "s/api:8080/host.docker.internal:${mock_port}/g" \
  "$repository_root/docker/web-edge/nginx.conf.template" >"$temporary_dir/nginx.conf.template"

docker build --tag "$image_tag" --file "$repository_root/docker/Dockerfile.web-edge" "$repository_root"
docker run --detach --name "$container_name" \
  --publish '127.0.0.1::80' --publish '127.0.0.1::443' \
  --add-host 'host.docker.internal:host-gateway' \
  --env "MROLESCOPE_WEB_HOSTNAME=${hostname}" \
  --mount "type=bind,src=$temporary_dir/nginx.conf.template,dst=/etc/nginx/nginx.conf.template,readonly" \
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
# it must not serve the SPA or proxy its API before invite authentication.
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

access_page="$(curl --insecure --silent --show-error --include --max-time 5 \
  --resolve "${hostname}:${https_port}:127.0.0.1" \
  "https://${hostname}:${https_port}/access")"
grep -Eq '^HTTP/1\.[01] 200 ' <<<"$access_page" || die 'invite page was not served anonymously'
grep -Fq '输入邀请码' <<<"$access_page" || die 'invite page content was not served'

access_css="$(curl --insecure --silent --show-error --include --max-time 5 \
  --resolve "${hostname}:${https_port}:127.0.0.1" \
  "https://${hostname}:${https_port}/access/access.css")"
grep -Eq '^HTTP/1\.[01] 200 ' <<<"$access_css" || die 'invite CSS was not served anonymously'

business_asset="$(curl --insecure --silent --show-error --include --max-time 5 \
  --resolve "${hostname}:${https_port}:127.0.0.1" \
  "https://${hostname}:${https_port}/assets/business.js")"
grep -Eq '^HTTP/1\.[01] 302 ' <<<"$business_asset" || die 'anonymous business asset was not redirected'

unauthenticated_root="$(curl --insecure --silent --show-error --include --max-time 5 \
  --resolve "${hostname}:${https_port}:127.0.0.1" \
  "https://${hostname}:${https_port}/")"
grep -Eq '^HTTP/1\.[01] 302 ' <<<"$unauthenticated_root" || die 'unauthenticated HTTPS root was not redirected'
grep -Eq "^Location: (https://${hostname}/access|/access)([[:space:]]|$)" <<<"$unauthenticated_root" \
  || die 'unauthenticated root did not redirect to invite page'
if grep -Eiq '(__vite|岗位)' <<<"$unauthenticated_root"; then
  die 'unauthenticated HTTPS root exposed business assets'
fi

unauthenticated_api="$(curl --insecure --silent --show-error --include --max-time 5 \
  --resolve "${hostname}:${https_port}:127.0.0.1" \
  "https://${hostname}:${https_port}/api/v1/jobs")"
grep -Eq '^HTTP/1\.[01] 401 ' <<<"$unauthenticated_api" || die 'unauthenticated HTTPS API was not denied'
if grep -Eiq '(job_id|"title"|岗位)' <<<"$unauthenticated_api"; then
  die 'unauthenticated HTTPS API exposed JD data'
fi

printf '%s' '{"invite_code":"local-test-invite"}' | \
curl --insecure --silent --show-error --max-time 5 --include \
  --resolve "${hostname}:${https_port}:127.0.0.1" \
  --cookie-jar "$temporary_dir/cookies.txt" \
  --header 'Content-Type: application/json' --request POST \
  --data-binary @- "https://${hostname}:${https_port}/api/v1/access/verify" >"$temporary_dir/verify-response"
grep -Eq '^HTTP/1\.[01] 204 ' "$temporary_dir/verify-response" || die 'invite verification was not proxied'
grep -Eq '[[:space:]]mrolescope_access[[:space:]]' "$temporary_dir/cookies.txt" \
  || die 'verification cookie was not persisted'

authorized_root="$(curl --insecure --silent --show-error --include --max-time 5 \
  --resolve "${hostname}:${https_port}:127.0.0.1" --cookie "$temporary_dir/cookies.txt" \
  "https://${hostname}:${https_port}/")"
grep -Eq '^HTTP/1\.[01] 200 ' <<<"$authorized_root" || die 'authorized root was not served'

authorized_api="$(curl --insecure --silent --show-error --include --max-time 5 \
  --resolve "${hostname}:${https_port}:127.0.0.1" --cookie "$temporary_dir/cookies.txt" \
  "https://${hostname}:${https_port}/api/v1/jobs")"
grep -Eq '^HTTP/1\.[01] 200 ' <<<"$authorized_api" || die 'authorized API was not served'
grep -Fq 'mock-job' <<<"$authorized_api" || die 'authorized API response was not returned'

unknown_host="$(curl --insecure --silent --show-error --include --max-time 5 \
  --resolve "not-the-approved-host.example:${https_port}:127.0.0.1" \
  "https://not-the-approved-host.example:${https_port}/" || true)"
if grep -Eq '^HTTP/1\.[01] (200|301|302|401) ' <<<"$unknown_host"; then
  die 'unknown HTTPS Host received a routable response'
fi

# A short verification burst must be bounded at the edge too. Keep this check
# sequential so the test itself cannot hang while reaping many curl children.
rate_results="$temporary_dir/rate-results"
for attempt in $(seq 1 8); do
  curl --insecure --silent --connect-timeout 1 --max-time 5 --output /dev/null --write-out '%{http_code}\n' \
    --resolve "${hostname}:${https_port}:127.0.0.1" \
    --request POST --header 'Content-Type: application/json' --data '{"invite_code":"invalid"}' \
    "https://${hostname}:${https_port}/api/v1/access/verify" >>"$rate_results" || true
done
grep -qx '429' "$rate_results" || die 'edge rate limit did not bound an authenticated burst'

printf '%s\n' 'Web Edge container boundary test passed'
