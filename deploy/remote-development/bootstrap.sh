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
readonly ANALYSIS_HELPER_DESTINATION='/usr/local/sbin/mrolescope-analysis'
readonly BACKFILL_HELPER_DESTINATION='/usr/local/sbin/mrolescope-backfill-versioned-content'
readonly DICTIONARY_HELPER_DESTINATION='/usr/local/sbin/mrolescope-dictionary'
readonly RELEASE_HELPER_DESTINATION='/usr/local/sbin/mrolescope-release'
readonly AI_ANALYSIS_HELPER_DESTINATION='/usr/local/sbin/mrolescope-ai-analysis'
readonly OPERATIONS_HELPER_DESTINATION='/usr/local/sbin/mrolescope-operations'
readonly SYNC_SERVICE_DESTINATION='/etc/systemd/system/mrolescope-sync.service'
readonly SYNC_TIMER_DESTINATION='/etc/systemd/system/mrolescope-sync.timer'
readonly ANALYSIS_SERVICE_DESTINATION='/etc/systemd/system/mrolescope-analysis.service'
readonly CANDIDATE_ANALYSIS_SERVICE_DESTINATION='/etc/systemd/system/mrolescope-candidate-analysis.service'
readonly CANDIDATE_V02_ANALYSIS_SERVICE_DESTINATION='/etc/systemd/system/mrolescope-candidate-analysis-v0.2.service'
readonly BACKFILL_SERVICE_DESTINATION='/etc/systemd/system/mrolescope-backfill-versioned-content.service'
readonly AI_TRIAL_PREPARE_SERVICE_DESTINATION='/etc/systemd/system/mrolescope-ai-trial-prepare.service'
readonly AI_PLUS_TRIAL_SERVICE_DESTINATION='/etc/systemd/system/mrolescope-ai-plus-trial.service'
readonly AI_FLASH_TRIAL_SERVICE_DESTINATION='/etc/systemd/system/mrolescope-ai-flash-trial.service'
readonly AI_PLUS_EXPAND_SERVICE_DESTINATION='/etc/systemd/system/mrolescope-ai-plus-expand.service'
readonly AI_PLUS_PUBLISH_SERVICE_DESTINATION='/etc/systemd/system/mrolescope-ai-plus-publish.service'
readonly DEPLOY_CONFIG_DESTINATION='/etc/mrolescope/remote-development.conf'
readonly RELEASE_DIR='/var/lib/mrolescope/releases'
readonly OUTPUT_AUDIT_DIR='/var/lib/mrolescope/ai-output-audit'
readonly SECRETS_DIR='/etc/mrolescope/secrets'
readonly SUDOERS_DESTINATION='/etc/sudoers.d/mrolescope-deploy'
readonly DEPLOY_USER='mrolescope-deploy'
readonly ASSET_MIRROR="${MROLESCOPE_ASSET_MIRROR:-github-raw}"
readonly ASSET_DOWNLOAD_ATTEMPTS=2

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

case "$ASSET_MIRROR" in
  github-raw|jsdelivr)
    ;;
  *)
    die "MROLESCOPE_ASSET_MIRROR must be github-raw or jsdelivr"
    ;;
esac

staging_dir=$(mktemp -d /tmp/mrolescope-bootstrap.XXXXXX)
cleanup() {
  rm -rf -- "$staging_dir"
}
trap cleanup EXIT

asset_url_for_mirror() {
  local mirror="$1" source_path="$2"

  case "$mirror" in
    github-raw)
      printf '%s\n' "https://raw.githubusercontent.com/${REPOSITORY}/${REF}/${source_path}"
      ;;
    jsdelivr)
      printf '%s\n' "https://cdn.jsdelivr.net/gh/${REPOSITORY}@${REF}/${source_path}"
      ;;
    *)
      die "unknown reviewed asset mirror: ${mirror}"
      ;;
  esac
}

download_asset_from_mirror() {
  local mirror="$1" source_path="$2" destination="$3" asset_url partial_destination attempt
  asset_url=$(asset_url_for_mirror "$mirror" "$source_path")
  partial_destination="${destination}.partial"

  for ((attempt = 1; attempt <= ASSET_DOWNLOAD_ATTEMPTS; attempt++)); do
    rm -f -- "$partial_destination"
    printf 'mRoleScope bootstrap: downloading %s via %s (attempt %d/%d)...\n' \
      "$source_path" "$mirror" "$attempt" "$ASSET_DOWNLOAD_ATTEMPTS"
    if curl --fail --location --proto '=https' --tlsv1.2 --silent --show-error \
      --connect-timeout 10 --max-time 60 \
      "$asset_url" \
      --output "$partial_destination" \
      && [[ -s "$partial_destination" ]]; then
      mv -- "$partial_destination" "$destination"
      return 0
    fi

    rm -f -- "$partial_destination"
    printf 'mRoleScope bootstrap: download of %s via %s failed (attempt %d/%d).\n' \
      "$source_path" "$mirror" "$attempt" "$ASSET_DOWNLOAD_ATTEMPTS" >&2
  done

  return 1
}

download_asset() {
  local source_path="$1" destination="$2" fallback_mirror

  if download_asset_from_mirror "$ASSET_MIRROR" "$source_path" "$destination"; then
    return
  fi

  case "$ASSET_MIRROR" in
    github-raw)
      fallback_mirror='jsdelivr'
      ;;
    jsdelivr)
      fallback_mirror='github-raw'
      ;;
  esac

  printf 'mRoleScope bootstrap: retrying %s via fallback mirror %s.\n' \
    "$source_path" "$fallback_mirror" >&2
  download_asset_from_mirror "$fallback_mirror" "$source_path" "$destination" \
    || die "could not download ${source_path} from either reviewed asset mirror"
}

download_asset 'deploy/remote-development/compose.yaml' "$staging_dir/compose.yaml"
download_asset 'deploy/remote-development/mrolescope-deploy' "$staging_dir/mrolescope-deploy"
download_asset 'deploy/remote-development/mrolescope-sync' "$staging_dir/mrolescope-sync"
download_asset 'deploy/remote-development/mrolescope-analysis' "$staging_dir/mrolescope-analysis"
download_asset 'deploy/remote-development/mrolescope-backfill-versioned-content' "$staging_dir/mrolescope-backfill-versioned-content"
download_asset 'deploy/remote-development/mrolescope-dictionary' "$staging_dir/mrolescope-dictionary"
download_asset 'deploy/remote-development/mrolescope-release' "$staging_dir/mrolescope-release"
download_asset 'deploy/remote-development/mrolescope-ai-analysis' "$staging_dir/mrolescope-ai-analysis"
download_asset 'deploy/remote-development/mrolescope-operations' "$staging_dir/mrolescope-operations"
download_asset 'deploy/remote-development/mrolescope-sync.service' "$staging_dir/mrolescope-sync.service"
download_asset 'deploy/remote-development/mrolescope-sync.timer' "$staging_dir/mrolescope-sync.timer"
download_asset 'deploy/remote-development/mrolescope-analysis.service' "$staging_dir/mrolescope-analysis.service"
download_asset 'deploy/remote-development/mrolescope-candidate-analysis.service' "$staging_dir/mrolescope-candidate-analysis.service"
download_asset 'deploy/remote-development/mrolescope-candidate-analysis-v0.2.service' "$staging_dir/mrolescope-candidate-analysis-v0.2.service"
download_asset 'deploy/remote-development/mrolescope-backfill-versioned-content.service' "$staging_dir/mrolescope-backfill-versioned-content.service"
download_asset 'deploy/remote-development/mrolescope-ai-trial-prepare.service' "$staging_dir/mrolescope-ai-trial-prepare.service"
download_asset 'deploy/remote-development/mrolescope-ai-plus-trial.service' "$staging_dir/mrolescope-ai-plus-trial.service"
download_asset 'deploy/remote-development/mrolescope-ai-flash-trial.service' "$staging_dir/mrolescope-ai-flash-trial.service"
download_asset 'deploy/remote-development/mrolescope-ai-plus-expand.service' "$staging_dir/mrolescope-ai-plus-expand.service"
download_asset 'deploy/remote-development/mrolescope-ai-plus-publish.service' "$staging_dir/mrolescope-ai-plus-publish.service"
bash -n "$staging_dir/mrolescope-deploy"
bash -n "$staging_dir/mrolescope-sync"
bash -n "$staging_dir/mrolescope-analysis"
bash -n "$staging_dir/mrolescope-backfill-versioned-content"
bash -n "$staging_dir/mrolescope-dictionary"
bash -n "$staging_dir/mrolescope-release"
bash -n "$staging_dir/mrolescope-ai-analysis"
bash -n "$staging_dir/mrolescope-operations"
printf 'DOCKERHUB_NAMESPACE=%s\n' "$DOCKERHUB_NAMESPACE" >"$staging_dir/remote-development.conf"

if ! getent passwd "$DEPLOY_USER" >/dev/null; then
  useradd --system --user-group --home-dir /opt/mrolescope-runner --shell /usr/sbin/nologin "$DEPLOY_USER"
fi
getent group "$DEPLOY_USER" >/dev/null || die "the deployment user's private group is missing"
usermod --gid "$DEPLOY_USER" --groups "$DEPLOY_USER" --shell /usr/sbin/nologin "$DEPLOY_USER"
install -d -o "$DEPLOY_USER" -g "$DEPLOY_USER" -m 0750 /opt/mrolescope-runner

install -d -o root -g root -m 0755 /opt/mrolescope /etc/mrolescope /var/lib/mrolescope
install -d -o root -g root -m 0700 "$SECRETS_DIR" "$RELEASE_DIR" "$OUTPUT_AUDIT_DIR"
install -o root -g root -m 0644 "$staging_dir/compose.yaml" "$COMPOSE_DESTINATION"
install -o root -g root -m 0750 "$staging_dir/mrolescope-deploy" "$HELPER_DESTINATION"
install -o root -g root -m 0750 "$staging_dir/mrolescope-sync" "$SYNC_HELPER_DESTINATION"
install -o root -g root -m 0750 "$staging_dir/mrolescope-analysis" "$ANALYSIS_HELPER_DESTINATION"
install -o root -g root -m 0750 "$staging_dir/mrolescope-backfill-versioned-content" "$BACKFILL_HELPER_DESTINATION"
install -o root -g root -m 0750 "$staging_dir/mrolescope-dictionary" "$DICTIONARY_HELPER_DESTINATION"
install -o root -g root -m 0750 "$staging_dir/mrolescope-release" "$RELEASE_HELPER_DESTINATION"
install -o root -g root -m 0750 "$staging_dir/mrolescope-ai-analysis" "$AI_ANALYSIS_HELPER_DESTINATION"
install -o root -g root -m 0750 "$staging_dir/mrolescope-operations" "$OPERATIONS_HELPER_DESTINATION"
install -o root -g root -m 0644 "$staging_dir/mrolescope-sync.service" "$SYNC_SERVICE_DESTINATION"
install -o root -g root -m 0644 "$staging_dir/mrolescope-sync.timer" "$SYNC_TIMER_DESTINATION"
install -o root -g root -m 0644 "$staging_dir/mrolescope-analysis.service" "$ANALYSIS_SERVICE_DESTINATION"
install -o root -g root -m 0644 "$staging_dir/mrolescope-candidate-analysis.service" "$CANDIDATE_ANALYSIS_SERVICE_DESTINATION"
install -o root -g root -m 0644 "$staging_dir/mrolescope-candidate-analysis-v0.2.service" "$CANDIDATE_V02_ANALYSIS_SERVICE_DESTINATION"
install -o root -g root -m 0644 "$staging_dir/mrolescope-backfill-versioned-content.service" "$BACKFILL_SERVICE_DESTINATION"
install -o root -g root -m 0644 "$staging_dir/mrolescope-ai-trial-prepare.service" "$AI_TRIAL_PREPARE_SERVICE_DESTINATION"
install -o root -g root -m 0644 "$staging_dir/mrolescope-ai-plus-trial.service" "$AI_PLUS_TRIAL_SERVICE_DESTINATION"
install -o root -g root -m 0644 "$staging_dir/mrolescope-ai-flash-trial.service" "$AI_FLASH_TRIAL_SERVICE_DESTINATION"
install -o root -g root -m 0644 "$staging_dir/mrolescope-ai-plus-expand.service" "$AI_PLUS_EXPAND_SERVICE_DESTINATION"
install -o root -g root -m 0644 "$staging_dir/mrolescope-ai-plus-publish.service" "$AI_PLUS_PUBLISH_SERVICE_DESTINATION"
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
printf '%s\n' 'The daily sync timer is installed; bootstrap did not change its enabled or active state.'
printf '%s\n' 'Next: ensure the labeled GitHub Actions runner and root-only runtime env file are ready.'
