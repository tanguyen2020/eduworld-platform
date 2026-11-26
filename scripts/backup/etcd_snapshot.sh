#!/usr/bin/env bash
# Daily snapshot of the APISIX etcd (F130-AC-14; BACKUP_RECOVERY §2: Git is the source of truth, the snapshot is a
# fast restore path). Saved, checked (`etcdctl snapshot status`) and stored encrypted in <repo bucket>/etcd/<day>.db
# (30 days by bucket lifecycle). Metric target="etcd",type="snapshot".
set -euo pipefail
# shellcheck source=scripts/backup/lib-backup.sh
. "$(dirname "$0")/lib-backup.sh"

ENDPOINTS="${EW_ETCD_ENDPOINTS:-http://etcd-apisix-1:2379,http://etcd-apisix-2:2379,http://etcd-apisix-3:2379}"
day=$(date -u +%Y-%m-%d)
load_secret ETCD_BACKUP_PASSWORD "${EW_SECRETS_DIR:-/run/secrets}/backup_etcd_password"
load_repo_write_credentials
rclone_s3_remote repo "${EW_BACKUP_REPO_URL:?EW_BACKUP_REPO_URL required}" "${PGBACKREST_REPO1_S3_REGION:?}" PGBACKREST_REPO1_S3_KEY PGBACKREST_REPO1_S3_KEY_SECRET
rclone_crypt_remote repocrypt "repo:${PGBACKREST_REPO1_S3_BUCKET:?}/etcd" PGBACKREST_REPO1_CIPHER_PASS

work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
start=$(date +%s)
status=1
# One endpoint is enough for a consistent snapshot; try them in order.
export ETCDCTL_API=3 ETCDCTL_USER="${EW_ETCD_BACKUP_USER:-backup}:$ETCD_BACKUP_PASSWORD"
saved=false
IFS=',' read -r -a eps <<< "$ENDPOINTS"
for ep in "${eps[@]}"; do
  if etcdctl --endpoints="$ep" snapshot save "$work/snapshot.db" >/dev/null 2>"$work/err"; then saved=true; break; fi
  log_json warn "snapshot from endpoint failed" endpoint="$ep"
done
unset ETCDCTL_USER
if [ "$saved" = true ] && etcdctl snapshot status "$work/snapshot.db" -w json > "$work/status.json" 2>/dev/null \
   && rclone copyto "$work/snapshot.db" "repocrypt:$day.db"; then
  log_json info "etcd snapshot stored" revision="$(jq -r '.revision' "$work/status.json")" keys="$(jq -r '.totalKey' "$work/status.json")"
else
  status=0; log_json error "etcd snapshot failed"
fi
metrics_write etcd snapshot "$status" "$start" "$(date +%s)"
[ "$status" = 1 ] || exit 1
