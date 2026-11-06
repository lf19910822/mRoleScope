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

revision=''

die() {
  printf 'remote release failed: %s\n' "$1" >&2
  exit 1
}

usage() {
  cat >&2 <<'USAGE'
Usage: scripts/release-remote-development.sh --dry-run|--deploy

  --dry-run  Verify that the local checkout is the clean, current main revision
              and print the planned release without building or changing state.
  --deploy   Run release verification, build and push four immutable images,
              refresh reviewed CVM deployment assets, and deploy their digests.
USAGE
  exit 2
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command is unavailable: $1"
}

preflight() {
  local branch remote_revision status

  for command in docker git make sha256sum ssh; do
    require_command "$command"
  done

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
  local image_name="$1" dockerfile="$2" image_tag local_revision

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
  docker push "$image_tag" >&2
  resolve_pushed_digest "$image_name" "$image_tag"
}

refresh_and_deploy() {
  local api_image="$1" migrate_image="$2" worker_image="$3" web_edge_image="$4"
  local bootstrap_sha bootstrap_url bootstrap_fallback remote_command

  bootstrap_sha=$(sha256sum "$repository_root/deploy/remote-development/bootstrap.sh" | awk '{print $1}')
  [[ "$bootstrap_sha" =~ ^[a-f0-9]{64}$ ]] || die 'could not checksum the reviewed CVM bootstrap asset'
  bootstrap_url="https://raw.githubusercontent.com/lf19910822/mRoleScope/${revision}/deploy/remote-development/bootstrap.sh"
  bootstrap_fallback="https://cdn.jsdelivr.net/gh/lf19910822/mRoleScope@${revision}/deploy/remote-development/bootstrap.sh"
  remote_command=$(cat <<REMOTE
set -euo pipefail
bootstrap=\$(mktemp /tmp/mrolescope-bootstrap.XXXXXX)
trap 'rm -f -- "\$bootstrap"' EXIT
if ! curl --fail --location --proto '=https' --tlsv1.2 --silent --show-error --connect-timeout 10 --max-time 60 '${bootstrap_url}' --output "\$bootstrap"; then
  curl --fail --location --proto '=https' --tlsv1.2 --silent --show-error --connect-timeout 10 --max-time 60 '${bootstrap_fallback}' --output "\$bootstrap"
fi
actual=\$(sha256sum "\$bootstrap" | awk '{print \$1}')
[[ "\$actual" == '${bootstrap_sha}' ]]
sudo env MROLESCOPE_ASSET_MIRROR=github-raw bash "\$bootstrap" '${revision}' '${dockerhub_namespace}'
sudo -u mrolescope-deploy sudo -n /usr/local/sbin/mrolescope-deploy '${api_image}' '${migrate_image}' '${worker_image}' '${web_edge_image}'
REMOTE
)
  ssh -o BatchMode=yes -o ConnectTimeout=15 "$remote_host" "$remote_command"
}

[[ "$#" -eq 1 ]] || usage
case "$1" in
  --dry-run)
    preflight
    printf 'Ready to release %s from %s. No images were built or pushed.\n' "$revision" "$repository_root"
    ;;
  --deploy)
    preflight
    make -C "$repository_root" remote-release-verify
    api_image=$(build_and_push 'mrolescope-api' "$repository_root/docker/Dockerfile.api")
    migrate_image=$(build_and_push 'mrolescope-migrate' "$repository_root/docker/Dockerfile.migrate")
    worker_image=$(build_and_push 'mrolescope-worker' "$repository_root/docker/Dockerfile.worker")
    web_edge_image=$(build_and_push 'mrolescope-web-edge' "$repository_root/docker/Dockerfile.web-edge")
    for image in "$api_image" "$migrate_image" "$worker_image" "$web_edge_image"; do
      [[ "$image" =~ $digest_pattern ]] || die 'a release image failed digest validation'
    done
    refresh_and_deploy "$api_image" "$migrate_image" "$worker_image" "$web_edge_image"
    printf 'Remote release completed for %s.\n' "$revision"
    ;;
  *)
    usage
    ;;
esac
