#!/usr/bin/env bash
# pgBackRest backup of the single PostgreSQL cluster (F130-AC-12, AC-13; BACKUP_RECOVERY §3.2, §3.4, §10).
#   pg_backup.sh full|diff|incr [--annotation=semester]
# Weekly full (Sunday 01:00 ICT) and daily differential (other days) per ops-jobs.crontab; WAL is archived
# continuously by PostgreSQL (archive_command). After the backup, `pgbackrest check` verifies archiving (daily).
# Metrics (textfile): eduworld_backup_{last_run_timestamp,last_status,duration_seconds,last_success_timestamp}
# {target="postgres",type=<full|diff|incr|check>}. A backup during a Patroni switchover either runs from the new
# topology or fails loudly (status 0); pgBackRest never leaves a silently broken backup set.
set -euo pipefail
# shellcheck source=scripts/backup/lib-backup.sh
. "$(dirname "$0")/lib-backup.sh"

TYPE="${1:-diff}"
case "$TYPE" in full|diff|incr) ;; *) die "usage: pg_backup.sh full|diff|incr [--annotation=<label>]" ;; esac
ANNOTATION=""
for arg in "${@:2}"; do
  case "$arg" in
    --annotation=*) ANNOTATION="${arg#*=}"; [[ "$ANNOTATION" =~ ^[a-z0-9-]{1,32}$ ]] || die "annotation must match ^[a-z0-9-]{1,32}$" ;;
    *) die "unknown argument $arg" ;;
  esac
done
STANZA="${PGBACKREST_STANZA:-eduworld}"
PGBACKREST="${PGBACKREST_BIN:-pgbackrest}"
load_repo_write_credentials

backup_args=(--stanza="$STANZA" --type="$TYPE")
[ -n "$ANNOTATION" ] && backup_args+=(--annotation="ew-label=$ANNOTATION")

start=$(date +%s)
log_json info "backup started" target=postgres type="$TYPE" stanza="$STANZA"
status=1
if ! "$PGBACKREST" "${backup_args[@]}" backup; then
  status=0
  log_json error "backup failed" target=postgres type="$TYPE"
fi
end=$(date +%s)
metrics_write postgres "$TYPE" "$status" "$start" "$end"

cstart=$(date +%s)
cstatus=1
if ! "$PGBACKREST" --stanza="$STANZA" check; then
  cstatus=0
  log_json error "pgbackrest check failed: WAL archiving or repo unreachable" target=postgres type=check
fi
metrics_write postgres check "$cstatus" "$cstart" "$(date +%s)"

[ "$status" = 1 ] && [ "$cstatus" = 1 ] || exit 1
log_json info "backup done" target=postgres type="$TYPE" seconds=$((end - start))
