#!/usr/bin/env bash
# End-to-end replication lag probe of the private bucket(s) (F130-AC-14 b; BACKUP_RECOVERY §6; ADR-0075): writes a
# canary object under _replication-canary/ of the primary bucket, polls the second location until the same content is
# readable and exports eduworld_object_storage_replication_lag_seconds{bucket}. The run fails (status 0) when the copy
# is not visible within the RPO of learning data (`rpo_rto`, EW_RPO_SECONDS, default 300 s = 5 minutes).
# Replication itself is the backend's: SeaweedFS `filer.sync` to the second site (data.stack.yml) or the managed
# provider's cross-region replication after RB-05b. Metric target="object_storage",type="replication".
set -euo pipefail
# shellcheck source=scripts/backup/lib-backup.sh
. "$(dirname "$0")/lib-backup.sh"

BUCKETS="${EW_REPLICATED_BUCKETS:-ew-media-private}"
RPO="${EW_RPO_SECONDS:-300}"
POLL="${EW_REPLICATION_POLL_SECONDS:-5}"
SECRETS="${EW_SECRETS_DIR:-/run/secrets}"
load_secret SRC_ACCESS_KEY "$SECRETS/backup_source_access_key"
load_secret SRC_SECRET_KEY "$SECRETS/backup_source_secret_key"
load_secret REPLICA_ACCESS_KEY "${EW_REPLICA_ACCESS_KEY_FILE:-$SECRETS/backup_source_access_key}"
load_secret REPLICA_SECRET_KEY "${EW_REPLICA_SECRET_KEY_FILE:-$SECRETS/backup_source_secret_key}"
rclone_s3_remote src "${EW_BACKUP_SOURCE_URL:?EW_BACKUP_SOURCE_URL required}" "${EW_BACKUP_SOURCE_REGION:-us-east-1}" SRC_ACCESS_KEY SRC_SECRET_KEY
rclone_s3_remote replica "${EW_REPLICA_URL:?EW_REPLICA_URL required}" "${EW_REPLICA_REGION:-us-east-1}" REPLICA_ACCESS_KEY REPLICA_SECRET_KEY

start=$(date +%s)
status=1
extra=()
host=$(hostname | tr -cd 'a-z0-9-' | cut -c1-32)
for bucket in $BUCKETS; do
  [[ "$bucket" =~ ^[a-z0-9][a-z0-9.-]{2,62}$ ]] || die "invalid bucket name"
  key="_replication-canary/${host:-probe}"
  token="$(date +%s%N)-$RANDOM"
  t0=$(date +%s)
  if ! printf '%s' "$token" | rclone rcat "src:$bucket/$key"; then
    status=0; log_json error "cannot write canary" bucket="$bucket"; continue
  fi
  lag=-1
  while :; do
    now=$(date +%s)
    if [ "$(rclone cat "replica:$bucket/$key" 2>/dev/null || true)" = "$token" ]; then lag=$((now - t0)); break; fi
    if [ $((now - t0)) -ge "$RPO" ]; then break; fi
    sleep "$POLL"
  done
  if [ "$lag" -lt 0 ]; then
    status=0; lag=$(( $(date +%s) - t0 ))
    log_json error "replication lag exceeds the RPO" bucket="$bucket" rpo_seconds="$RPO"
  else
    log_json info "replication lag measured" bucket="$bucket" lag_seconds="$lag"
  fi
  extra+=("eduworld_object_storage_replication_lag_seconds{bucket=\"$bucket\"} $lag")
done
metrics_write object_storage replication "$status" "$start" "$(date +%s)" "${extra[@]}"
[ "$status" = 1 ] || exit 1
