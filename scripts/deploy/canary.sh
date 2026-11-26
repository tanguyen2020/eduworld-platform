#!/usr/bin/env bash
# Production canary of core-api (F130-AC-04; DEPLOYMENT §11.2; research R4):
#   1. create Swarm service core-api-canary = spec of core-api with the NEW image (1–2 replicas, label ew.canary);
#   2. APISIX traffic-split: weight W % (default 10) of every route whose upstream is core-api → upstream
#      core-api-canary (tasks.core-api-canary), through the Admin API;
#   3. watch for D seconds (default 1800 = 30 minutes): abort as soon as Prometheus has an alert ModuleHighErrorRate
#      or APILatencyP95High pending/firing for service="core-api-canary" — thresholds are those of the alert rules
#      (MONITORING §5, rules of F131), never redefined here; no such rule loaded → abort (fail closed);
#   4a. abort: remove traffic-split, remove core-api-canary, exit 1 (core-api unchanged);
#   4b. pass: rolling update of core-api to the new image, remove traffic-split and core-api-canary, exit 0.
#   canary.sh --env=production --version=vX.Y.Z [--weight=10] [--duration=1800] [--interval=60] [--replicas=2]
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=scripts/deploy/lib-deploy.sh
. "$here/lib-deploy.sh"

ENV="" VERSION="" WEIGHT=10 DURATION=1800 INTERVAL=60 REPLICAS=2
for arg in "$@"; do
  case "$arg" in
    --env=*) ENV="${arg#*=}" ;;
    --version=*) VERSION="${arg#*=}" ;;
    --weight=*) WEIGHT="${arg#*=}" ;;
    --duration=*) DURATION="${arg#*=}" ;;
    --interval=*) INTERVAL="${arg#*=}" ;;
    --replicas=*) REPLICAS="${arg#*=}" ;;
    *) die "unknown argument $arg" ;;
  esac
done
check_env "$ENV"; check_version "$VERSION"
[[ "$WEIGHT" =~ ^[0-9]+$ ]] && [ "$WEIGHT" -ge 1 ] && [ "$WEIGHT" -le 50 ] || die "--weight must be 1..50"
[[ "$REPLICAS" =~ ^[12]$ ]] || die "--replicas must be 1 or 2 (DEPLOYMENT §11.2)"
load_stack_env "$ENV"
ADMIN_URL="${EW_APISIX_ADMIN_URL:?EW_APISIX_ADMIN_URL required}"
PROM="${EW_PROMETHEUS_URL:?EW_PROMETHEUS_URL required}"
[ -s "${EW_APISIX_ADMIN_KEY_FILE:-}" ] || die "EW_APISIX_ADMIN_KEY_FILE: secret file missing or empty"
work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
chmod 700 "$work"
printf 'header = "X-API-KEY: %s"\n' "$(cat "$EW_APISIX_ADMIN_KEY_FILE")" > "$work/apisix.cfg"
CURL="${CURL_BIN:-curl}"
apisix() { "$CURL" -fsS -K "$work/apisix.cfg" -H 'Content-Type: application/json' "$@"; }
canary="${STACK}_core-api-canary"
image="ghcr.io/$GHCR_ORG/eduworld/core-api:$VERSION"

# Routes synced by ADC (scripts/deploy/apisix-sync.sh) reference their service (service_id) and carry the label
# ew-upstream=core-api (deploy/apisix/scripts/render.mjs); upstream_id == core-api covers routes created otherwise.
routes_of_core_api() {
  apisix "$ADMIN_URL/apisix/admin/routes" \
    | jq -r '.list[]?.value | select(.labels["ew-upstream"] == "core-api" or .upstream_id == "core-api") | .id'
}
set_split() { # weight or "null"; fails when no core-api route exists (a canary without traffic would pass blindly)
  local body id ids
  if [ "$1" = null ]; then body='{"plugins":{"traffic-split":null}}'
  else body=$(jq -nc --argjson w "$1" '{plugins:{"traffic-split":{rules:[{weighted_upstreams:[{upstream_id:"core-api-canary",weight:$w},{weight:(100-$w)}]}]}}}')
  fi
  ids=$(routes_of_core_api) || return 1
  [ -n "$ids" ] || { log error "no APISIX route of upstream core-api (label ew-upstream): run apisix:sync first"; return 1; }
  for id in $ids; do apisix -X PATCH "$ADMIN_URL/apisix/admin/routes/$id" -d "$body" >/dev/null; done
}
cleanup_canary() {
  set_split null || log error "could not remove traffic-split: remove it by hand (routes with upstream core-api)"
  $DOCKER service rm "$canary" >/dev/null 2>&1 || true
}
abort() {
  audit_line canary_aborted service=core-api-canary tag="$VERSION" reason="$1"
  cleanup_canary
  die "canary aborted: $1 (core-api unchanged)"
}

# Fail closed when the alert rules the canary relies on are not loaded (F131 owns them).
rules=$("$CURL" -fsS "$PROM/api/v1/rules?type=alert" | jq -r '[.data.groups[].rules[].name] | join(" ")') || die "Prometheus unreachable"
for r in ModuleHighErrorRate APILatencyP95High; do
  [[ " $rules " == *" $r "* ]] || die "alert rule $r is not loaded in Prometheus: refusing to run a blind canary"
done

require_quorum
spec=$($DOCKER service inspect "${STACK}_core-api" --format '{{json .Spec}}') || die "core-api not found"
clone_service_args "$spec"
audit_line canary_started service=core-api-canary tag="$VERSION" weight="$WEIGHT" duration="$DURATION"
$DOCKER service create --detach=false --with-registry-auth --name "$canary" --replicas "$REPLICAS" \
  --label ew.canary=true --env OTEL_SERVICE_NAME=core-api-canary "${CLONE_ARGS[@]}" \
  --health-cmd "/app/core-api healthcheck --endpoint=/livez" --health-interval 10s --health-retries 3 --health-start-period 20s \
  "$image" >/dev/null || abort "canary service did not become healthy"
apisix -X PUT "$ADMIN_URL/apisix/admin/upstreams/core-api-canary" \
  -d '{"type":"roundrobin","discovery_type":"dns","service_name":"tasks.core-api-canary:8100","checks":{"active":{"http_path":"/readyz","healthy":{"interval":5},"unhealthy":{"interval":5}}}}' >/dev/null \
  || abort "cannot create the APISIX upstream core-api-canary"
set_split "$WEIGHT" || abort "cannot apply traffic-split"

query='ALERTS{alertname=~"ModuleHighErrorRate|APILatencyP95High",service="core-api-canary",alertstate=~"pending|firing"}'
deadline=$(( $(date +%s) + DURATION ))
while [ "$(date +%s)" -lt "$deadline" ]; do
  sleep "$INTERVAL"
  res=$("$CURL" -fsS -G "$PROM/api/v1/query" --data-urlencode "query=$query") || abort "Prometheus unreachable during the canary"
  n=$(jq '.data.result | length' <<< "$res")
  if [ "$n" -gt 0 ]; then
    abort "alert $(jq -r '[.data.result[].metric | "\(.alertname)/\(.module // "-")"] | join(",")' <<< "$res")"
  fi
done

audit_line canary_promoted service=core-api tag="$VERSION"
$DOCKER service update --detach=false --with-registry-auth --image "$image" "${STACK}_core-api" >/dev/null \
  || { cleanup_canary; die "update of core-api failed after a good canary (Swarm rolled back)"; }
cleanup_canary
audit_line canary_completed service=core-api tag="$VERSION"
