#!/usr/bin/env bash
# Restore test (F130-AC-16, AC-17 isolated part; BACKUP_RECOVERY §8): restores the cluster (full) or one module
# database (module, pgBackRest --db-include) to a point in time T into an ISOLATED scratch cluster (local socket only,
# no TCP listener, archive_mode off), reconciles, writes the report and removes the scratch copy.
#
#   restore_test.sh --mode=full|module [--db=eduworld_learning] [--target=<YYYY-MM-DD HH:MM:SS+07>|random]
#                   --actor=<who> --purpose=<why> [--pipeline=<url>] [--report-dir=DIR] [--dry-run]
#
# - T random (default): uniform in [max(now − 7 days, end of the oldest backup), now − 10 minutes].
# - Reads the repo with the RESTORE credential only (files backup_restore_access_key / backup_restore_secret_key in
#   EW_RESTORE_CREDENTIALS_DIR), issued for the run by scripts/backup/run-restore-test.sh (F130-AC-15); the scheduled
#   write credential is never used to read backups.
# - Reconciliation: submitted attempts of eduworld_learning (assessment.attempts, submitted_at ≤ T − grace) in the
#   restored copy == the same count on the live replica (role backup_reader); every restored database answers and its
#   Atlas revision is readable. Override per database: EW_RESTORE_RECONCILE_SQL_<DB>.
# - Pass criteria: duration ≤ EW_RESTORE_MAX_SECONDS (BACKUP_RECOVERY §8: full ≤ 2 h), 0 unexplained difference.
# - Report: <report-dir>/<YYYY-MM>.md (appended, fields of data-model §1) + one audit JSON line on stdout.
set -euo pipefail
# shellcheck source=scripts/backup/lib-backup.sh
. "$(dirname "$0")/lib-backup.sh"

MODE="" DB="" TARGET="random" ACTOR="${EW_ACTOR:-}" PURPOSE="" PIPELINE="${EW_PIPELINE_URL:-}" DRY_RUN=false
REPORT_DIR="${EW_RESTORE_REPORT_DIR:-/restore/reports}"
for arg in "$@"; do
  case "$arg" in
    --mode=*) MODE="${arg#*=}" ;;
    --db=*) DB="${arg#*=}" ;;
    --target=*) TARGET="${arg#*=}" ;;
    --actor=*) ACTOR="${arg#*=}" ;;
    --purpose=*) PURPOSE="${arg#*=}" ;;
    --pipeline=*) PIPELINE="${arg#*=}" ;;
    --report-dir=*) REPORT_DIR="${arg#*=}" ;;
    --dry-run) DRY_RUN=true ;;
    *) die "unknown argument $arg" ;;
  esac
done
case "$MODE" in
  full) ;;
  module) [[ "$DB" =~ ^eduworld_(identity|learning|game|competition|chat)$ ]] || die "--mode=module needs --db=eduworld_<module>" ;;
  *) die "usage: restore_test.sh --mode=full|module [--db=…] [--target=…|random] --actor=… --purpose=…" ;;
esac
[ -n "$ACTOR" ] || die "--actor is required (who runs the restore, F130-AC-15)"
[ -n "$PURPOSE" ] || die "--purpose is required (why, F130-AC-15)"
[ ${#ACTOR} -le 64 ] && [ ${#PURPOSE} -le 200 ] || die "--actor ≤ 64 and --purpose ≤ 200 characters"
export EW_ACTOR="$ACTOR"

STANZA="${PGBACKREST_STANZA:-eduworld}"
PGBACKREST="${PGBACKREST_BIN:-pgbackrest}"
WORK_ROOT="${EW_RESTORE_WORK_DIR:-/restore}"
PORT="${EW_RESTORE_PORT:-55432}"
GRACE="${EW_RESTORE_RECONCILE_GRACE_SECONDS:-60}"
MAX_SECONDS="${EW_RESTORE_MAX_SECONDS:-7200}"
START_TIMEOUT="${EW_RESTORE_START_TIMEOUT_SECONDS:-3600}"
LIVE_HOST="${EW_BACKUP_PG_HOST:-pg-router}"
LIVE_PORT="${EW_BACKUP_PG_PORT:-5433}"
LIVE_USER="${EW_BACKUP_PG_USER:-backup_reader}"
ALL_DBS="eduworld_identity eduworld_learning eduworld_game eduworld_competition eduworld_chat keycloak"
DEFAULT_LEARNING_SQL="SELECT count(*) FROM assessment.attempts WHERE submitted_at IS NOT NULL AND submitted_at <= (:'t')::timestamptz - make_interval(secs => :grace)"

# pick_random_target OLDEST_EPOCH NOW_EPOCH → "YYYY-MM-DD HH:MM:SS+00"
pick_random_target() {
  local oldest=$1 now=$2 lo hi
  lo=$(( now - 7 * 86400 )); [ "$oldest" -gt "$lo" ] && lo=$oldest
  hi=$(( now - 600 ))
  [ "$hi" -gt "$lo" ] || die "no backup old enough for a point-in-time target"
  local span=$(( hi - lo )) r=$(( (RANDOM << 15 | RANDOM) ))
  date -u -d "@$(( lo + r % span ))" '+%Y-%m-%d %H:%M:%S+00'
}

started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
start=$(date +%s)
work="$WORK_ROOT/run-$(date -u +%Y%m%dT%H%M%S)-$$"
pgdata="$work/pgdata"
sock="$work/sock"
cleanup() {
  if [ -f "$pgdata/postmaster.pid" ]; then pg_ctl -D "$pgdata" -m immediate -w stop >/dev/null 2>&1 || true; fi
  rm -rf "$work"   # scratch restored copy only (never a live database)
}
trap cleanup EXIT

verdict="PASS" issues=() recon_lines=()
target_desc="$TARGET"
if [ "$DRY_RUN" = true ]; then
  [ "$TARGET" = random ] && TARGET=$(pick_random_target $(( start - 8 * 86400 )) "$start")
  target_desc="$TARGET"
  recon_lines+=("dry-run: no restore executed")
  verdict="DRY-RUN"
else
  creds="${EW_RESTORE_CREDENTIALS_DIR:-/run/restore-credentials}"
  load_secret PGBACKREST_REPO1_S3_KEY "$creds/backup_restore_access_key"
  load_secret PGBACKREST_REPO1_S3_KEY_SECRET "$creds/backup_restore_secret_key"
  load_secret PGBACKREST_REPO1_CIPHER_PASS "${EW_SECRETS_DIR:-/run/secrets}/backup_repo_cipher_pass"

  if [ "$TARGET" = random ]; then
    oldest=$("$PGBACKREST" --stanza="$STANZA" --output=json info | jq '[.[0].backup[].timestamp.stop] | min // empty')
    [ -n "$oldest" ] || die "the repo has no backup"
    TARGET=$(pick_random_target "$oldest" "$start")
  fi
  target_desc="$TARGET"
  date -d "$TARGET" >/dev/null 2>&1 || die "invalid --target"
  mkdir -p "$pgdata" "$sock"; chmod 700 "$pgdata"
  audit restore_started mode="$MODE" db="${DB:-all}" target="$TARGET" purpose="$PURPOSE"

  restore_args=(--stanza="$STANZA" --reset-pg1-host --pg1-path="$pgdata" --type=time --target="$TARGET"
    --target-action=promote --archive-mode=off --log-level-console=warn)
  [ "$MODE" = module ] && restore_args+=(--db-include="$DB")
  log_json info "restore started" mode="$MODE" db="${DB:-all}" target="$TARGET"
  if ! "$PGBACKREST" "${restore_args[@]}" restore; then
    verdict="FAIL"; issues+=("pgbackrest restore failed")
  else
    # Isolation and a small footprint: no TCP listener, no archiving, own socket, hot_standby off during recovery.
    cat >> "$pgdata/postgresql.auto.conf" <<CONF
listen_addresses = ''
port = $PORT
unix_socket_directories = '$sock'
archive_mode = off
hot_standby = off
shared_buffers = '256MB'
huge_pages = off
shared_preload_libraries = ''
logging_collector = off
hba_file = '$pgdata/pg_hba.conf'
ident_file = '$pgdata/pg_ident.conf'
ssl = off
restore_command = '${PGBACKREST} --stanza=$STANZA --reset-pg1-host --pg1-path=$pgdata archive-get %f "%p"'
CONF
    # Local socket in a private directory only: trust is safe here and independent of the production pg_hba.
    echo 'local all all trust' > "$pgdata/pg_hba.conf"
    : > "$pgdata/pg_ident.conf"
    rm -f "$pgdata/postmaster.pid"
    if ! pg_ctl -D "$pgdata" -l "$work/postgres.log" -w -t "$START_TIMEOUT" start >/dev/null; then
      verdict="FAIL"; issues+=("scratch cluster did not start (see postgres.log in the run directory)")
      tail -n 20 "$work/postgres.log" >&2 || true
    else
      for _ in $(seq 1 "$START_TIMEOUT"); do
        [ "$(psql -h "$sock" -p "$PORT" -U postgres -d postgres -tAXc 'SELECT pg_is_in_recovery()' 2>/dev/null || echo t)" = f ] && break
        sleep 1
      done
      if [ "$(psql -h "$sock" -p "$PORT" -U postgres -d postgres -tAXc 'SELECT pg_is_in_recovery()')" != f ]; then
        verdict="FAIL"; issues+=("recovery did not reach the target and promote")
      fi
      dbs="$ALL_DBS"; [ "$MODE" = module ] && dbs="$DB"
      livepass=""
      if [ -s "${EW_SECRETS_DIR:-/run/secrets}/backup_pgpass" ]; then
        livepass=$(mktemp); cat "${EW_SECRETS_DIR:-/run/secrets}/backup_pgpass" > "$livepass"; chmod 600 "$livepass"
      fi
      for db in $dbs; do
        if ! psql -h "$sock" -p "$PORT" -U postgres -d "$db" -tAXc 'SELECT 1' >/dev/null 2>&1; then
          verdict="FAIL"; issues+=("$db: not reachable after restore"); continue
        fi
        rev=$(psql -h "$sock" -p "$PORT" -U postgres -d "$db" -tAXc \
          "SELECT coalesce(max(version),'-') FROM atlas_schema_revisions.atlas_schema_revisions" 2>/dev/null || echo "n/a")
        recon_lines+=("$db: reachable, Atlas revision $rev")
        var="EW_RESTORE_RECONCILE_SQL_$(printf '%s' "$db" | tr '[:lower:]' '[:upper:]')"
        sql="${!var:-}"
        [ -z "$sql" ] && [ "$db" = eduworld_learning ] && sql="$DEFAULT_LEARNING_SQL"
        [ -n "$sql" ] || continue
        restored=$(psql -h "$sock" -p "$PORT" -U postgres -d "$db" -tAX -v t="$TARGET" -v grace="$GRACE" <<< "$sql" 2>&1) || restored="error"
        if [ -z "$livepass" ]; then
          verdict="FAIL"; issues+=("$db: no live credential (backup_pgpass) to reconcile"); continue
        fi
        live=$(PGPASSFILE="$livepass" PGCONNECT_TIMEOUT=10 psql -h "$LIVE_HOST" -p "$LIVE_PORT" -U "$LIVE_USER" -d "$db" -tAX \
          -v t="$TARGET" -v grace="$GRACE" <<< "$sql" 2>&1) || live="error"
        recon_lines+=("$db: reconcile (rows up to T − ${GRACE}s) restored=$restored live=$live")
        if [ "$restored" != "$live" ] || [ "$restored" = error ]; then
          verdict="FAIL"; issues+=("$db: reconciliation differs (restored $restored, live $live)")
        fi
      done
      [ -n "$livepass" ] && rm -f "$livepass"
    fi
  fi
fi
end=$(date +%s)
duration=$(( end - start ))
if [ "$verdict" = PASS ] && [ "$duration" -gt "$MAX_SECONDS" ]; then
  verdict="FAIL"; issues+=("duration ${duration}s exceeds ${MAX_SECONDS}s (BACKUP_RECOVERY §8)")
fi

kind="Restore full + PITR (hằng tháng)"; [ "$MODE" = module ] && kind="Restore một database module ($DB, PITR)"
mkdir -p "$REPORT_DIR"
report="$REPORT_DIR/$(date -u +%Y-%m).md"
if [ ! -f "$report" ]; then
  printf '# Restore tests %s\n\nBiên bản kiểm thử khôi phục (BACKUP_RECOVERY §8), sinh bởi scripts/backup/restore_test.sh.\n' "$(date -u +%Y-%m)" > "$report"
fi
{
  printf '\n## %s — %s\n\n' "$started_at" "$kind"
  printf '| Trường | Giá trị |\n|---|---|\n'
  printf '| Loại kiểm thử | %s |\n' "$kind"
  printf '| Môi trường | %s |\n' "$EW_BACKUP_ENV"
  printf '| Người thực hiện | %s |\n' "$ACTOR"
  printf '| Mục đích | %s |\n' "$PURPOSE"
  printf '| Thời điểm đích T | %s |\n' "$target_desc"
  printf '| Bắt đầu | %s |\n' "$started_at"
  printf '| Kết thúc | %s |\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf '| Thời lượng | %ss (ngưỡng %ss) |\n' "$duration" "$MAX_SECONDS"
  printf '| Kết quả | **%s** |\n' "$verdict"
  printf '| Pipeline | %s |\n' "${PIPELINE:-—}"
  printf '\n**Kết quả đối soát**\n\n'
  if [ ${#recon_lines[@]} -eq 0 ]; then echo "- (không có)"; else printf -- '- %s\n' "${recon_lines[@]}"; fi
  printf '\n**Vấn đề**\n\n'
  if [ ${#issues[@]} -eq 0 ]; then echo "- Không có"; else printf -- '- %s\n' "${issues[@]}"; fi
} >> "$report"

ok=0; [ "$verdict" = PASS ] && ok=1
[ "$DRY_RUN" = true ] || metrics_write postgres "restore_${MODE}" "$ok" "$start" "$end"
audit restore_finished mode="$MODE" db="${DB:-all}" target="$target_desc" purpose="$PURPOSE" verdict="$verdict" seconds="$duration" report="$report"
log_json info "restore test finished" verdict="$verdict" seconds="$duration" report="$report"
[ "$verdict" = PASS ] || [ "$verdict" = DRY-RUN ]
