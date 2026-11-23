#!/usr/bin/env bash
set -euo pipefail

readonly helper="${1:-deploy/remote-development/mrolescope-ai-evaluation}"
readonly compose_file="${2:-deploy/remote-development/compose.yaml}"
readonly bootstrap="${3:-deploy/remote-development/bootstrap.sh}"
readonly worker_dockerfile="${4:-docker/Dockerfile.worker}"

die() {
  printf 'remote-development AI evaluation check failed: %s\n' "$1" >&2
  exit 1
}

[[ -f "$helper" ]] || die 'evaluation helper is missing'
[[ -f "$compose_file" ]] || die 'Compose file is missing'
[[ -f "$bootstrap" ]] || die 'bootstrap is missing'
[[ -f "$worker_dockerfile" ]] || die 'worker Dockerfile is missing'
grep -Fqx "[[ \"\$#\" -eq 1 && ( \"\$1\" == 'run-trend' || \"\$1\" == 'run-agent' ) ]] || die 'operation is not allowlisted'" "$helper" \
  || die 'helper must allowlist exactly run-trend and run-agent'
grep -Fq -- "--profile ai-trend-evaluation run --rm --no-deps ai-trend-evaluation" "$helper" \
  || die 'trend helper operation is not fixed'
grep -Fq -- "--profile ai-trend-evaluation run --rm --no-deps ai-agent-evaluation" "$helper" \
  || die 'Agent helper operation is not fixed'
if grep -Fq 'exec compose' "$helper"; then
  die 'AI evaluation helper must call the compose shell function without exec'
fi
for mapping in \
  '  export MROLESCOPE_API_IMAGE="$api_image"' \
  '  export MROLESCOPE_MIGRATE_IMAGE="$migrate_image"' \
  '  export MROLESCOPE_WORKER_IMAGE="$worker_image"' \
  '  export MROLESCOPE_WEB_EDGE_IMAGE="$web_edge_image"'; do
  grep -Fqx "$mapping" "$helper" || die "AI evaluation helper must export verified image mapping: ${mapping}"
done
if grep -Eq 'printf.*MROLESCOPE_QWEN_API_KEY|echo.*MROLESCOPE_QWEN_API_KEY|cat[[:space:]].*SECRETS' "$helper"; then
  die 'helper may print secret values'
fi
grep -Fqx "readonly AI_EVALUATION_HELPER_DESTINATION='/usr/local/sbin/mrolescope-ai-evaluation'" "$bootstrap" \
  || die 'bootstrap does not install the evaluation helper'
grep -Fqx "  download_asset 'deploy/remote-development/mrolescope-ai-evaluation' \"\$staging_dir/mrolescope-ai-evaluation\"" "$bootstrap" \
  || die 'bootstrap does not download the evaluation helper'
grep -Fq "readonly EVALUATION_DIR='/var/lib/mrolescope/ai-evaluation'" "$bootstrap" \
  || die 'bootstrap does not create the evaluation artifact directory'
grep -Fq "go build -trimpath -ldflags='-s -w' -o /out/mrolescope-ai-evaluation ./cmd/ai-evaluation" "$worker_dockerfile" \
  || die 'worker image does not build the evaluation binary'
grep -Fq 'COPY --from=build /out/mrolescope-ai-evaluation /usr/local/bin/mrolescope-ai-evaluation' "$worker_dockerfile" \
  || die 'worker image does not ship the evaluation binary'

for service in ai-trend-evaluation ai-agent-evaluation; do
  block="$(sed -n "/^  ${service}:/,/^  [a-z0-9-]*:/p" "$compose_file" | sed '$d')"
  [[ -n "$block" ]] || die "${service} is missing"
  grep -Fqx '    profiles: [ai-trend-evaluation]' <<<"$block" || die "${service} profile is not isolated"
  grep -Fqx '    networks: [ai-edge]' <<<"$block" || die "${service} must use only ai-edge"
  grep -Fqx '    read_only: true' <<<"$block" || die "${service} must be read-only"
  grep -Fqx '        source: /var/lib/mrolescope/ai-evaluation' <<<"$block" || die "${service} report path is not fixed"
  if grep -Eq '^[[:space:]]+ports:|DATABASE_URL|backend|raw|COS_' <<<"$block"; then
    die "${service} has an unsafe production boundary"
  fi
done
