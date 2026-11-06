#!/usr/bin/env bash
# Run the release-quality checks locally. This script owns only the
# disposable verification database named below; it never touches the project's
# development Compose database or a remote environment.

set -euo pipefail

readonly repository_root="$(cd -- "$(dirname -- "$0")/.." && pwd)"
readonly postgres_image='postgres:16.4-alpine@sha256:5660c2cbfea50c7a9127d17dc4e48543eedd3d7a41a595a2dfa572471e37e64c'
readonly migrate_image='migrate/migrate:v4.19.1@sha256:cc4ad8e19d66791e3689405d9a028ce6e9614f32032db14acda1469f7201d6e4'
readonly test_port="${MROLESCOPE_RELEASE_TEST_PORT:-54329}"
readonly container_name="mrolescope-release-verify-$$"
readonly verification_dir="$(mktemp -d "${TMPDIR:-/tmp}/mrolescope-release-verify.XXXXXX")"
readonly database_url="postgres://mrolescope:ci-only-password@127.0.0.1:${test_port}/mrolescope_ci?sslmode=disable"

die() {
  printf 'remote release verification failed: %s\n' "$1" >&2
  exit 1
}

cleanup() {
  docker rm --force -- "$container_name" >/dev/null 2>&1 || true
  rm -rf -- "$verification_dir"
}
trap cleanup EXIT

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command is unavailable: $1"
}

[[ "$test_port" =~ ^[0-9]+$ ]] && ((test_port >= 1024 && test_port <= 65535)) \
  || die 'MROLESCOPE_RELEASE_TEST_PORT must be an unprivileged TCP port'

for command in docker git go make npm python3 sqlc; do
  require_command "$command"
done

cd "$repository_root"
export GOCACHE="$verification_dir/go-build"
mkdir -p "$GOCACHE"

sqlc generate
git diff --exit-code -- internal/database/sqlc
make test
make deploy-config-check

docker run --detach --rm --name "$container_name" \
  --env POSTGRES_DB=mrolescope_ci \
  --env POSTGRES_USER=mrolescope \
  --env POSTGRES_PASSWORD=ci-only-password \
  --publish "127.0.0.1:${test_port}:5432" \
  "$postgres_image" >/dev/null

for ((attempt = 1; attempt <= 30; attempt += 1)); do
  if docker exec "$container_name" pg_isready -U mrolescope -d mrolescope_ci >/dev/null 2>&1; then
    break
  fi
  if [[ "$attempt" -eq 30 ]]; then
    die 'the disposable PostgreSQL verification database did not become ready'
  fi
  sleep 1
done

docker run --rm --network host \
  --mount "type=bind,source=${repository_root}/migrations,target=/migrations,readonly" \
  "$migrate_image" -path=/migrations -database="$database_url" up
TEST_DATABASE_URL="$database_url" go test -p 1 ./internal/jdanalysis ./internal/jobsync -count=1

(
  cd web
  npm ci
  npm run type-check
  npm run test:unit -- --run
  npm run build
)

bash scripts/test-web-edge-container.sh
git diff --check
printf '%s\n' 'Remote release verification passed'
