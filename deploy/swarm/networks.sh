#!/usr/bin/env bash
# Creates the overlay networks of the stacks (F130 T002, F130-AC-07; DEPLOYMENT §4.3), idempotent: an existing network
# is checked (driver overlay, encryption as required) and left untouched. `app`, `data`, `admin` are encrypted (IPsec
# between nodes); `edge` carries only gateway → core-api/realtime/Keycloak traffic. All are attachable=false: only
# stack services and one-off jobs (service create --network) join them.
#   deploy/swarm/networks.sh [--dry-run]     (DOCKER_HOST = a Swarm manager)
set -euo pipefail
DOCKER="${DOCKER_BIN:-docker}"
DRY_RUN=false
[ "${1:-}" = "--dry-run" ] && DRY_RUN=true
rc=0
for spec in edge:plain app:encrypted data:encrypted admin:encrypted; do
  name=${spec%%:*} mode=${spec#*:}
  if info=$($DOCKER network inspect "$name" --format '{{.Driver}} {{index .Options "encrypted"}}' 2>/dev/null); then
    driver=${info%% *} enc=${info#* }
    if [ "$driver" != overlay ]; then echo "network $name exists with driver $driver (want overlay)" >&2; rc=1; continue; fi
    if [ "$mode" = encrypted ] && { [ "$enc" = "<no value>" ] || [ -z "$enc" ]; }; then
      echo "network $name exists WITHOUT encryption: recreate it in a maintenance window (stacks must be removed from it first)" >&2; rc=1; continue
    fi
    echo "network $name ok"
    continue
  fi
  args=(network create --driver overlay --scope swarm)
  [ "$mode" = encrypted ] && args+=(--opt encrypted)
  args+=("$name")
  if [ "$DRY_RUN" = true ]; then echo "would run: docker ${args[*]}"; else $DOCKER "${args[@]}" >/dev/null && echo "network $name created ($mode)"; fi
done
exit "$rc"
