#!/usr/bin/env bash
# Build the reviewed main revision locally, publish immutable images, and
# release them through the CVM's root-owned digest allowlist.

set -euo pipefail

readonly repository_root="$(cd -- "$(dirname -- "$0")/.." && pwd)"
readonly dockerhub_namespace='daud1991'
readonly remote_host='root@118.89.68.77'
readonly revision_label='org.opencontainers.image.revision'
readonly source_label='org.opencontainers.image.source'
readonly source_url='https://github.com/lf19910822/mRoleScope'
readonly sha_pattern='^[a-f0-9]{40}$'
readonly digest_pattern='^docker\.io/daud1991/mrolescope-(api|migrate|worker|web-edge)@sha256:[a-f0-9]{64}$'
readonly asset_fingerprint_pattern='^[a-f0-9]{64}$'

revision=''
pinned_postgres_image=''

now_milliseconds() {
  date +%s%3N
}

log_phase_duration() {
  local phase="$1" started_at="$2" ended_at
  ended_at=$(now_milliseconds)
  printf 'release timing: phase=%s elapsed_ms=%s\n' "$phase" "$((ended_at - started_at))"
}

die() {
  printf 'remote release failed: %s\n' "$1" >&2
  exit 1
}

usage() {
  cat >&2 <<'USAGE'
Usage: scripts/release-remote-development.sh --dry-run|--deploy

  --dry-run  Verify that the local checkout is the clean, current main revision
              and print the planned release without building or changing state.
  --deploy   Preflight the CVM Docker pull path, run release verification,
              build and push four immutable images, refresh reviewed CVM
              deployment assets, and deploy their digests.
USAGE
  exit 2
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command is unavailable: $1"
}

resolve_pinned_postgres_image() {
  local -a candidates=()

  mapfile -t candidates < <(
    sed -nE 's/^    image: (postgres:[^[:space:]]+@sha256:[a-f0-9]{64})$/\1/p' \
      "$repository_root/deploy/remote-development/compose.yaml"
  )
  [[ "${#candidates[@]}" -eq 1 ]] \
    || die 'the reviewed Compose definition must contain exactly one pinned PostgreSQL image'
  pinned_postgres_image="${candidates[0]}"
}

deployment_asset_fingerprint() {
  bash "$repository_root/scripts/hash-deployment-assets.sh" "$repository_root/deploy/remote-development"
}

preflight() {
  local branch remote_revision status

  for command in docker git make sha256sum ssh; do
    require_command "$command"
  done

  resolve_pinned_postgres_image
  git -C "$repository_root" fetch --quiet origin main
  status=$(git -C "$repository_root" status --porcelain)
  [[ -z "$status" ]] || die 'the worktree is not clean; commit or stash release-unrelated changes first'
  branch=$(git -C "$repository_root" branch --show-current)
  [[ "$branch" == main ]] || die 'releases must be built from the local main branch'
  revision=$(git -C "$repository_root" rev-parse HEAD)
  remote_revision=$(git -C "$repository_root" rev-parse refs/remotes/origin/main)
  [[ "$revision" =~ $sha_pattern ]] || die 'HEAD is not a full lowercase Git revision'
  [[ "$revision" == "$remote_revision" ]] || die 'local main does not exactly match origin/main; run git pull --ff-only'
}

remote_registry_preflight() {
  local remote_command

  remote_command=$(cat <<REMOTE
set -euo pipefail
printf '%s\n' 'CVM Docker daemon pull preflight: ${pinned_postgres_image}'
mirrors=\$(docker info --format '{{range .RegistryConfig.Mirrors}}{{println .}}{{end}}' 2>/dev/null || true)
if [[ -n "\$mirrors" ]]; then
  printf '%s\n' 'CVM Docker daemon registry mirrors:'
  printf '%s\n' "\$mirrors"
else
  printf '%s\n' 'CVM Docker daemon registry mirrors: none configured'
fi
command -v timeout >/dev/null 2>&1 || {
  printf '%s\n' 'CVM Docker daemon pull preflight failed: timeout is unavailable' >&2
  exit 1
}
if ! timeout 120 docker pull '${pinned_postgres_image}'; then
  printf '%s\n' 'CVM Docker daemon pull preflight failed for the pinned PostgreSQL image' >&2
  printf '%s\n' 'This release gate uses the Docker daemon pull path; direct Registry probes are not used as evidence.' >&2
  exit 1
fi
printf '%s\n' 'CVM Docker daemon pull preflight passed'
REMOTE
)
  ssh -o BatchMode=yes -o ConnectTimeout=15 "$remote_host" "$remote_command"
}

resolve_pushed_digest() {
  local image_name="$1" image_tag="$2" candidate digest=''

  while IFS= read -r candidate; do
    candidate="${candidate#docker.io/}"
    candidate="docker.io/${candidate}"
    if [[ "$candidate" =~ ^docker\.io/${dockerhub_namespace}/${image_name}@sha256:[a-f0-9]{64}$ ]]; then
      digest="$candidate"
      break
    fi
  done < <(docker image inspect --format '{{range .RepoDigests}}{{println .}}{{end}}' "$image_tag")

  [[ "$digest" =~ $digest_pattern ]] || die "Docker did not return an immutable pushed digest for ${image_name}"
  printf '%s\n' "$digest"
}

build_and_push() {
  local image_name="$1" dockerfile="$2" image_tag local_revision push_log pushed_digest

  image_tag="docker.io/${dockerhub_namespace}/${image_name}:${revision}"
  printf 'Building %s…\n' "$image_name" >&2
  docker build --pull --platform linux/amd64 \
    --file "$dockerfile" \
    --label "${revision_label}=${revision}" \
    --label "${source_label}=${source_url}" \
    --tag "$image_tag" \
    "$repository_root" >&2
  local_revision=$(docker image inspect --format "{{index .Config.Labels \"${revision_label}\"}}" "$image_tag")
  [[ "$local_revision" == "$revision" ]] \
    || die "${image_name} is missing the expected immutable source revision label"
  printf 'Pushing %s…\n' "$image_name" >&2
  push_log=$(mktemp "${TMPDIR:-/tmp}/mrolescope-push.XXXXXX")
  if ! docker push "$image_tag" 2>&1 | tee "$push_log" >&2; then
    rm -f -- "$push_log"
    die "Docker push failed for ${image_name}"
  fi
  pushed_digest=$(sed -nE 's/.*digest: (sha256:[a-f0-9]{64}).*/\1/p' "$push_log" | tail -n 1)
  rm -f -- "$push_log"
  if [[ "$pushed_digest" =~ ^sha256:[a-f0-9]{64}$ ]]; then
    printf 'docker.io/%s/%s@%s\n' "$dockerhub_namespace" "$image_name" "$pushed_digest"
    return 0
  fi
  # Older/mock Docker clients may omit the push digest; retain the local
  # RepoDigests fallback for those clients.
  resolve_pushed_digest "$image_name" "$image_tag"
}

refresh_assets() {
  local asset_fingerprint="$1"
  local bootstrap_sha bootstrap_url bootstrap_fallback remote_command

  [[ "$asset_fingerprint" =~ $asset_fingerprint_pattern ]] \
    || die 'deployment asset fingerprint is invalid'

  bootstrap_sha=$(sha256sum "$repository_root/deploy/remote-development/bootstrap.sh" | awk '{print $1}')
  [[ "$bootstrap_sha" =~ ^[a-f0-9]{64}$ ]] || die 'could not checksum the reviewed CVM bootstrap asset'
  bootstrap_url="https://raw.githubusercontent.com/lf19910822/mRoleScope/${revision}/deploy/remote-development/bootstrap.sh"
  bootstrap_fallback="https://cdn.jsdelivr.net/gh/lf19910822/mRoleScope@${revision}/deploy/remote-development/bootstrap.sh"
  remote_command=$(cat <<REMOTE
set -euo pipefail
asset_marker='/opt/mrolescope/remote-development.assets.sha256'
if [[ -f "\$asset_marker" && ! -L "\$asset_marker" ]] && [[ "\$(stat -c '%U:%G:%a' "\$asset_marker" 2>/dev/null || true)" == 'root:root:644' ]] && [[ "\$(cat "\$asset_marker")" == '${asset_fingerprint}' ]]; then
  printf '%s\n' 'mRoleScope deployment assets unchanged; bootstrap download skipped.'
  exit 0
fi
bootstrap=\$(mktemp /tmp/mrolescope-bootstrap.XXXXXX)
trap 'rm -f -- "\$bootstrap"' EXIT
if ! curl --fail --location --proto '=https' --tlsv1.2 --silent --show-error --connect-timeout 10 --max-time 60 '${bootstrap_url}' --output "\$bootstrap"; then
  curl --fail --location --proto '=https' --tlsv1.2 --silent --show-error --connect-timeout 10 --max-time 60 '${bootstrap_fallback}' --output "\$bootstrap"
fi
actual=\$(sha256sum "\$bootstrap" | awk '{print \$1}')
[[ "\$actual" == '${bootstrap_sha}' ]]
sudo env MROLESCOPE_ASSET_MIRROR=github-raw bash "\$bootstrap" '${revision}' '${dockerhub_namespace}' '${asset_fingerprint}'
REMOTE
)
  ssh -o BatchMode=yes -o ConnectTimeout=15 "$remote_host" "$remote_command"
}

deploy_remote() {
  local api_image="$1" migrate_image="$2" worker_image="$3" web_edge_image="$4"
  ssh -o BatchMode=yes -o ConnectTimeout=15 "$remote_host" \
    "sudo -u mrolescope-deploy sudo -n /usr/local/sbin/mrolescope-deploy '${api_image}' '${migrate_image}' '${worker_image}' '${web_edge_image}'"
}

[[ "$#" -eq 1 ]] || usage
case "$1" in
  --dry-run)
    preflight
    printf 'Ready to release %s from %s. No images were built or pushed.\n' "$revision" "$repository_root"
    ;;
  --deploy)
    release_started_at=$(now_milliseconds)
    phase_started_at=$(now_milliseconds)
    preflight
    log_phase_duration 'preflight' "$phase_started_at"
    phase_started_at=$(now_milliseconds)
    remote_registry_preflight
    log_phase_duration 'remote-registry-preflight' "$phase_started_at"
    asset_fingerprint=$(deployment_asset_fingerprint)
    [[ "$asset_fingerprint" =~ $asset_fingerprint_pattern ]] || die 'could not compute deployment asset fingerprint'
    phase_started_at=$(now_milliseconds)
    make -C "$repository_root" remote-release-verify
    log_phase_duration 'quality-gate' "$phase_started_at"
    phase_started_at=$(now_milliseconds)
    api_image=$(build_and_push 'mrolescope-api' "$repository_root/docker/Dockerfile.api")
    migrate_image=$(build_and_push 'mrolescope-migrate' "$repository_root/docker/Dockerfile.migrate")
    worker_image=$(build_and_push 'mrolescope-worker' "$repository_root/docker/Dockerfile.worker")
    web_edge_image=$(build_and_push 'mrolescope-web-edge' "$repository_root/docker/Dockerfile.web-edge")
    log_phase_duration 'image-publication' "$phase_started_at"
    for image in "$api_image" "$migrate_image" "$worker_image" "$web_edge_image"; do
      [[ "$image" =~ $digest_pattern ]] || die 'a release image failed digest validation'
    done
    phase_started_at=$(now_milliseconds)
    refresh_assets "$asset_fingerprint"
    log_phase_duration 'asset-refresh' "$phase_started_at"
    phase_started_at=$(now_milliseconds)
    deploy_remote "$api_image" "$migrate_image" "$worker_image" "$web_edge_image"
    log_phase_duration 'remote-deployment' "$phase_started_at"
    log_phase_duration 'total' "$release_started_at"
    printf 'Remote release completed for %s.\n' "$revision"
    ;;
  *)
    usage
    ;;
esac
