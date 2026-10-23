#!/usr/bin/env bash
# Exercise the bootstrap download boundary with controlled curl failures. The
# installation commands are mocked, so this neither contacts a network source
# nor changes host configuration.

set -euo pipefail

readonly bootstrap="${1:-deploy/remote-development/bootstrap.sh}"

die() {
  printf 'bootstrap asset mirror fallback check failed: %s\n' "$1" >&2
  exit 1
}

[[ -f "$bootstrap" ]] || die "bootstrap script does not exist: $bootstrap"

test_dir=$(mktemp -d)
cleanup() {
  rm -rf -- "$test_dir"
}
trap cleanup EXIT

readonly mock_bin="$test_dir/bin"
readonly event_log="$test_dir/events.log"
readonly curl_attempts="$test_dir/curl-attempts.log"
readonly bootstrap_log="$test_dir/bootstrap.log"
mkdir -p "$mock_bin"

cat >"$mock_bin/curl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail

output=''
url=''
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --output)
      output="$2"
      shift 2
      ;;
    https://*)
      url="$1"
      shift
      ;;
    *)
      shift
      ;;
  esac
done

[[ -n "$output" && -n "$url" ]] || exit 64
printf 'curl %s\n' "$url" >>"$MROLESCOPE_TEST_EVENT_LOG"

if [[ "$url" == 'https://cdn.jsdelivr.net/gh/lf19910822/mRoleScope@'*'/deploy/remote-development/mrolescope-analysis' ]]; then
  printf '%s\n' "$url" >>"$MROLESCOPE_TEST_CURL_ATTEMPTS"
  printf 'partial download\n' >"$output"
  printf 'curl: (35) OpenSSL SSL_connect: Connection reset by peer\n' >&2
  exit 35
fi

printf 'fixture asset\n' >"$output"
MOCK

cat >"$mock_bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
[[ "$#" -eq 2 && "$1" == compose && "$2" == version ]]
MOCK

cat >"$mock_bin/getent" <<'MOCK'
#!/usr/bin/env bash
exit 0
MOCK

cat >"$mock_bin/usermod" <<'MOCK'
#!/usr/bin/env bash
exit 0
MOCK

cat >"$mock_bin/visudo" <<'MOCK'
#!/usr/bin/env bash
exit 0
MOCK

cat >"$mock_bin/systemctl" <<'MOCK'
#!/usr/bin/env bash
exit 0
MOCK

cat >"$mock_bin/install" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail

printf 'install\n' >>"$MROLESCOPE_TEST_EVENT_LOG"
for argument in "$@"; do
  case "$argument" in
    /tmp/mrolescope-bootstrap.*/remote-development.conf|/tmp/mrolescope-bootstrap.*/mrolescope-deploy.sudoers)
      ;;
    /tmp/mrolescope-bootstrap.*/*)
      [[ "$(<"$argument")" == 'fixture asset' ]] || exit 1
      ;;
  esac
done
MOCK

chmod 0755 "$mock_bin"/*

run_bootstrap() {
  local -a command=(
    env
    "PATH=$mock_bin:$PATH"
    "MROLESCOPE_ASSET_MIRROR=jsdelivr"
    "MROLESCOPE_TEST_EVENT_LOG=$event_log"
    "MROLESCOPE_TEST_CURL_ATTEMPTS=$curl_attempts"
    bash "$bootstrap"
    0123456789abcdef0123456789abcdef01234567
    mrolescopeverify
  )

  if [[ "$EUID" -eq 0 ]]; then
    "${command[@]}"
    return
  fi

  sudo -n "${command[@]}"
}

if [[ "$EUID" -ne 0 ]] && ! sudo -n true 2>/dev/null; then
  printf '%s\n' 'bootstrap asset mirror fallback check skipped: root or passwordless sudo is unavailable'
  exit 0
fi

if ! run_bootstrap >"$bootstrap_log" 2>&1; then
  sed -n '1,160p' "$bootstrap_log" >&2
  die 'bootstrap did not recover after the preferred mirror failed'
fi

readonly jsdelivr_analysis_url='https://cdn.jsdelivr.net/gh/lf19910822/mRoleScope@0123456789abcdef0123456789abcdef01234567/deploy/remote-development/mrolescope-analysis'
readonly github_analysis_url='https://raw.githubusercontent.com/lf19910822/mRoleScope/0123456789abcdef0123456789abcdef01234567/deploy/remote-development/mrolescope-analysis'

[[ "$(grep -Fxc "$jsdelivr_analysis_url" "$curl_attempts")" -eq 2 ]] \
  || die 'the preferred mirror was not retried exactly twice for the failed asset'
[[ "$(grep -Fxc "curl $github_analysis_url" "$event_log")" -eq 1 ]] \
  || die 'the alternate mirror was not used after preferred-mirror retries failed'

last_download_line=$(grep -n '^curl ' "$event_log" | tail -n 1 | cut -d: -f1)
first_install_line=$(grep -n '^install$' "$event_log" | head -n 1 | cut -d: -f1)
[[ -n "$last_download_line" && -n "$first_install_line" && "$first_install_line" -gt "$last_download_line" ]] \
  || die 'installation started before all staged downloads succeeded'

printf '%s\n' 'bootstrap asset mirror fallback check passed'
