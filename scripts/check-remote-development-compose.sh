#!/usr/bin/env bash
# Guard the network boundary that lets the host reach only the API. Docker
# cannot publish a port for a container attached exclusively to an internal
# bridge network, so the API also needs the ordinary `edge` bridge.

set -euo pipefail

readonly compose_file="${1:-deploy/remote-development/compose.yaml}"

die() {
  printf 'remote-development Compose check failed: %s\n' "$1" >&2
  exit 1
}

[[ -f "$compose_file" ]] || die "Compose file does not exist: $compose_file"

grep -Fqx '    networks: [backend, edge]' "$compose_file" \
  || die 'API must be attached to backend and edge networks'

[[ "$(grep -Fxc '    networks: [backend]' "$compose_file")" -eq 2 ]] \
  || die 'only PostgreSQL and migration services may use backend alone'

grep -Fqx '    profiles: [sync]' "$compose_file" \
  || die 'worker must remain an explicitly requested one-shot profile'

[[ "$(grep -Fxc '    networks: [backend, edge]' "$compose_file")" -eq 2 ]] \
  || die 'only API and no-port worker may attach to the ordinary edge network'

[[ "$(grep -Fxc '    ports:' "$compose_file")" -eq 1 ]] \
  || die 'only API may publish a host port'

grep -Fqx '      - "127.0.0.1:8080:8080"' "$compose_file" \
  || die 'API must publish port 8080 only on the host loopback address'

grep -Fqx '    internal: true' "$compose_file" \
  || die 'backend network must remain internal'

grep -Fqx '  edge: {}' "$compose_file" \
  || die 'edge network must be an ordinary bridge network'
