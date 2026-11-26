#!/usr/bin/env bash
# RB-04 — restore Keycloak (F130-AC-19; BACKUP_RECOVERY §4, RB-04). A database restored to an older time can bring back
# revoked sessions/refresh tokens, so logins reopen only after the realm keys are rotated and not-before is set.
#   rb04-keycloak-restore.sh --phase=plan                                  (default: prints the runbook)
#   rb04-keycloak-restore.sh --phase=rotate --actor=… --purpose=… [--old-token-file=F] --yes
# Phase rotate (Admin REST API, client of the ops job, secret ops_kc_admin_secret — never in a stack):
#   1. create new rsa-generated, hmac-generated and aes-generated key providers with a priority above every existing one;
#   2. disable the previous generated providers (tokens signed with them no longer verify);
#   3. set the realm not-before to now and push the revocation to the clients;
#   4. with --old-token-file, introspect a token issued before the restore: it must be inactive (exit 1 otherwise).
set -euo pipefail
# shellcheck source=scripts/backup/lib-backup.sh
. "$(dirname "$0")/../backup/lib-backup.sh" 2>/dev/null || . /opt/eduworld/backup/lib-backup.sh

PHASE=plan ACTOR="${EW_ACTOR:-}" PURPOSE="" OLD_TOKEN="" YES=false
for arg in "$@"; do
  case "$arg" in
    --phase=*) PHASE="${arg#*=}" ;;
    --actor=*) ACTOR="${arg#*=}" ;;
    --purpose=*) PURPOSE="${arg#*=}" ;;
    --old-token-file=*) OLD_TOKEN="${arg#*=}" ;;
    --yes) YES=true ;;
    *) die "unknown argument $arg" ;;
  esac
done
KC="${EW_KC_URL:-http://keycloak:8080}"
REALM="${EW_KC_REALM:-eduworld}"
ADMIN_REALM="${EW_KC_ADMIN_REALM:-master}"
CLIENT="${EW_KC_OPS_CLIENT_ID:-ew-ops-admin}"
CURL="${CURL_BIN:-curl}"

if [ "$PHASE" = plan ]; then
  cat <<PLAN
RB-04 plan (BACKUP_RECOVERY §9 RB-04)
 1. Scale keycloak to 0 (logins stop; tokens already issued keep working until step 3).
 2. Restore the keycloak database: rb01-pitr-restore.sh --phase=restore --db=keycloak --target=… then --phase=swap.
 3. Scale keycloak back; apply the realm from Git (keycloak-config-cli, make keycloak-import ENV=<env>).
 4. make ops-job ENV=<env> LIKE=backup-tools SECRET=ops_kc_admin_secret -- rb04-keycloak-restore.sh --phase=rotate --yes …
 5. Test logins per account type (student, school staff, platform staff) and IdP brokering.
 6. Total loss of the database: realm from Git → SIS full sync → job identity-relink (IDENTITY_MAPPING).
PLAN
  exit 0
fi
[ "$PHASE" = rotate ] || die "--phase=plan|rotate"
[ "$YES" = true ] || die "rotation logs every user out: pass --yes"
[ -n "$ACTOR" ] && [ -n "$PURPOSE" ] || die "--actor and --purpose are required (F130-AC-15)"
export EW_ACTOR="$ACTOR"
load_secret KC_OPS_SECRET "${EW_SECRETS_DIR:-/run/secrets}/ops_kc_admin_secret"
work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
chmod 700 "$work"
printf 'data-urlencode = "client_secret=%s"\n' "$KC_OPS_SECRET" > "$work/cred"
token=$("$CURL" -fsS -K "$work/cred" -d grant_type=client_credentials -d "client_id=$CLIENT" \
  "$KC/realms/$ADMIN_REALM/protocol/openid-connect/token" | jq -r '.access_token // empty')
[ -n "$token" ] || die "cannot obtain an admin token"
printf 'header = "Authorization: Bearer %s"\n' "$token" > "$work/auth"
kc() { "$CURL" -fsS -K "$work/auth" -H 'Content-Type: application/json' "$@"; }

audit rb04_rotate_started realm="$REALM" purpose="$PURPOSE"
realm_id=$(kc "$KC/admin/realms/$REALM" | jq -r '.id')
kc "$KC/admin/realms/$REALM/components?type=org.keycloak.keys.KeyProvider" > "$work/keys.json"
prio=$(jq '[.[].config.priority[0] // "0" | tonumber] | max // 100' "$work/keys.json")
stamp=$(date -u +%Y%m%d%H%M%S)
for provider in rsa-generated hmac-generated aes-generated; do
  body=$(jq -nc --arg n "$provider-$stamp" --arg p "$provider" --arg parent "$realm_id" --arg prio "$((prio + 10))" \
    '{name:$n, providerId:$p, providerType:"org.keycloak.keys.KeyProvider", parentId:$parent, config:{priority:[$prio], enabled:["true"], active:["true"]}}')
  kc -X POST "$KC/admin/realms/$REALM/components" -d "$body" >/dev/null || die "cannot create $provider key provider"
done
jq -r --arg s "$stamp" '.[] | select(.providerId | test("^(rsa|hmac|aes)-generated$")) | select(.name | endswith($s) | not) | .id' "$work/keys.json" \
  | while read -r id; do
      jq -c --arg id "$id" '.[] | select(.id == $id) | .config.enabled = ["false"] | .config.active = ["false"]' "$work/keys.json" > "$work/c.json"
      kc -X PUT "$KC/admin/realms/$REALM/components/$id" -d @"$work/c.json" >/dev/null || die "cannot disable key provider $id"
    done
now=$(date +%s)
kc -X PUT "$KC/admin/realms/$REALM" -d "{\"notBefore\": $now}" >/dev/null || die "cannot set not-before"
kc -X POST "$KC/admin/realms/$REALM/push-revocation" >/dev/null || log_json warn "push-revocation failed (clients still check not-before on refresh)"
if [ -n "$OLD_TOKEN" ]; then
  [ -s "$OLD_TOKEN" ] || die "old token file empty"
  printf 'data-urlencode = "token=%s"\n' "$(cat "$OLD_TOKEN")" > "$work/tok"
  active=$("$CURL" -fsS -K "$work/cred" -K "$work/tok" -d "client_id=$CLIENT" \
    "$KC/realms/$REALM/protocol/openid-connect/token/introspect" | jq -r '.active')
  [ "$active" = false ] || die "a token issued before the restore is still active"
fi
audit rb04_rotate_finished realm="$REALM" not_before="$now" new_providers="rsa,hmac,aes-$stamp"
log_json info "realm keys rotated and not-before set: logins may reopen" realm="$REALM"
