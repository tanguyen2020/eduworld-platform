#!/usr/bin/env bash
# Daily off-cluster / off-provider copy of object storage (F130-AC-14 c; BACKUP_RECOVERY §6; ADR-0075, ADR-0093).
# For every bucket except ew-media-quarantine (temporary files, never backed up):
#   rclone sync SRC:<bucket> → OFFSITE(crypt):current/<bucket>, objects replaced or removed at the source move to
#   OFFSITE:history/<YYYY-MM-DD>/<bucket> (nothing is deleted by this script; retention 30 daily + 12 monthly is the
#   lifecycle of the history/ prefixes on the off-site bucket, `object_storage_backup` of CONFIG_DEFAULTS);
# then verifies: `rclone cryptcheck` of the whole bucket (checksums through the encryption layer) and, for every
# object copied today under media/<sha256>/original.*, sha256(content read back from the off-site copy) == <sha256>.
# A manifest (<sha256>  <key>) of the verified originals is written to OFFSITE:manifests/<YYYY-MM-DD>/<bucket>.sha256.
# Metric target="object_storage",type="offsite" (+ eduworld_backup_objects_copied{bucket}).
# Source/off-site endpoints are configuration (backup.env); credentials are Docker secrets. Standard S3 API only.
set -euo pipefail
# shellcheck source=scripts/backup/lib-backup.sh
. "$(dirname "$0")/lib-backup.sh"

BUCKETS="${EW_BACKUP_BUCKETS:-ew-media-public ew-media-private ew-game-bundles ew-exports}"
SECRETS="${EW_SECRETS_DIR:-/run/secrets}"
day="${EW_BACKUP_DAY:-$(date -u +%Y-%m-%d)}"
[[ "$day" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || die "EW_BACKUP_DAY must be YYYY-MM-DD"

load_secret SRC_ACCESS_KEY "$SECRETS/backup_source_access_key"
load_secret SRC_SECRET_KEY "$SECRETS/backup_source_secret_key"
load_secret OFF_ACCESS_KEY "$SECRETS/backup_offsite_access_key"
load_secret OFF_SECRET_KEY "$SECRETS/backup_offsite_secret_key"
load_secret OFF_CRYPT_PASSWORD "$SECRETS/backup_offsite_crypt_password"
rclone_s3_remote src "${EW_BACKUP_SOURCE_URL:?EW_BACKUP_SOURCE_URL required}" "${EW_BACKUP_SOURCE_REGION:-us-east-1}" SRC_ACCESS_KEY SRC_SECRET_KEY
rclone_s3_remote off "${EW_BACKUP_OFFSITE_URL:?EW_BACKUP_OFFSITE_URL required}" "${EW_BACKUP_OFFSITE_REGION:-us-east-1}" OFF_ACCESS_KEY OFF_SECRET_KEY
OFF_BUCKET="${EW_BACKUP_OFFSITE_BUCKET:?EW_BACKUP_OFFSITE_BUCKET required}"
OFF_PREFIX="${EW_BACKUP_OFFSITE_PREFIX:-object-storage}"
rclone_crypt_remote offcrypt "off:$OFF_BUCKET/$OFF_PREFIX" OFF_CRYPT_PASSWORD

work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
start=$(date +%s)
status=1
extra=()
for bucket in $BUCKETS; do
  if [ "$bucket" = "ew-media-quarantine" ]; then
    log_json warn "bucket skipped: quarantine is never backed up" bucket="$bucket"; continue
  fi
  [[ "$bucket" =~ ^[a-z0-9][a-z0-9.-]{2,62}$ ]] || die "invalid bucket name"
  log_json info "sync started" bucket="$bucket"
  if ! rclone sync "src:$bucket" "offcrypt:current/$bucket" --backup-dir "offcrypt:history/$day/$bucket" \
      --checksum --fast-list --use-json-log --log-level INFO --log-file "$work/$bucket.log"; then
    status=0; log_json error "sync failed" bucket="$bucket"; continue
  fi
  copied=$(grep -c '"msg":"Copied (' "$work/$bucket.log" || true)
  extra+=("eduworld_backup_objects_copied{target=\"object_storage\",type=\"offsite\",bucket=\"$bucket\"} $copied")
  if ! rclone cryptcheck "src:$bucket" "offcrypt:current/$bucket" --one-way --log-level ERROR; then
    status=0; log_json error "checksum mismatch between source and off-site copy" bucket="$bucket"; continue
  fi
  # Content-addressed originals copied today: the key carries the sha256 of the content (ADR-0044).
  : > "$work/$bucket.sha256"
  { jq -r 'select(.msg | startswith("Copied (")) | .object // empty' "$work/$bucket.log" 2>/dev/null || true; } \
    | grep -E '^media/[0-9a-f]{64}/original\.' > "$work/$bucket.originals" || true
  bad=0
  while IFS= read -r key; do
    want=${key#media/}; want=${want%%/*}
    got=$(rclone cat "offcrypt:current/$bucket/$key" | sha256sum | cut -d' ' -f1)
    if [ "$got" != "$want" ]; then bad=$((bad + 1)); log_json error "content hash differs from its key" bucket="$bucket" key="$key"; fi
    printf '%s  %s\n' "$got" "$key" >> "$work/$bucket.sha256"
  done < "$work/$bucket.originals"
  rclone rcat "off:$OFF_BUCKET/$OFF_PREFIX/manifests/$day/$bucket.sha256" < "$work/$bucket.sha256"
  if [ "$bad" -gt 0 ]; then status=0; continue; fi
  log_json info "sync verified" bucket="$bucket" copied="$copied" originals_checked="$(wc -l < "$work/$bucket.originals" | tr -d ' ')"
done
metrics_write object_storage offsite "$status" "$start" "$(date +%s)" "${extra[@]}"
[ "$status" = 1 ] || exit 1
