#!/usr/bin/env bash
# RB-03 — loss of the whole PostgreSQL cluster / DR on the standby site (F130-AC-21; BACKUP_RECOVERY §3.5, RB-03).
# Orchestrated from the deploy runner (DOCKER_HOST = manager of the target site). Every database comes back to the
# SAME point T (latest archived WAL, or --target), then every consumer is reset to T, then the binaries restart in the
# order of RB-03 step 4: core-api → worker-events, worker-chat, worker-jobs → realtime.
#   rb03-cluster-restore.sh --env=<env> [--target="<timestamp>"] --actor=… --purpose=… [--dry-run] --yes
# Steps (each audited): 1 scale the app stack to 0; 2 one-off restore job on pg-1's node (pgbackrest restore, type
# default or time, into the pg-1 volume; Patroni bootstraps the replicas from it); 3 deploy the data stack and wait for
# a Patroni leader; 4 reset the consumers of the 5 modules to T (reset-consumers.sh in a nats-box job); 5 start the
# binaries in order, waiting for their replicas; 6 rotate the DB credentials when the cause is unknown (secrets-sync).
# RTO target: `rpo_rto` (≤ 4 h). Elapsed time is printed for the DR drill report.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=scripts/deploy/lib-deploy.sh
. "$here/../deploy/lib-deploy.sh"

ENV="" TARGET="" ACTOR="${EW_ACTOR:-}" PURPOSE="" DRY_RUN=false YES=false
for arg in "$@"; do
  case "$arg" in
    --env=*) ENV="${arg#*=}" ;;
    --target=*) TARGET="${arg#*=}" ;;
    --actor=*) ACTOR="${arg#*=}" ;;
    --purpose=*) PURPOSE="${arg#*=}" ;;
    --dry-run) DRY_RUN=true ;;
    --yes) YES=true ;;
    *) die "unknown argument $arg" ;;
  esac
done
check_env "$ENV"
[ -n "$ACTOR" ] && [ -n "$PURPOSE" ] || die "--actor and --purpose are required (F130-AC-15)"
[ "$DRY_RUN" = true ] || [ "$YES" = true ] || die "RB-03 stops every module: pass --yes (or --dry-run)"
export EW_ACTOR="$ACTOR"
run() { if [ "$DRY_RUN" = true ]; then echo "would run: $*"; else "$@"; fi; }
start=$(date +%s)
since_utc=$(date -u -d "${TARGET:-now}" +%Y-%m-%dT%H:%M:%SZ)
audit_line rb03_started target="${TARGET:-latest}" purpose="$PURPOSE" dry_run="$DRY_RUN"

for s in realtime worker-jobs worker-chat worker-events core-api; do run $DOCKER service scale --detach=false "${STACK}_$s=0"; done
restore_cmd=(pgbackrest --stanza=eduworld --delta --pg1-path=/var/lib/postgresql/data/pgdata restore)
[ -n "$TARGET" ] && restore_cmd+=(--type=time --target="$TARGET" --target-action=promote)
run "$here/run-ops-job.sh" --env="$ENV" --like=pgbackrest-1 --restore -- "${restore_cmd[@]}"
run "$REPO_ROOT/scripts/deploy/deploy-service.sh" --env="$ENV" --stack=data
for m in $MODULES; do
  run "$here/run-ops-job.sh" --env="$ENV" --like=backup-tools --network=app --secret=ops_nats_creds --     reset-consumers.sh --module="$m" --since="$since_utc" --yes
done
replicas_of() {
  node -e 'const y=require(process.argv[1]);const d=y.load(require("fs").readFileSync(process.argv[2],"utf8"));console.log(d.services[process.argv[3]]?.deploy?.replicas ?? 2)'     "$REPO_ROOT/scripts/node_modules/js-yaml" "$REPO_ROOT/deploy/swarm/$ENV/app.stack.yml" "$1"
}
for group in "core-api" "worker-events worker-chat worker-jobs" "realtime"; do
  for s in $group; do run $DOCKER service scale --detach=false "${STACK}_$s=$(replicas_of "$s")"; done
done
audit_line rb03_finished seconds=$(( $(date +%s) - start )) target="${TARGET:-latest}"
log info "RB-03 done in $(( $(date +%s) - start ))s; rotate DB credentials if the cause is unknown (secrets-sync.sh)"
