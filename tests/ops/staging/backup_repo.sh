#!/usr/bin/env bash
# F130-AC-12, AC-15 on staging: pgbackrest info shows full + diff + continuous WAL with a cipher; the WRITE credential
# cannot read a backup data file, the restore credential (attached to one-off jobs only) can read the repo.
# shellcheck source=tests/ops/staging/lib.sh
. "$(dirname "$0")/lib.sh"
info=$(job --like=backup-tools -- sh -c '. /opt/eduworld/backup/lib-backup.sh; load_repo_write_credentials; pgbackrest --stanza=eduworld --output=json info' 2>/dev/null | grep '^\[')
check "full backup present" jq -e '[.[0].backup[].type] | index("full") != null' <<< "$info"
check "differential backup present" jq -e '[.[0].backup[].type] | index("diff") != null' <<< "$info"
check "repo encrypted (aes-256-cbc)" jq -e '.[0].cipher == "aes-256-cbc"' <<< "$info"
check "WAL archive continuous (max segment set)" jq -e '.[0].archive[0].max != null' <<< "$info"
read_data_file='. /opt/eduworld/backup/lib-backup.sh; load_repo_write_credentials
rclone_s3_remote r "$EW_BACKUP_REPO_URL" "$PGBACKREST_REPO1_S3_REGION" PGBACKREST_REPO1_S3_KEY PGBACKREST_REPO1_S3_KEY_SECRET
f=$(rclone lsf -R --files-only "r:$PGBACKREST_REPO1_S3_BUCKET/pgbackrest/backup/eduworld" | grep pg_data | head -1)
rclone cat "r:$PGBACKREST_REPO1_S3_BUCKET/pgbackrest/backup/eduworld/$f" >/dev/null'
if job --like=backup-tools -- sh -c "$read_data_file" >/dev/null 2>&1; then ko "write credential could read a backup data file"
else ok "write credential cannot read backup data files"; fi
restore_info='export PGBACKREST_REPO1_S3_KEY="$(cat /run/restore-credentials/backup_restore_access_key)"
export PGBACKREST_REPO1_S3_KEY_SECRET="$(cat /run/restore-credentials/backup_restore_secret_key)"
export PGBACKREST_REPO1_CIPHER_PASS="$(cat /run/secrets/backup_repo_cipher_pass)"
pgbackrest --stanza=eduworld info >/dev/null'
check "restore credential can read the repo (pgbackrest info)" job --like=backup-tools --restore -- sh -c "$restore_info"
finish
