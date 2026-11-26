#!/usr/bin/env bash
# Deploys one stack of an environment, or one backend Swarm service (F130-AC-02, AC-03, AC-05; DEPLOYMENT §10–§12).
#   deploy-service.sh --env=<staging|production> --stack=app [--version=vX.Y.Z] [--service=<swarm-service>]
#                     [--expect-digests=FILE] [--record-digests=FILE] [--hotfix] [--migrate-only|--skip-migrate] [--redeploy]
#   deploy-service.sh --env=<env> --stack=infra|data|ops|obs
# Order for the app stack: manager quorum → (production) same digests as staging + maintenance window → migrate
# job of every module database (run-migrate-job.sh; any failure stops here) → `docker stack deploy` (only services
# whose spec changed are updated; rolling start-first, failure_action rollback). --service updates the image of
# that one service only (TC-SVC-015: no other service restarts). Re-running with the same tag changes nothing.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=scripts/deploy/lib-deploy.sh
. "$here/lib-deploy.sh"

ENV="" STACK_NAME="" VERSION="${BACKEND_VERSION:-}" SERVICE="" EXPECT="" RECORD="" HOTFIX=false MIGRATE=run REDEPLOY=false
for arg in "$@"; do
  case "$arg" in
    --env=*) ENV="${arg#*=}" ;;
    --stack=*) STACK_NAME="${arg#*=}" ;;
    --version=*) VERSION="${arg#*=}" ;;
    --service=*) SERVICE="${arg#*=}" ;;
    --expect-digests=*) EXPECT="${arg#*=}" ;;
    --record-digests=*) RECORD="${arg#*=}" ;;
    --hotfix) HOTFIX=true ;;
    --migrate-only) MIGRATE=only ;;   # production: migrations before the canary (canary.sh), then --skip-migrate
    --skip-migrate) MIGRATE=skip ;;
    --redeploy) REDEPLOY=true ;;       # same version again (secret rotation): no digest file, version must be the running one
    *) die "unknown argument $arg" ;;
  esac
done
check_env "$ENV"
[[ "$STACK_NAME" =~ ^(app|infra|data|ops|obs)$ ]] || die "--stack=app|infra|data|ops|obs required"
file="$REPO_ROOT/deploy/swarm/$ENV/$STACK_NAME.stack.yml"
[ -f "$file" ] || die "missing $file"
require_quorum
load_stack_env "$ENV"

if [ "$STACK_NAME" = app ]; then
  check_version "$VERSION"
  export BACKEND_VERSION="$VERSION"
  if [ "$ENV" = production ]; then
    # F130-AC-05: only what staging ran, inside the maintenance window unless hotfix (approval: GitHub Environment).
    if [ "$REDEPLOY" = true ]; then
      running=$($DOCKER service inspect "${STACK}_core-api" --format "{{.Spec.TaskTemplate.ContainerSpec.Image}}" | sed -E 's#@.*$##; s#^.*:##')
      [ "$running" = "$VERSION" ] || die "--redeploy only redeploys the running version ($running), not $VERSION"
      [ "$MIGRATE" = skip ] || die "--redeploy requires --skip-migrate"
    fi
    if [ "$REDEPLOY" = false ]; then
      { [ -n "$EXPECT" ] && [ -s "$EXPECT" ]; } || die "production needs --expect-digests=<digests deployed on staging>"
    fi
    if [ "$REDEPLOY" = false ]; then for img in core-api realtime worker migrate; do
      want=$(awk -v i="$img" '$1 == i {print $2}' "$EXPECT")
      got=$(image_digest "ghcr.io/$GHCR_ORG/eduworld/$img:$VERSION")
      [ -n "$want" ] && [ "$want" = "$got" ] || { audit_line deploy_refused tag="$VERSION" reason="digest of $img differs from staging"; die "digest of $img:$VERSION ($got) is not the one deployed on staging (${want:-none})"; }
    done; fi
    if [ "$HOTFIX" = false ] && ! in_maintenance_window; then
      audit_line deploy_refused tag="$VERSION" reason="outside maintenance_window"
      die "production deploys run 22:00–06:00 ICT or at the weekend (maintenance_window); use --hotfix for a hotfix"
    fi
  fi
  if [ "$MIGRATE" != skip ]; then "$here/run-migrate-job.sh" --env="$ENV" --version="$VERSION"; fi
  if [ "$MIGRATE" = only ]; then audit_line migrate_only_completed tag="$VERSION"; exit 0; fi
fi

files=("$file")
export_secret_versions "${files[@]}"
export_config_versions "${files[@]}"
if [ -n "$SERVICE" ]; then
  [ "$STACK_NAME" = app ] || die "--service only applies to the app stack"
  check_service "$SERVICE"
  image=$(case "$SERVICE" in core-api) echo core-api ;; realtime) echo realtime ;; *) echo worker ;; esac)
  audit_line deploy_started service="$SERVICE" tag="$VERSION" reason="${EW_DEPLOY_REASON:-release}"
  $DOCKER service update --detach=false --with-registry-auth --image "ghcr.io/$GHCR_ORG/eduworld/$image:$VERSION" "${STACK}_$SERVICE" >/dev/null \
    || { audit_line deploy_failed service="$SERVICE" tag="$VERSION"; die "update of $SERVICE failed (Swarm rolled back if the new tasks were unhealthy)"; }
else
  audit_line deploy_started stack="$STACK_NAME" tag="${VERSION:-n/a}" reason="${EW_DEPLOY_REASON:-release}"
  (cd "$(dirname "$file")" && $DOCKER stack deploy --detach=false --with-registry-auth -c "$(basename "$file")" "$STACK") \
    || { audit_line deploy_failed stack="$STACK_NAME" tag="${VERSION:-n/a}"; die "stack deploy of $STACK_NAME failed"; }
fi
audit_line deploy_completed stack="$STACK_NAME" service="${SERVICE:-all}" tag="${VERSION:-n/a}"

if [ -n "$RECORD" ] && [ "$STACK_NAME" = app ]; then
  : > "$RECORD"
  for img in core-api realtime worker migrate; do
    printf '%s %s\n' "$img" "$(image_digest "ghcr.io/$GHCR_ORG/eduworld/$img:$VERSION")" >> "$RECORD"
  done
fi
