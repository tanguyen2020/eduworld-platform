#!/usr/bin/env bash
# RB-01 — restore ONE database of the single cluster to T (PITR) and bring the modules back to consistency
# (F130-AC-17, AC-19 for keycloak; BACKUP_RECOVERY §3.5, §7, RB-01, RB-04 step 1). Runs inside a one-off
# eduworld/backup-tools job (scripts/ops/run-ops-job.sh) that holds the RESTORE credential and the admin .pgpass —
# neither is ever part of a stack (deploy/swarm/policy.yaml).
#
#   rb01-pitr-restore.sh --phase=restore --db=<database> --target="<YYYY-MM-DD HH:MM:SS+07>" --actor=… --purpose=…
#       PITR of the database into a scratch cluster, then pg_dump → a NEW database <db>_restored_<stamp> on the
#       production cluster (same privileges as <db>). The live database is untouched.
#   rb01-pitr-restore.sh --phase=swap --db=<database> --restored=<db>_restored_<stamp> --actor=… --purpose=… --yes
#       Terminates sessions of <db> (its routes answer 503 meanwhile, other modules keep serving) and swaps names:
#       <db> → <db>_pre_<stamp> (kept, never dropped: R12/R21), <db>_restored_<stamp> → <db>.
#   rb01-pitr-restore.sh --phase=plan --db=<database> --target=… (default) — prints the whole runbook, changes nothing.
#
# After the swap, from the deploy runner (printed by --phase=plan): reset the module consumers to T
# (reset-consumers.sh), rebuild ref_* projections (`worker projection rebuild`), `retention-reapply`, and
# `worker reconcile --module=<module>` (reconcile/retention-reapply are owned by the module features: [TBD]).
set -euo pipefail
# shellcheck source=scripts/backup/lib-backup.sh
. "$(dirname "$0")/../backup/lib-backup.sh" 2>/dev/null || . /opt/eduworld/backup/lib-backup.sh

PHASE=plan DB="" TARGET="" RESTORED="" ACTOR="${EW_ACTOR:-}" PURPOSE="" YES=false
for arg in "$@"; do
  case "$arg" in
    --phase=*) PHASE="${arg#*=}" ;;
    --db=*) DB="${arg#*=}" ;;
    --target=*) TARGET="${arg#*=}" ;;
    --restored=*) RESTORED="${arg#*=}" ;;
    --actor=*) ACTOR="${arg#*=}" ;;
    --purpose=*) PURPOSE="${arg#*=}" ;;
    --yes) YES=true ;;
    *) die "unknown argument $arg" ;;
  esac
done
[[ "$DB" =~ ^(eduworld_(identity|learning|game|competition|chat)|keycloak)$ ]] || die "--db=eduworld_<module>|keycloak required"
module=${DB#eduworld_}
export EW_ACTOR="$ACTOR"
stamp=$(date -u +%Y%m%d%H%M)

plan() {
  cat <<PLAN
RB-01 plan for $DB → T=${TARGET:-<T>} (BACKUP_RECOVERY §9 RB-01)
 1. Scope the incident: T_incident, module $module, tables affected (audit/log).
 2. Freeze writes of $module: disable its route prefixes on APISIX (or maintenance), pause its consumers/jobs.
 3. Incident channel + Incident Commander.
 4. make ops-job ENV=<env> LIKE=backup-tools RESTORE=1 -- rb01-pitr-restore.sh --phase=restore --db=$DB --target="$TARGET" --actor=… --purpose=…
 5. Verify the data of ${DB}_restored_<stamp> (read-only queries).
 6. Small scope: extract the right rows with a reviewed script in a transaction (audit). Global scope: continue.
    make ops-job ENV=<env> LIKE=backup-tools RESTORE=1 -- rb01-pitr-restore.sh --phase=swap --db=$DB --restored=${DB}_restored_<stamp> --yes …
 7. Consistency (§3.5): scale worker-events/worker-chat to 0 →
    make ops-job ENV=<env> LIKE=nats-box -- reset-consumers.sh --module=$module --since=<T UTC> --yes → scale back;
    make ops-job ENV=<env> LIKE=worker-jobs -- projection rebuild --module=$module --source=<module>;
    retention-reapply [TBD: command of the module features]; worker reconcile --module=$module [TBD].
$( [ "$DB" = keycloak ] && echo " 7b. Keycloak (RB-04): rb04-keycloak-restore.sh --phase=rotate — new realm keys + not-before before reopening logins." )
 8. Unfreeze, watch dashboards 1 h.  9. Post-mortem within 5 working days.
PLAN
}

admin_pgpass() {
  local f; f=$(mktemp); cat "${EW_SECRETS_DIR:-/run/secrets}/ops_pg_admin_pgpass" > "$f" 2>/dev/null || die "ops_pg_admin_pgpass missing"
  [ -s "$f" ] || die "ops_pg_admin_pgpass empty"; chmod 600 "$f"; echo "$f"
}

case "$PHASE" in
  plan) plan ;;
  restore)
    [ -n "$TARGET" ] && date -d "$TARGET" >/dev/null 2>&1 || die "--target=<timestamp with zone> required"
    [ -n "$ACTOR" ] && [ -n "$PURPOSE" ] || die "--actor and --purpose are required (F130-AC-15)"
    creds="${EW_RESTORE_CREDENTIALS_DIR:-/run/restore-credentials}"
    load_secret PGBACKREST_REPO1_S3_KEY "$creds/backup_restore_access_key"
    load_secret PGBACKREST_REPO1_S3_KEY_SECRET "$creds/backup_restore_secret_key"
    load_secret PGBACKREST_REPO1_CIPHER_PASS "${EW_SECRETS_DIR:-/run/secrets}/backup_repo_cipher_pass"
    work="${EW_RESTORE_WORK_DIR:-/restore}/rb01-$stamp"; pgdata="$work/pgdata"; sock="$work/sock"
    mkdir -p "$pgdata" "$sock"; chmod 700 "$pgdata"
    trap 'pg_ctl -D "$pgdata" -m fast -w stop >/dev/null 2>&1 || true; rm -rf "$work"' EXIT
    audit rb01_restore_started db="$DB" target="$TARGET" purpose="$PURPOSE"
    "${PGBACKREST_BIN:-pgbackrest}" --stanza="${PGBACKREST_STANZA:-eduworld}" --reset-pg1-host --pg1-path="$pgdata" \
      --type=time --target="$TARGET" --target-action=promote --archive-mode=off --db-include="$DB" restore
    printf "listen_addresses = ''\nport = 55433\nunix_socket_directories = '%s'\narchive_mode = off\nhot_standby = off\nshared_buffers = '256MB'\nshared_preload_libraries = ''\nhba_file = '%s/pg_hba.conf'\nident_file = '%s/pg_ident.conf'\nssl = off\nrestore_command = '%s --stanza=%s --reset-pg1-host --pg1-path=%s archive-get %%f \"%%p\"'\n" \
      "$sock" "$pgdata" "$pgdata" "${PGBACKREST_BIN:-pgbackrest}" "${PGBACKREST_STANZA:-eduworld}" "$pgdata" >> "$pgdata/postgresql.auto.conf"
    echo 'local all all trust' > "$pgdata/pg_hba.conf"; : > "$pgdata/pg_ident.conf"; rm -f "$pgdata/postmaster.pid"
    pg_ctl -D "$pgdata" -l "$work/pg.log" -w -t 3600 start >/dev/null
    until [ "$(psql -h "$sock" -p 55433 -U postgres -d postgres -tAXc 'SELECT pg_is_in_recovery()')" = f ]; do sleep 2; done
    pg_dump -h "$sock" -p 55433 -U postgres -Fc -d "$DB" -f "$work/db.dump"
    pgpass=$(admin_pgpass); export PGPASSFILE="$pgpass"
    host="${EW_PG_ADMIN_HOST:-pg-router}" port="${EW_PG_ADMIN_PORT:-5432}" user="${EW_PG_ADMIN_USER:-postgres}"
    new="${DB}_restored_$stamp"
    psql -h "$host" -p "$port" -U "$user" -d postgres -v ON_ERROR_STOP=1 -qc "CREATE DATABASE \"$new\" TEMPLATE template0"
    pg_restore -h "$host" -p "$port" -U "$user" -d "$new" --exit-on-error "$work/db.dump"
    # Database-level privileges (CONNECT …) are not in the dump: copy the ACL and owner of the live database.
    psql -h "$host" -p "$port" -U "$user" -d postgres -v ON_ERROR_STOP=1 -qX -v src="$DB" -v dst="$new" <<'SQL'
SELECT format('ALTER DATABASE %I OWNER TO %I', :'dst', pg_get_userbyid(datdba)) FROM pg_database WHERE datname = :'src' \gexec
SELECT format('GRANT %s ON DATABASE %I TO %I', a.privilege_type, :'dst', pg_get_userbyid(a.grantee))
  FROM pg_database d, aclexplode(d.datacl) a WHERE d.datname = :'src' AND a.grantee <> 0 \gexec
SQL
    rm -f "$pgpass"
    audit rb01_restore_finished db="$DB" target="$TARGET" restored_database="$new"
    log_json info "restored into a new database; verify it, then run --phase=swap" restored="$new"
    ;;
  swap)
    [ "$YES" = true ] || die "swap renames the live database: pass --yes"
    [[ "$RESTORED" =~ ^${DB}_restored_[0-9]{12}$ ]] || die "--restored=${DB}_restored_<stamp> required"
    [ -n "$ACTOR" ] && [ -n "$PURPOSE" ] || die "--actor and --purpose are required (F130-AC-15)"
    pgpass=$(admin_pgpass); export PGPASSFILE="$pgpass"
    host="${EW_PG_ADMIN_HOST:-pg-router}" port="${EW_PG_ADMIN_PORT:-5432}" user="${EW_PG_ADMIN_USER:-postgres}"
    old="${DB}_pre_$stamp"
    audit rb01_swap_started db="$DB" restored="$RESTORED" kept_as="$old" purpose="$PURPOSE"
    psql -h "$host" -p "$port" -U "$user" -d postgres -v ON_ERROR_STOP=1 -qX -v db="$DB" -v old="$old" -v new="$RESTORED" <<'SQL'
SELECT format('ALTER DATABASE %I WITH ALLOW_CONNECTIONS false', :'db') \gexec
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = :'db' AND pid <> pg_backend_pid();
SELECT format('ALTER DATABASE %I RENAME TO %I', :'db', :'old') \gexec
SELECT format('ALTER DATABASE %I RENAME TO %I', :'new', :'db') \gexec
SELECT format('ALTER DATABASE %I WITH ALLOW_CONNECTIONS true', :'db') \gexec
SQL
    rm -f "$pgpass"
    audit rb01_swap_finished db="$DB" kept_as="$old"
    log_json info "database swapped; continue with consumers, projections, retention-reapply, reconcile" db="$DB" kept_as="$old"
    ;;
  *) die "--phase=plan|restore|swap" ;;
esac
