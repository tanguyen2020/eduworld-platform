#!/usr/bin/env bash
# Migration jobs before a backend rollout (F130-AC-02; DEPLOYMENT §12; ADR-0039): for each module database, in order,
# a one-off Swarm service `migrate --module=<module>` (replicated-job, restart none, network data) with ONLY the
# secret <module>_migrator_dsn; waits until its task is Complete. Any failure or timeout stops here (exit 1) and the
# caller (deploy-service.sh) never runs `docker stack deploy`. Expand/contract (AGENTS R12): no automatic rollback.
#   run-migrate-job.sh --env=<staging|production> --version=vX.Y.Z [--modules="identity learning …"] [--timeout=600]
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=scripts/deploy/lib-deploy.sh
. "$here/lib-deploy.sh"

ENV="" VERSION="${BACKEND_VERSION:-}" MODS="$MODULES" TIMEOUT="${EW_MIGRATE_TIMEOUT_SECONDS:-600}" POLL="${EW_MIGRATE_POLL_SECONDS:-5}"
for arg in "$@"; do
  case "$arg" in
    --env=*) ENV="${arg#*=}" ;;
    --version=*) VERSION="${arg#*=}" ;;
    --modules=*) MODS="${arg#*=}" ;;
    --timeout=*) TIMEOUT="${arg#*=}" ;;
    *) die "unknown argument $arg" ;;
  esac
done
check_env "$ENV"; check_version "$VERSION"
load_stack_env "$ENV"
image="ghcr.io/${GHCR_ORG:?GHCR_ORG required}/eduworld/migrate:$VERSION"

for m in $MODS; do
  [[ " $MODULES " == *" $m "* ]] || die "unknown module $m"
  secret=$(latest_secret "${m}_migrator_dsn")
  name="${STACK}_migrate-$m-$(date -u +%Y%m%d%H%M%S)"
  up=$(printf '%s' "$m" | tr '[:lower:]' '[:upper:]')
  audit_line migrate_started module="$m" tag="$VERSION" job="$name"
  $DOCKER service create --detach --with-registry-auth --mode replicated-job --restart-condition none \
    --name "$name" --label ew.migrate=true --label "ew.module=$m" --network data \
    --secret "source=$secret,target=${m}_migrator_dsn" \
    --env "EW_DB_${up}_MIGRATOR_URL_FILE=/run/secrets/${m}_migrator_dsn" \
    "$image" "--module=$m" >/dev/null || die "cannot create the migrate job of $m"
  deadline=$(( $(date +%s) + TIMEOUT ))
  state=""
  while :; do
    state=$($DOCKER service ps "$name" --format '{{.CurrentState}}' 2>/dev/null | head -1 | awk '{print $1}')
    case "$state" in
      Complete) break ;;
      Failed|Rejected|Shutdown|Orphaned)
        $DOCKER service logs --raw "$name" 2>&1 | tail -n 50 >&2 || true
        audit_line migrate_failed module="$m" tag="$VERSION" state="$state"
        die "migration of $m failed ($state): rollout stopped" ;;
    esac
    if [ "$(date +%s)" -ge "$deadline" ]; then
      audit_line migrate_failed module="$m" tag="$VERSION" state=timeout
      die "migration of $m did not complete within ${TIMEOUT}s: rollout stopped"
    fi
    sleep "$POLL"
  done
  audit_line migrate_completed module="$m" tag="$VERSION"
done
log info "all migrations complete: $MODS"
