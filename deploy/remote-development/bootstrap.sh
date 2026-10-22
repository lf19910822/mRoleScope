#!/usr/bin/env bash
#
# One-time, root-run installer for the remote-development deployment boundary.
# It intentionally downloads only reviewed infrastructure assets from a full,
# immutable Git commit. It never downloads application source, builds an
# image, starts an application service, or accepts a secret.

set -euo pipefail

readonly REPOSITORY='lf19910822/mRoleScope'
readonly COMPOSE_DESTINATION='/opt/mrolescope/remote-development.compose.yaml'
readonly HELPER_DESTINATION='/usr/local/sbin/mrolescope-deploy'
readonly SYNC_HELPER_DESTINATION='/usr/local/sbin/mrolescope-sync'
readonly OPERATIONS_HELPER_DESTINATION='/usr/local/sbin/mrolescope-operations'
readonly SYNC_SERVICE_DESTINATION='/etc/systemd/system/mrolescope-sync.service'
readonly SYNC_TIMER_DESTINATION='/etc/systemd/system/mrolescope-sync.timer'
readonly DEPLOY_CONFIG_DESTINATION='/etc/mrolescope/remote-development.conf'
readonly RELEASE_DIR='/var/lib/mrolescope/releases'
readonly SECRETS_DIR='/etc/mrolescope/secrets'
readonly SUDOERS_DESTINATION='/etc/sudoers.d/mrolescope-deploy'
readonly DEPLOY_USER='mrolescope-deploy'
readonly ASSET_MIRROR="${MROLESCOPE_ASSET_MIRROR:-github-raw}"

die() {
  printf 'mrolescope bootstrap failed: %s\n' "$1" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || die "run this script with sudo as root"
[[ "$#" -eq 2 ]] || die "usage: $0 <full-protected-main-commit-sha> <docker-hub-namespace>"
readonly REF="$1"
readonly DOCKERHUB_NAMESPACE="$2"
[[ "$REF" =~ ^[a-f0-9]{40}$ ]] || die "the commit reference must be a full 40-character lowercase SHA"
[[ "$DOCKERHUB_NAMESPACE" =~ ^[a-z0-9][a-z0-9_-]{0,38}$ ]] \
  || die "the Docker Hub namespace must contain only lowercase letters, digits, hyphens, or underscores"

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v docker >/dev/null 2>&1 || die "Docker Engine and the Compose plugin must be installed first"
docker compose version >/dev/null 2>&1 || die "the Docker Compose plugin is required"
command -v visudo >/dev/null 2>&1 || die "sudo/visudo is required"

staging_dir=$(mktemp -d /tmp/mrolescope-bootstrap.XXXXXX)
cleanup() {
  rm -rf -- "$staging_dir"
}
trap cleanup EXIT

download_asset() {
  local source_path="$1" destination="$2" asset_url

  case "$ASSET_MIRROR" in
    github-raw)
      asset_url="https://raw.githubusercontent.com/${REPOSITORY}/${REF}/${source_path}"
      ;;
    jsdelivr)
      asset_url="https://cdn.jsdelivr.net/gh/${REPOSITORY}@${REF}/${source_path}"
      ;;
    *)
      die "MROLESCOPE_ASSET_MIRROR must be github-raw or jsdelivr"
      ;;
  esac

  printf 'mRoleScope bootstrap: downloading %s via %s...\n' "$source_path" "$ASSET_MIRROR"
  curl --fail --location --proto '=https' --tlsv1.2 --silent --show-error \
    --connect-timeout 10 --max-time 60 \
    "$asset_url" \
    --output "$destination"
}

download_asset 'deploy/remote-development/compose.yaml' "$staging_dir/compose.yaml"
download_asset 'deploy/remote-development/mrolescope-deploy' "$staging_dir/mrolescope-deploy"
download_asset 'deploy/remote-development/mrolescope-sync' "$staging_dir/mrolescope-sync"
download_asset 'deploy/remote-development/mrolescope-operations' "$staging_dir/mrolescope-operations"
download_asset 'deploy/remote-development/mrolescope-sync.service' "$staging_dir/mrolescope-sync.service"
download_asset 'deploy/remote-development/mrolescope-sync.timer' "$staging_dir/mrolescope-sync.timer"
bash -n "$staging_dir/mrolescope-deploy"
bash -n "$staging_dir/mrolescope-sync"
bash -n "$staging_dir/mrolescope-operations"
printf 'DOCKERHUB_NAMESPACE=%s\n' "$DOCKERHUB_NAMESPACE" >"$staging_dir/remote-development.conf"

if ! getent passwd "$DEPLOY_USER" >/dev/null; then
  useradd --system --user-group --home-dir /opt/mrolescope-runner --shell /usr/sbin/nologin "$DEPLOY_USER"
fi
getent group "$DEPLOY_USER" >/dev/null || die "the deployment user's private group is missing"
usermod --gid "$DEPLOY_USER" --groups "$DEPLOY_USER" --shell /usr/sbin/nologin "$DEPLOY_USER"
install -d -o "$DEPLOY_USER" -g "$DEPLOY_USER" -m 0750 /opt/mrolescope-runner

install -d -o root -g root -m 0755 /opt/mrolescope /etc/mrolescope /var/lib/mrolescope
install -d -o root -g root -m 0700 "$SECRETS_DIR" "$RELEASE_DIR"
install -o root -g root -m 0644 "$staging_dir/compose.yaml" "$COMPOSE_DESTINATION"
install -o root -g root -m 0750 "$staging_dir/mrolescope-deploy" "$HELPER_DESTINATION"
install -o root -g root -m 0750 "$staging_dir/mrolescope-sync" "$SYNC_HELPER_DESTINATION"
install -o root -g root -m 0750 "$staging_dir/mrolescope-operations" "$OPERATIONS_HELPER_DESTINATION"
install -o root -g root -m 0644 "$staging_dir/mrolescope-sync.service" "$SYNC_SERVICE_DESTINATION"
install -o root -g root -m 0644 "$staging_dir/mrolescope-sync.timer" "$SYNC_TIMER_DESTINATION"
install -o root -g root -m 0644 "$staging_dir/remote-development.conf" "$DEPLOY_CONFIG_DESTINATION"

sudoers_candidate="$staging_dir/mrolescope-deploy.sudoers"
printf '%s\n' \
  '# Root-owned helpers validate all arguments before invoking Docker or systemd.' \
  'mrolescope-deploy ALL=(root) NOPASSWD: /usr/local/sbin/mrolescope-deploy *, /usr/local/sbin/mrolescope-operations *' \
  >"$sudoers_candidate"
chmod 0440 "$sudoers_candidate"
visudo -cf "$sudoers_candidate" >/dev/null
install -o root -g root -m 0440 "$sudoers_candidate" "$SUDOERS_DESTINATION"
visudo -cf "$SUDOERS_DESTINATION" >/dev/null
systemctl daemon-reload

printf '%s\n' 'mRoleScope remote-development deployment boundary is installed.'
printf '%s\n' 'No application image was pulled and no runtime secret was created.'
printf '%s\n' 'The daily sync timer is installed but remains disabled until a verified initialization run succeeds.'
printf '%s\n' 'Next: ensure the labeled GitHub Actions runner and root-only runtime env file are ready.'
