#!/usr/bin/env bash
# CI job apisix:sync (F130 T052; DEPLOYMENT §5, §11.2): renders the ADC configuration of both gateways from
# deploy/apisix/routes/routes.yaml + the routes env of the environment, lints it, shows the diff against the running
# gateway and (mode sync) applies it with `adc sync` through the Admin API of each gateway (public: its own etcd
# prefix /apisix; internal: /apisix-internal). ADC owns every route/service/plugin metadata of a gateway (no label
# selector): a route created by hand is removed by the next sync.
#   apisix-sync.sh --env=<staging|production> --mode=diff|sync [--gateway=public|internal|all]
# Environment (GitHub Environment vars/secrets of the job; never in Git):
#   EW_APISIX_ROUTES_ENV_FILE  routes env file (the job writes vars.APISIX_ROUTES_ENV there; template
#                              deploy/apisix/routes/env/staging.env.example)
#   EW_APISIX_ADMIN_URL        Admin API of the public gateway (http://apisix:9180 on the admin network)
#   EW_APISIX_INTERNAL_ADMIN_URL  Admin API of apisix-internal
#   EW_APISIX_ADMIN_KEY_FILE   admin key file (mode sync; also accepted for diff)
#   EW_APISIX_VIEWER_KEY_FILE  read-only key file (mode diff of pull requests: role viewer cannot write)
# Canary interplay: `adc sync` would revert the traffic-split that scripts/deploy/canary.sh patches onto the core-api
# routes, so mode sync refuses to run while any route of the public gateway carries traffic-split (a canary in
# progress, or one that failed to clean up: remove the split first). The release workflow syncs before the canary,
# in the same job, and every sync job shares the concurrency group deploy-<env> with the deploy jobs.
# ADC: `adc` on PATH at the version of deploy/apisix/adc.env, else the pinned image (local Docker engine; DOCKER_HOST,
# which points at the Swarm manager on the deploy runner, is ignored). The key reaches ADC only as ADC_TOKEN in the
# process environment (never argv, never the log).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=scripts/deploy/lib-deploy.sh
. "$here/lib-deploy.sh"

ENV="" MODE="" GATEWAY=all
for arg in "$@"; do
  case "$arg" in
    --env=*) ENV="${arg#*=}" ;;
    --mode=*) MODE="${arg#*=}" ;;
    --gateway=*) GATEWAY="${arg#*=}" ;;
    *) die "unknown argument $arg" ;;
  esac
done
check_env "$ENV"
[[ "$MODE" =~ ^(diff|sync)$ ]] || die "--mode=diff|sync required"
[[ "$GATEWAY" =~ ^(public|internal|all)$ ]] || die "--gateway=public|internal|all"
ROUTES_ENV="${EW_APISIX_ROUTES_ENV_FILE:-}"
[ -s "$ROUTES_ENV" ] || die "EW_APISIX_ROUTES_ENV_FILE: routes env of $ENV missing (GitHub Environment variable APISIX_ROUTES_ENV)"
if [ "$MODE" = sync ]; then KEY_FILE="${EW_APISIX_ADMIN_KEY_FILE:-}"; else KEY_FILE="${EW_APISIX_VIEWER_KEY_FILE:-${EW_APISIX_ADMIN_KEY_FILE:-}}"; fi
[ -s "$KEY_FILE" ] || die "Admin API key file missing or empty (sync: EW_APISIX_ADMIN_KEY_FILE; diff: EW_APISIX_VIEWER_KEY_FILE)"

# shellcheck disable=SC1091
. "$REPO_ROOT/deploy/apisix/adc.env"
work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
chmod 700 "$work"
node "$REPO_ROOT/deploy/apisix/scripts/render.mjs" --format=adc --env-file="$ROUTES_ENV" --out="$work" >&2 \
  || die "cannot render the ADC configuration"

ADC_TOKEN=$(cat "$KEY_FILE"); export ADC_TOKEN
if [ -n "${ADC_BIN:-}" ] || command -v adc >/dev/null 2>&1; then
  ADC=("${ADC_BIN:-adc}")
  have=$("${ADC[@]}" --version 2>/dev/null | tr -d '[:space:]v')
  [ "$have" = "$ADC_VERSION" ] || die "adc $have on the runner, deploy/apisix/adc.env pins $ADC_VERSION"
  FILE_DIR="$work"
else
  host_path() { if command -v cygpath >/dev/null; then cygpath -w "$1"; else printf '%s' "$1"; fi; }
  # EW_ADC_DOCKER_NETWORK: network of the ADC container (default host: the runner reaches the admin network).
  ADC=(env -u DOCKER_HOST -u DOCKER_TLS_VERIFY -u DOCKER_CERT_PATH MSYS_NO_PATHCONV=1 docker run --rm
    --network "${EW_ADC_DOCKER_NETWORK:-host}" -e ADC_TOKEN -v "$(host_path "$work"):/adc:ro" "$ADC_IMAGE")
  FILE_DIR=/adc
fi

canary_split_active() { # admin URL → 0 when a route carries traffic-split (key given to curl on stdin: -K -)
  local routes
  routes=$(printf 'header = "X-API-KEY: %s"\n' "$ADC_TOKEN" | "${CURL_BIN:-curl}" -fsS -K - "$1/apisix/admin/routes") \
    || die "cannot list the routes of $1"
  [ "$(node -e 'let s="";process.stdin.on("data",(c)=>s+=c).on("end",()=>{const l=JSON.parse(s).list??[];
    process.stdout.write(String(l.filter((r)=>r.value?.plugins?.["traffic-split"]).length))})' <<< "$routes")" -gt 0 ]
}

summary() { if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then printf '%s\n' "$@" >> "$GITHUB_STEP_SUMMARY"; fi; }
gateways=(public internal); [ "$GATEWAY" = all ] || gateways=("$GATEWAY")
for gw in "${gateways[@]}"; do
  if [ "$gw" = public ]; then url="${EW_APISIX_ADMIN_URL:?EW_APISIX_ADMIN_URL required}"; else url="${EW_APISIX_INTERNAL_ADMIN_URL:?EW_APISIX_INTERNAL_ADMIN_URL required}"; fi
  file="$FILE_DIR/$gw.adc.yaml"
  "${ADC[@]}" lint -f "$file" >&2 || die "adc lint failed for the $gw gateway"
  if [ "$MODE" = sync ] && [ "$gw" = public ] && canary_split_active "$url"; then
    die "a core-api canary is in progress on the public gateway (traffic-split on its routes): not syncing (rerun after the canary, or remove the split)"
  fi
  out=$("${ADC[@]}" diff --backend apisix --server "$url" --timeout 30s -f "$file" 2>&1) || { printf '%s\n' "$out" >&2; die "adc diff failed for the $gw gateway"; }
  printf '%s\n' "$out" >&2
  summary "### APISIX $gw gateway ($ENV): adc diff" '```' "$out" '```'
  if [ "$MODE" = sync ]; then
    "${ADC[@]}" sync --backend apisix --server "$url" --timeout 30s -f "$file" >&2 || die "adc sync failed for the $gw gateway"
    audit_line apisix_synced gateway="$gw" adc="$ADC_VERSION"
  fi
done
log info "apisix-sync $ENV: mode=$MODE gateways=${gateways[*]}"
