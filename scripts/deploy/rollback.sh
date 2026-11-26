#!/usr/bin/env bash
# Rolls ONE backend Swarm service back to its previous spec (F130-AC-03; DEPLOYMENT §11.3): other services are not
# touched; target ≤ 15 minutes (fails when exceeded). Migrations are never rolled back (expand/contract, R12).
#   rollback.sh --env=<staging|production> --service=<core-api|realtime|worker-events|worker-chat|worker-jobs> [--reason=…]
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=scripts/deploy/lib-deploy.sh
. "$here/lib-deploy.sh"
ENV="" SERVICE="" REASON="manual"
for arg in "$@"; do
  case "$arg" in
    --env=*) ENV="${arg#*=}" ;;
    --service=*) SERVICE="${arg#*=}" ;;
    --reason=*) REASON="${arg#*=}" ;;
    *) die "unknown argument $arg" ;;
  esac
done
check_env "$ENV"; check_service "$SERVICE"
require_quorum
LIMIT="${EW_ROLLBACK_LIMIT_SECONDS:-900}"
start=$(date +%s)
audit_line rollback_started service="$SERVICE" reason="$REASON"
if ! timeout "$LIMIT" $DOCKER service rollback --detach=false "${STACK}_$SERVICE" >/dev/null; then
  audit_line rollback_failed service="$SERVICE" seconds=$(( $(date +%s) - start ))
  die "rollback of $SERVICE failed or exceeded ${LIMIT}s"
fi
seconds=$(( $(date +%s) - start ))
audit_line rollback_completed service="$SERVICE" seconds="$seconds"
log info "rollback of $SERVICE completed in ${seconds}s"
