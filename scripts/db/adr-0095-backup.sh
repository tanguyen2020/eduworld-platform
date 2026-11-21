#!/usr/bin/env bash
# ADR-0095 mục 8 — backup plan (AGENTS R12) around the one-time contract migrations that DROP the class concepts.
# Local compose stack only (container "postgres", superuser postgres); staging/production follow BACKUP_RECOVERY (F130).
# The objects of each module are listed in scripts/db/rollback/adr-0095-<module>.manifest.
#
#   scripts/db/adr-0095-backup.sh <module>                          BEFORE the migration (steps 1–3):
#       pg_dump -Fc eduworld_<module> → data/backups/pre-adr0095-eduworld_<module>-<YYYYMMDD>.dump + .sha256,
#       pg_restore into eduworld_<module>_restore_check (must succeed), row counts of every manifest object on the live
#       database and on the restored copy (must be equal) → <dump>.counts-before.txt
#   scripts/db/adr-0095-backup.sh <module> --verify-rollback <dump>  BEFORE the migration (step 4, on scratch copies):
#       eduworld_<module>_rollback_check = copy of _restore_check; apply the contract migration, then
#       scripts/db/rollback/adr-0095-<module>.sql, then the data of the dump; counts must equal counts-before
#   scripts/db/adr-0095-backup.sh <module> --after <dump>           AFTER `make migrate-<module>`: counts → <dump>.counts-after.txt
#   scripts/db/adr-0095-backup.sh <module> --restore-data <dump>    REAL rollback, after the rollback SQL was applied to
#       the live database: data of dropped tables from the dump, values of dropped columns from _restore_check
#
# Read-only on the live database except --restore-data. Never DROPs anything but its own scratch databases
# (eduworld_<module>_restore_check, eduworld_<module>_rollback_check).
set -euo pipefail
cd "$(dirname "$0")/../.."

m="${1:?usage: adr-0095-backup.sh <module> [--verify-rollback|--after|--restore-data <dump>]}"
case "$m" in identity|learning|competition|chat) ;; *) echo "unknown module: $m (identity|learning|competition|chat)" >&2; exit 1 ;; esac
mode="${2:-backup}"
db="eduworld_${m}"
manifest="scripts/db/rollback/adr-0095-${m}.manifest"
rollback="scripts/db/rollback/adr-0095-${m}.sql"
[ -f "$manifest" ] && [ -f "$rollback" ] || { echo "missing $manifest or $rollback" >&2; exit 1; }
[ "${EW_ENV:-local}" = "local" ] || { echo "adr-0095-backup.sh only runs with EW_ENV=local (got ${EW_ENV})" >&2; exit 1; }

dc=(docker compose -f deploy/compose/docker-compose.yml)
psql_() { # psql_ <database> [psql args...]: superuser psql inside the postgres container (RLS does not apply)
  local d="$1"; shift
  MSYS_NO_PATHCONV=1 "${dc[@]}" exec -T postgres psql -U postgres -v ON_ERROR_STOP=1 -X -Atq -d "$d" "$@"
}
pg_() { MSYS_NO_PATHCONV=1 "${dc[@]}" exec -T postgres "$@"; }

entries() { grep -Ev '^\s*(#|$)' "$manifest"; }
migration_file() { entries | awk '$1 == "migration" { print $2 }'; }

# counts <database>: one line per manifest object, "dropped" when the table/column no longer exists.
counts() {
  local d="$1" kind obj a b
  while read -r kind obj a b; do
    case "$kind" in
      table)
        if [ "$(psql_ "$d" -c "SELECT to_regclass('$obj') IS NOT NULL")" = t ]; then
          echo "table $obj rows=$(psql_ "$d" -c "SELECT count(*) FROM $obj")"
        else echo "table $obj dropped"; fi ;;
      column)
        if [ "$(psql_ "$d" -c "SELECT count(*) FROM information_schema.columns WHERE table_schema || '.' || table_name = '$obj' AND column_name = '$b'")" = 1 ]; then
          echo "column $obj.$b non_null=$(psql_ "$d" -c "SELECT count($b) FROM $obj")"
        else echo "column $obj.$b dropped"; fi ;;
      values)
        echo "values $obj.$a in ($b) rows=$(psql_ "$d" -c "SELECT count(*) FROM $obj WHERE $a::text = ANY (string_to_array('$b', ','))")" ;;
    esac
  done < <(entries)
}

# restore_data <target database> <dump> <source database for column values>
restore_data() {
  local d="$1" dump="$2" src="$3" kind obj a b s t leaves args=()
  while read -r kind obj a b; do
    [ "$kind" = table ] || continue
    s="${obj%%.*}"; t="${obj#*.}"
    # Data of a partitioned table lives in its leaf partitions (listed from the pre-migration copy).
    leaves=$(psql_ "$src" -c "SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
      WHERE n.nspname = '$s' AND c.relkind = 'r' AND (c.relname = '$t' OR c.oid IN (
        WITH RECURSIVE p(oid) AS (SELECT '$obj'::regclass::oid UNION ALL SELECT i.inhrelid FROM pg_inherits i JOIN p ON i.inhparent = p.oid)
        SELECT oid FROM p))")
    for t in $leaves; do args+=(-n "$s" -t "$t"); done
  done < <(entries)
  if [ ${#args[@]} -gt 0 ]; then
    pg_ pg_restore -U postgres --data-only --disable-triggers "${args[@]}" -d "$d" < "$dump"
  fi
  while read -r kind obj a b; do
    [ "$kind" = column ] || continue
    local on="" sel="$a,$b" c
    for c in ${a//,/ }; do on="$on${on:+ AND }x.$c = r.$c"; done
    {
      echo "BEGIN; SET LOCAL session_replication_role = replica;"
      echo "CREATE TEMP TABLE r ON COMMIT DROP AS SELECT $sel FROM $obj WITH NO DATA;"
      echo "COPY r FROM STDIN;"
      psql_ "$src" -c "COPY (SELECT $sel FROM $obj WHERE $b IS NOT NULL) TO STDOUT"
      printf '%s\n' '\.'
      echo "UPDATE $obj x SET $b = r.$b FROM r WHERE $on;"
      echo "COMMIT;"
    } | MSYS_NO_PATHCONV=1 "${dc[@]}" exec -T postgres psql -U postgres -v ON_ERROR_STOP=1 -X -q -d "$d" -f -
  done < <(entries)
}

as_migrator() { # as_migrator <database> <file>: apply a SQL file in one transaction as the module's migrator role
  { echo "SET ROLE ${m}_migrator;"; cat "$2"; } | MSYS_NO_PATHCONV=1 "${dc[@]}" exec -T postgres \
    psql -U postgres -v ON_ERROR_STOP=1 -X -q --single-transaction -d "$1" -f -
}

case "$mode" in
  backup)
    mkdir -p data/backups
    dump="data/backups/pre-adr0095-${db}-$(date -u +%Y%m%d).dump"
    pg_ pg_dump -U postgres -Fc "$db" > "$dump"
    [ -s "$dump" ] || { echo "backup is empty: $dump" >&2; exit 1; }
    sha256sum "$dump" > "$dump.sha256"
    psql_ postgres -c "DROP DATABASE IF EXISTS ${db}_restore_check" -c "CREATE DATABASE ${db}_restore_check"
    pg_ pg_restore -U postgres --exit-on-error -d "${db}_restore_check" < "$dump"
    counts "$db" > "$dump.counts-before.txt"
    counts "${db}_restore_check" > "$dump.counts-restore-check.txt"
    diff -u "$dump.counts-before.txt" "$dump.counts-restore-check.txt" || { echo "restore check differs from $db" >&2; exit 1; }
    cat "$dump.sha256" "$dump.counts-before.txt"
    echo "backup OK: $dump (restored into ${db}_restore_check). Next: $0 $m --verify-rollback $dump" ;;
  --verify-rollback)
    dump="${3:?dump file}"; sha256sum -c "$dump.sha256"
    psql_ postgres -c "DROP DATABASE IF EXISTS ${db}_rollback_check" -c "CREATE DATABASE ${db}_rollback_check TEMPLATE ${db}_restore_check"
    as_migrator "${db}_rollback_check" "$(migration_file)"
    counts "${db}_rollback_check" > "$dump.counts-migrated-check.txt"
    as_migrator "${db}_rollback_check" "$rollback"
    restore_data "${db}_rollback_check" "$dump" "${db}_restore_check"
    counts "${db}_rollback_check" > "$dump.counts-rollback-check.txt"
    diff -u "$dump.counts-before.txt" "$dump.counts-rollback-check.txt" || { echo "rollback check differs from the backup" >&2; exit 1; }
    cat "$dump.counts-migrated-check.txt"
    echo "rollback verified on ${db}_rollback_check (migration → rollback SQL → data = counts before)" ;;
  --after)
    dump="${3:?dump file}"
    counts "$db" > "$dump.counts-after.txt"
    paste -d'|' "$dump.counts-before.txt" "$dump.counts-after.txt" ;;
  --restore-data)
    dump="${3:?dump file}"; sha256sum -c "$dump.sha256"
    restore_data "$db" "$dump" "${db}_restore_check"
    counts "$db" ;;
  *) echo "unknown mode: $mode" >&2; exit 1 ;;
esac
