#!/usr/bin/env bash
# Weekly logical dump `pg_dump -Fc` of every module database (F130-AC-12, BACKUP_RECOVERY §3.2): selective restore of
# one database/table. Dumps are streamed, encrypted client-side (rclone crypt, repo cipher) to
# <repo bucket>/logical/<database>/<YYYY-MM-DD>.dump; retention (4 copies) is the bucket lifecycle of the prefix —
# this script never deletes. Read role `backup_reader` (pg_read_all_data) through the replica router, password from
# the Docker secret backup_pgpass (.pgpass format). Metric target="postgres",type="logical".
set -euo pipefail
# shellcheck source=scripts/backup/lib-backup.sh
. "$(dirname "$0")/lib-backup.sh"

DATABASES="${EW_BACKUP_DATABASES:-eduworld_identity eduworld_learning eduworld_game eduworld_competition eduworld_chat}"
PGHOST="${EW_BACKUP_PG_HOST:-pg-router}"
PGPORT="${EW_BACKUP_PG_PORT:-5433}"
PGUSER="${EW_BACKUP_PG_USER:-backup_reader}"
export PGHOST PGPORT PGUSER PGCONNECT_TIMEOUT=10
day=$(date -u +%Y-%m-%d)

pgpass=$(mktemp); trap 'rm -f "$pgpass"' EXIT
cat "${EW_SECRETS_DIR:-/run/secrets}/backup_pgpass" > "$pgpass" 2>/dev/null || die "PGPASSFILE: secret file backup_pgpass missing"
[ -s "$pgpass" ] || die "PGPASSFILE: secret file backup_pgpass is empty"
chmod 600 "$pgpass"; export PGPASSFILE="$pgpass"

load_repo_write_credentials
rclone_s3_remote repo "${EW_BACKUP_REPO_URL:?EW_BACKUP_REPO_URL required}" "${PGBACKREST_REPO1_S3_REGION:?}" PGBACKREST_REPO1_S3_KEY PGBACKREST_REPO1_S3_KEY_SECRET
rclone_crypt_remote repocrypt "repo:${PGBACKREST_REPO1_S3_BUCKET:?}/logical" PGBACKREST_REPO1_CIPHER_PASS

start=$(date +%s)
status=1
for db in $DATABASES; do
  [[ "$db" =~ ^[a-z_]{1,63}$ ]] || die "invalid database name"
  dest="repocrypt:$db/$day.dump"
  log_json info "logical dump started" database="$db"
  if ! pg_dump -Fc --no-password -d "$db" | rclone rcat "$dest"; then
    status=0; log_json error "logical dump failed" database="$db"; continue
  fi
  size=$(rclone size --json "$dest" | jq -r '.bytes')
  if [ "${size:-0}" -le 0 ]; then status=0; log_json error "logical dump is empty" database="$db"; fi
  log_json info "logical dump done" database="$db" bytes="$size"
done
metrics_write postgres logical "$status" "$start" "$(date +%s)"
[ "$status" = 1 ] || exit 1
