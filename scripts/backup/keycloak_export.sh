#!/usr/bin/env bash
# Daily export of the realm configuration WITHOUT secrets (F130-AC-14; BACKUP_RECOVERY §4): Admin REST
# partial-export (clients, groups, roles; secrets masked by Keycloak), refused if any secret-like field is not masked,
# stored encrypted in <repo bucket>/keycloak/<day>.json, and compared with the realm as code in Git when available
# (drift reported, not failing). Client `svc-backup-export` (realm role view-realm + manage-realm for partial-export)
# with its client secret in the Docker secret backup_keycloak_export_secret. Metric target="keycloak_export",type="realm".
set -euo pipefail
# shellcheck source=scripts/backup/lib-backup.sh
. "$(dirname "$0")/lib-backup.sh"

KC="${EW_KC_URL:-http://keycloak:8080}"
REALM="${EW_KC_REALM:-eduworld}"
CLIENT="${EW_KC_EXPORT_CLIENT_ID:-svc-backup-export}"
day=$(date -u +%Y-%m-%d)
load_secret KC_EXPORT_SECRET "${EW_SECRETS_DIR:-/run/secrets}/backup_keycloak_export_secret"
work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
chmod 700 "$work"

start=$(date +%s)
status=1
# client secret in a curl config file (never argv)
printf 'data-urlencode = "client_secret=%s"\n' "$KC_EXPORT_SECRET" > "$work/cred"
token=$(curl -fsS -K "$work/cred" -d grant_type=client_credentials -d "client_id=$CLIENT" \
  "$KC/realms/$REALM/protocol/openid-connect/token" | jq -r '.access_token // empty') || token=""
if [ -z "$token" ]; then
  status=0; log_json error "cannot obtain an export token"
else
  printf 'header = "Authorization: Bearer %s"\n' "$token" > "$work/auth"
  if curl -fsS -K "$work/auth" -X POST "$KC/admin/realms/$REALM/partial-export?exportClients=true&exportGroupsAndRoles=true" -o "$work/realm.json"; then
    leaks=$(jq '[.. | objects | to_entries[] | select((.key | test("secret|password|credential|privateKey"; "i")) and (.value | type == "string") and (.value != "**********") and (.value != ""))] | length' "$work/realm.json")
    if [ "$leaks" != "0" ]; then
      status=0; log_json error "export contains unmasked secret-like fields: not stored" count="$leaks"
    else
      load_repo_write_credentials
      rclone_s3_remote repo "${EW_BACKUP_REPO_URL:?EW_BACKUP_REPO_URL required}" "${PGBACKREST_REPO1_S3_REGION:?}" PGBACKREST_REPO1_S3_KEY PGBACKREST_REPO1_S3_KEY_SECRET
      rclone_crypt_remote repocrypt "repo:${PGBACKREST_REPO1_S3_BUCKET:?}/keycloak" PGBACKREST_REPO1_CIPHER_PASS
      rclone copyto "$work/realm.json" "repocrypt:$day.json" || { status=0; log_json error "cannot store the realm export"; }
      if [ -n "${EW_KC_GIT_REALM_FILE:-}" ] && [ -f "$EW_KC_GIT_REALM_FILE" ]; then
        drift=$(jq -n --slurpfile a "$work/realm.json" --slurpfile b "$EW_KC_GIT_REALM_FILE" \
          '[$b[0].clients[]?.clientId] - [$a[0].clients[]?.clientId] | length')
        log_json info "realm drift vs Git (clients missing from the live realm)" count="$drift"
      fi
    fi
  else
    status=0; log_json error "partial-export failed"
  fi
fi
metrics_write keycloak_export realm "$status" "$start" "$(date +%s)"
[ "$status" = 1 ] || exit 1
log_json info "realm export stored" realm="$REALM"
