#!/usr/bin/env bash
# RB-05 — restore object storage from the off-cluster copy into a NEW cluster/bucket set with the SAME bucket names
# (F130-AC-20; BACKUP_RECOVERY §6, §8, RB-05; ADR-0075, ADR-0093). Works for SeaweedFS and a managed S3 (standard
# S3 API). The private bucket(s) come from the continuous replica when EW_REPLICA_URL is set (RPO of learning data),
# the others from the latest off-site copy (RPO 24 h).
#   rb05-object-storage-restore.sh --referenced=FILE [--buckets="…"] [--from-replica="ew-media-private"] [--dry-run] --yes
# FILE lists the object keys referenced by media_assets (one "<bucket> <key>" per line, exported by the owning modules).
# Verification: 100 % of referenced keys exist in the target; media/<sha256>/original.* hash == <sha256>; every other
# referenced key has the same sha256 as in the copy it came from. Missing keys are listed for the Content Admin
# (BACKUP_RECOVERY RB-05 step 4). Credentials of the NEW target are new Docker secrets (EW_TARGET_*_FILE).
# Losing ONE SeaweedFS node needs no restore: replace the node and run `volume.fix.replication` (RB-05 step 1).
set -euo pipefail
# shellcheck source=scripts/backup/lib-backup.sh
. "$(dirname "$0")/../backup/lib-backup.sh" 2>/dev/null || . /opt/eduworld/backup/lib-backup.sh

BUCKETS="${EW_BACKUP_BUCKETS:-ew-media-public ew-media-private ew-game-bundles ew-exports}"
FROM_REPLICA="${EW_REPLICATED_BUCKETS:-ew-media-private}"
REFERENCED="" DRY_RUN=false YES=false
for arg in "$@"; do
  case "$arg" in
    --referenced=*) REFERENCED="${arg#*=}" ;;
    --buckets=*) BUCKETS="${arg#*=}" ;;
    --from-replica=*) FROM_REPLICA="${arg#*=}" ;;
    --dry-run) DRY_RUN=true ;;
    --yes) YES=true ;;
    *) die "unknown argument $arg" ;;
  esac
done
[ -f "$REFERENCED" ] || die "--referenced=<file of '<bucket> <key>' lines> is required"
[ "$DRY_RUN" = true ] || [ "$YES" = true ] || die "restore writes into the new target: pass --yes (or --dry-run)"
SECRETS="${EW_SECRETS_DIR:-/run/secrets}"
load_secret OFF_ACCESS_KEY "$SECRETS/backup_offsite_access_key"
load_secret OFF_SECRET_KEY "$SECRETS/backup_offsite_secret_key"
load_secret OFF_CRYPT_PASSWORD "$SECRETS/backup_offsite_crypt_password"
load_secret TGT_ACCESS_KEY "${EW_TARGET_ACCESS_KEY_FILE:?EW_TARGET_ACCESS_KEY_FILE required}"
load_secret TGT_SECRET_KEY "${EW_TARGET_SECRET_KEY_FILE:?EW_TARGET_SECRET_KEY_FILE required}"
rclone_s3_remote off "${EW_BACKUP_OFFSITE_URL:?}" "${EW_BACKUP_OFFSITE_REGION:-us-east-1}" OFF_ACCESS_KEY OFF_SECRET_KEY
rclone_crypt_remote offcrypt "off:${EW_BACKUP_OFFSITE_BUCKET:?}/${EW_BACKUP_OFFSITE_PREFIX:-object-storage}" OFF_CRYPT_PASSWORD
rclone_s3_remote tgt "${EW_TARGET_URL:?EW_TARGET_URL required}" "${EW_TARGET_REGION:-us-east-1}" TGT_ACCESS_KEY TGT_SECRET_KEY
if [ -n "${EW_REPLICA_URL:-}" ]; then
  load_secret REPLICA_ACCESS_KEY "${EW_REPLICA_ACCESS_KEY_FILE:?}"
  load_secret REPLICA_SECRET_KEY "${EW_REPLICA_SECRET_KEY_FILE:?}"
  rclone_s3_remote replica "$EW_REPLICA_URL" "${EW_REPLICA_REGION:-us-east-1}" REPLICA_ACCESS_KEY REPLICA_SECRET_KEY
fi

start=$(date +%s)
audit rb05_started buckets="$BUCKETS" from_replica="$FROM_REPLICA" dry_run="$DRY_RUN"
source_of() {
  local b=$1
  if [ -n "${EW_REPLICA_URL:-}" ] && [[ " $FROM_REPLICA " == *" $b "* ]]; then echo "replica:$b"; else echo "offcrypt:current/$b"; fi
}
for bucket in $BUCKETS; do
  [ "$bucket" = ew-media-quarantine ] && continue
  [[ "$bucket" =~ ^[a-z0-9][a-z0-9.-]{2,62}$ ]] || die "invalid bucket name"
  src=$(source_of "$bucket")
  if [ "$DRY_RUN" = true ]; then log_json info "would restore bucket" bucket="$bucket" from="${src%%:*}"; continue; fi
  rclone mkdir "tgt:$bucket"
  rclone copy "$src" "tgt:$bucket" --checksum --fast-list --log-level ERROR || die "copy failed for $bucket"
  log_json info "bucket restored" bucket="$bucket" from="${src%%:*}"
done
[ "$DRY_RUN" = true ] && exit 0

total=0 ok=0 missing=0 bad=0
missing_file=$(mktemp)
while read -r bucket key; do
  [ -n "${bucket:-}" ] || continue
  total=$((total + 1))
  if [ -z "$(rclone lsf "tgt:$bucket/$key" 2>/dev/null)" ]; then
    missing=$((missing + 1)); echo "$bucket $key" >> "$missing_file"; continue
  fi
  got=$(rclone cat "tgt:$bucket/$key" | sha256sum | cut -d" " -f1)
  if [[ "$key" =~ ^media/([0-9a-f]{64})/original\. ]]; then
    want=${BASH_REMATCH[1]}
  else
    want=$(rclone cat "$(source_of "$bucket")/$key" | sha256sum | cut -d' ' -f1)
  fi
  if [ "$got" = "$want" ]; then ok=$((ok + 1)); else bad=$((bad + 1)); log_json error "checksum mismatch" bucket="$bucket" key="$key"; fi
done < "$REFERENCED"
seconds=$(( $(date +%s) - start ))
audit rb05_finished referenced="$total" verified="$ok" missing="$missing" mismatched="$bad" seconds="$seconds"
if [ "$missing" -gt 0 ]; then
  log_json warn "referenced objects missing after restore: send the list to Content Admin" count="$missing"
  cat "$missing_file" >&2
fi
rm -f "$missing_file"
[ "$total" -gt 0 ] && [ "$ok" = "$total" ] || die "RB-05 verification failed: $ok/$total verified ($missing missing, $bad mismatched)"
log_json info "RB-05 verification: 100% of referenced objects verified" referenced="$total" seconds="$seconds"
