#!/usr/bin/env bash
# Runs one runbook command as a one-off Swarm job (replicated-job, restart none) that copies the image, networks,
# environment and secrets of an existing service (F130 US4: RB-01/03/04/05, restore tests). Runs on the deploy runner
# (DOCKER_HOST = manager over TLS). Extra secrets that are NEVER in a stack (restore credential, admin .pgpass) are
# attached only to this job (F130-AC-15), at the paths the scripts read.
#   run-ops-job.sh --env=<staging|production> --like=<swarm-service> [--restore] [--admin] [--image=…] [--timeout=7200] -- <command…>
#     --restore  attach backup_restore_access_key/secret_key (latest version) under /run/restore-credentials
#     --admin    attach ops_pg_admin_pgpass (latest version)
#     --network=<n> / --secret=<name>  extra overlay network / latest version of an extra secret (e.g. ops_nats_creds)
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=scripts/deploy/lib-deploy.sh
. "$here/../deploy/lib-deploy.sh"

ENV="" LIKE="" RESTORE=false ADMIN=false IMAGE="" TIMEOUT=7200 EXTRA_NETS=() EXTRA_SECRETS=()
while [ $# -gt 0 ]; do
  case "$1" in
    --env=*) ENV="${1#*=}" ;;
    --like=*) LIKE="${1#*=}" ;;
    --restore) RESTORE=true ;;
    --admin) ADMIN=true ;;
    --image=*) IMAGE="${1#*=}" ;;
    --timeout=*) TIMEOUT="${1#*=}" ;;
    --network=*) EXTRA_NETS+=("${1#*=}") ;;
    --secret=*) EXTRA_SECRETS+=("${1#*=}") ;;
    --) shift; break ;;
    *) die "unknown argument $1" ;;
  esac
  shift
done
check_env "$ENV"
[[ "$LIKE" =~ ^[a-z][a-z0-9-]{0,40}$ ]] || die "--like=<swarm service> required"
[ $# -gt 0 ] || die "command required after --"

spec=$($DOCKER service inspect "${STACK}_$LIKE" --format '{{json .Spec}}') || die "service ${STACK}_$LIKE not found"
[ -n "$IMAGE" ] || IMAGE=$(jq -r '.TaskTemplate.ContainerSpec.Image' <<< "$spec")
args=(service create --detach --mode replicated-job --restart-condition none --name "${STACK}_ops-$(date -u +%Y%m%d%H%M%S)"
  --label ew.ops-job=true --label "ew.actor=${EW_ACTOR:-${GITHUB_ACTOR:-unknown}}")
while IFS= read -r n; do [ -n "$n" ] && args+=(--network "$n"); done < <(jq -r '.TaskTemplate.Networks[]?.Target' <<< "$spec")
while IFS= read -r e; do [ -n "$e" ] && args+=(--env "$e"); done < <(jq -r '.TaskTemplate.ContainerSpec.Env[]?' <<< "$spec")
while IFS=$'\t' read -r name target; do
  [ -n "$name" ] && args+=(--secret "source=$name,target=$target")
done < <(jq -r '.TaskTemplate.ContainerSpec.Secrets[]? | [.SecretName, .File.Name] | @tsv' <<< "$spec")
while IFS= read -r c; do [ -n "$c" ] && args+=(--constraint "$c"); done < <(jq -r '.TaskTemplate.Placement.Constraints[]?' <<< "$spec")
if [ "$RESTORE" = true ]; then
  args+=(--secret "source=$(latest_secret backup_restore_access_key),target=/run/restore-credentials/backup_restore_access_key")
  args+=(--secret "source=$(latest_secret backup_restore_secret_key),target=/run/restore-credentials/backup_restore_secret_key")
fi
[ "$ADMIN" = true ] && args+=(--secret "source=$(latest_secret ops_pg_admin_pgpass),target=ops_pg_admin_pgpass")
for n in "${EXTRA_NETS[@]}"; do args+=(--network "$n"); done
for s in "${EXTRA_SECRETS[@]}"; do [[ "$s" =~ ^[a-z][a-z0-9_]{1,62}$ ]] || die "invalid secret name"; args+=(--secret "source=$(latest_secret "$s"),target=$s"); done
args+=("$IMAGE" "$@")

id=$($DOCKER "${args[@]}")
audit_line ops_job_started job="$id" like="$LIKE" restore="$RESTORE" admin="$ADMIN" command="$1"
deadline=$(( $(date +%s) + TIMEOUT ))
state=""
while [ "$(date +%s)" -lt "$deadline" ]; do
  state=$($DOCKER service ps "$id" --format '{{.CurrentState}}' | head -1 | awk '{print $1}')
  case "$state" in Complete|Failed|Rejected|Shutdown) break ;; esac
  sleep 5
done
$DOCKER service logs --raw "$id" 2>&1 | tail -n 200 || true
audit_line ops_job_finished job="$id" state="${state:-timeout}"
[ "$state" = Complete ] || die "ops job $id ended in state ${state:-timeout}"
