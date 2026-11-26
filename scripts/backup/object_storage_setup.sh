#!/usr/bin/env bash
# Idempotent bucket settings that backups rely on (F130-AC-14 a; BACKUP_RECOVERY §6; DEPLOYMENT §9b): versioning on
# ew-media-public, ew-media-private, ew-game-bundles, checked afterwards (exit 1 if a bucket does not report Enabled).
# Uses the standard S3 API (PUT/GET ?versioning, SigV4 via curl --aws-sigv4), so it works on SeaweedFS and on a managed
# S3 after RB-05b. Credentials from Docker secrets (curl config file, never argv). Run at stack setup and by RB-05.
#   object_storage_setup.sh [--check-only]
set -euo pipefail
# shellcheck source=scripts/backup/lib-backup.sh
. "$(dirname "$0")/lib-backup.sh"

BUCKETS="${EW_VERSIONED_BUCKETS:-ew-media-public ew-media-private ew-game-bundles}"
ENDPOINT="${EW_BACKUP_SOURCE_URL:?EW_BACKUP_SOURCE_URL required}"
REGION="${EW_BACKUP_SOURCE_REGION:-us-east-1}"
SECRETS="${EW_SECRETS_DIR:-/run/secrets}"
CHECK_ONLY=false
[ "${1:-}" = "--check-only" ] && CHECK_ONLY=true
load_secret ADMIN_ACCESS_KEY "${EW_STORAGE_ADMIN_ACCESS_KEY_FILE:-$SECRETS/backup_source_access_key}"
load_secret ADMIN_SECRET_KEY "${EW_STORAGE_ADMIN_SECRET_KEY_FILE:-$SECRETS/backup_source_secret_key}"
cfg=$(mktemp); trap 'rm -f "$cfg"' EXIT
chmod 600 "$cfg"
printf 'user = "%s:%s"\n' "$ADMIN_ACCESS_KEY" "$ADMIN_SECRET_KEY" > "$cfg"

s3() { curl -fsS -K "$cfg" --aws-sigv4 "aws:amz:$REGION:s3" "$@"; }
rc=0
for bucket in $BUCKETS; do
  [[ "$bucket" =~ ^[a-z0-9][a-z0-9.-]{2,62}$ ]] || die "invalid bucket name"
  if [ "$CHECK_ONLY" = false ]; then
    s3 -X PUT -H 'Content-Type: application/xml' "$ENDPOINT/$bucket?versioning" \
      --data '<VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Status>Enabled</Status></VersioningConfiguration>' >/dev/null \
      || { log_json error "cannot enable versioning" bucket="$bucket"; rc=1; continue; }
  fi
  if s3 "$ENDPOINT/$bucket?versioning" | grep -q '<Status>Enabled</Status>'; then
    log_json info "versioning enabled" bucket="$bucket"
  else
    log_json error "versioning is not enabled" bucket="$bucket"; rc=1
  fi
done
exit "$rc"
