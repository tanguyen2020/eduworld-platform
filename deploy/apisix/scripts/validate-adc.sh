#!/usr/bin/env bash
# F130 T052 — proves the ADC output and scripts/deploy/apisix-sync.sh against real gateways in throwaway containers:
# etcd (auth enabled, user apisix limited to the /apisix prefix) + the two APISIX 3.13 gateways in traditional mode
# with the Swarm config (deploy/swarm/config/apisix.yaml: public prefix /apisix, internal /apisix-internal, admin and
# viewer keys, custom plugins), then: adc lint, diff with the viewer key, sync refused with the viewer key, sync with
# the admin key, route sets isolated per gateway, routing on the data plane, idempotent second sync, canary
# traffic-split selected by label and sync refused while it is active. TEST VALUES only (random keys, the staging
# routes env template). Stand-ins: one etcd node for the 3-node cluster (aliases etcd-apisix-1..3).
#   bash deploy/apisix/scripts/validate-adc.sh     (docker, node; EW_VALIDATE_PREFIX names the containers)
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$here/../../.." && pwd)
P="${EW_VALIDATE_PREFIX:-ew-adc-validate}"
APISIX_IMAGE="${APISIX_IMAGE:-apache/apisix:3.13.0-debian}"
ETCD_IMAGE="${ETCD_IMAGE:-registry.k8s.io/etcd:3.5.12-0}"
CURL_IMAGE="${CURL_IMAGE:-natsio/nats-box:0.16.0}"
# shellcheck disable=SC1091
. "$REPO_ROOT/deploy/apisix/adc.env"
work=$(mktemp -d); chmod 700 "$work"
host_path() { if command -v cygpath >/dev/null; then cygpath -w "$1"; else printf '%s' "$1"; fi; }
D() { MSYS_NO_PATHCONV=1 env -u DOCKER_HOST -u DOCKER_TLS_VERIFY -u DOCKER_CERT_PATH docker "$@"; }
cleanup() {
  D rm -f "$P-etcd" "$P-apisix" "$P-apisix-internal" >/dev/null 2>&1 || true
  D network rm "$P-net" >/dev/null 2>&1 || true
  rm -rf "${work:?}"
}
trap cleanup EXIT
fail=0
ok() { echo "ok   $*"; }
bad() { echo "FAIL $*"; fail=1; }
rnd() { node -e 'process.stdout.write(require("crypto").randomBytes(18).toString("hex"))'; }
ADMIN_KEY=$(rnd); VIEWER_KEY=$(rnd); ETCD_PW=$(rnd); ROOT_PW=$(rnd)
printf '%s' "$ADMIN_KEY" > "$work/admin.key"; printf '%s' "$VIEWER_KEY" > "$work/viewer.key"

D network create "$P-net" >/dev/null
D run -d --name "$P-etcd" --network "$P-net" --network-alias etcd-apisix-1 --network-alias etcd-apisix-2 --network-alias etcd-apisix-3 \
  "$ETCD_IMAGE" etcd --name etcd-apisix-1 --data-dir /tmp/etcd --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://etcd-apisix-1:2379 >/dev/null
for _ in $(seq 1 30); do D exec "$P-etcd" etcdctl endpoint health >/dev/null 2>&1 && break; sleep 1; done
E() { D exec "$P-etcd" etcdctl "$@" >/dev/null; }
E user add root --new-user-password="$ROOT_PW"; E user grant-role root root
E role add apisix; E role grant-permission apisix --prefix=true readwrite /apisix
E user add apisix --new-user-password="$ETCD_PW"; E user grant-role apisix apisix
E auth enable && ok "etcd: auth enabled, user apisix limited to prefix /apisix (covers /apisix-internal)"

gateway() { # name prefix
  local extra=()
  [ "$2" = /apisix ] || extra=(-e "APISIX_ETCD_PREFIX=$2")
  D run -d --name "$1" --network "$P-net" "${extra[@]}" \
    -e APISIX_ADMIN_KEY="$ADMIN_KEY" -e APISIX_VIEWER_KEY="$VIEWER_KEY" -e APISIX_ETCD_PASSWORD="$ETCD_PW" \
    -e EW_APISIX_LOGIN_KEY_SECRET=validate-only \
    -v "$(host_path "$REPO_ROOT/deploy/swarm/config/apisix.yaml"):/usr/local/apisix/conf/config.yaml:ro" \
    -v "$(host_path "$REPO_ROOT/deploy/apisix/plugins"):/usr/local/apisix/custom/apisix/plugins:ro" \
    "$APISIX_IMAGE" >/dev/null
}
gateway "$P-apisix" /apisix
gateway "$P-apisix-internal" /apisix-internal
# The APISIX image has no curl: a curl client container (nats-box) on the test network, which also matches allow_admin
# of the Swarm config (private ranges; 127.0.0.1 is not allowed).
C() { D run --rm -i --network "$P-net" --entrypoint curl "$CURL_IMAGE" "$@"; }
admin_get() { C -fsS -K - "http://$1:9180$2" <<< "header = \"X-API-KEY: $ADMIN_KEY\""; }
for gw in "$P-apisix" "$P-apisix-internal"; do
  up=false
  for _ in $(seq 1 60); do admin_get "$gw" /apisix/admin/routes >/dev/null 2>&1 && { up=true; break; }; sleep 1; done
  if $up; then ok "$gw: traditional mode up (etcd auth, admin key)"; else bad "$gw: did not start"; D logs "$gw" 2>&1 | tail -30; exit 1; fi
done
# Healthcheck of the Swarm stacks, run in the image as Swarm runs it (CMD form).
mapfile -t hc < <(node -e 'const y=require(process.argv[2]);const t=y.load(require("fs").readFileSync(process.argv[1],"utf8")).services.apisix.healthcheck.test;
  if(t[0]!=="CMD")throw new Error("CMD form expected");for(const a of t.slice(1))console.log(JSON.stringify(a))' \
  "$REPO_ROOT/deploy/swarm/production/infra.stack.yml" "$REPO_ROOT/scripts/node_modules/js-yaml" | tr -d '\r')
hc_args=(); for a in "${hc[@]}"; do hc_args+=("$(node -e 'process.stdout.write(JSON.parse(process.argv[1]))' "$a")"); done
if D exec "$P-apisix" "${hc_args[@]}"; then ok "Swarm healthcheck of apisix passes in the image"; else bad "Swarm healthcheck of apisix fails in the image"; fi
if D logs "$P-apisix" 2>&1 | grep -E 'failed to load plugin|failed to check' ; then bad "public gateway: plugin load errors"; else ok "custom plugins loaded (ew-tenant-header, ew-api-rate, ew-login-key)"; fi

# apisix-sync.sh against the throwaway gateways: ADC and curl containers on the test network.
cat > "$work/curl" <<EOF
#!/usr/bin/env bash
exec env -u DOCKER_HOST MSYS_NO_PATHCONV=1 docker run --rm -i --network $P-net --entrypoint curl $CURL_IMAGE "\$@"
EOF
chmod +x "$work/curl"
sync() { # mode keyfile-var keyfile
  env EW_APISIX_ROUTES_ENV_FILE="$REPO_ROOT/deploy/apisix/routes/env/staging.env.example" \
    EW_APISIX_ADMIN_URL="http://$P-apisix:9180" EW_APISIX_INTERNAL_ADMIN_URL="http://$P-apisix-internal:9180" \
    EW_ADC_DOCKER_NETWORK="$P-net" CURL_BIN="$work/curl" "$2=$3" EW_AUDIT_FILE=/dev/null \
    bash "$REPO_ROOT/scripts/deploy/apisix-sync.sh" --env=staging --mode="$1" > "$work/out" 2>&1
}
if sync diff EW_APISIX_VIEWER_KEY_FILE "$work/viewer.key"; then ok "apisix-sync --mode=diff with the viewer key (adc lint + diff, both gateways)"; else bad "diff with the viewer key"; tail -20 "$work/out"; fi
if sync sync EW_APISIX_ADMIN_KEY_FILE "$work/viewer.key"; then bad "sync with the viewer key must fail"; else ok "viewer key cannot sync (role viewer is read-only)"; fi
if sync sync EW_APISIX_ADMIN_KEY_FILE "$work/admin.key"; then ok "apisix-sync --mode=sync with the admin key"; else bad "sync with the admin key"; tail -30 "$work/out"; fi

count() { node -e 'let s="";process.stdin.on("data",(c)=>s+=c).on("end",()=>{const l=(JSON.parse(s).list??[]).map((r)=>r.value);
  const f=process.argv[1]; process.stdout.write(String(f==="names"?l.map((r)=>r.name).sort().join(","):l.filter((r)=>r.labels?.["ew-upstream"]===f).length))})' "$1"; }
pub_names=$(admin_get "$P-apisix" /apisix/admin/routes | count names)
int_names=$(admin_get "$P-apisix-internal" /apisix/admin/routes | count names)
node "$REPO_ROOT/deploy/apisix/scripts/render.mjs" --format=adc --env=staging.env.example --out="$work/adc" >/dev/null
exp_names() { node -e 'const y=require(process.argv[2]);const d=y.load(require("fs").readFileSync(process.argv[1],"utf8"));
  process.stdout.write(d.services.flatMap((s)=>s.routes.map((r)=>r.name)).sort().join(","))' "$1" "$REPO_ROOT/scripts/node_modules/js-yaml"; }
[ "$pub_names" = "$(exp_names "$work/adc/public.adc.yaml")" ] && ok "public gateway holds exactly the public ADC routes ($(tr ',' '\n' <<< "$pub_names" | wc -l))" || bad "public route set differs: $pub_names"
[ "$int_names" = "$(exp_names "$work/adc/internal.adc.yaml")" ] && ok "internal gateway holds exactly the internal ADC routes ($(tr ',' '\n' <<< "$int_names" | wc -l)), none of the public" || bad "internal route set differs: $int_names"

probe() { C -s -o /dev/null -w '%{http_code}' "http://$1:9080$2" </dev/null || true; }
body() { C -s "http://$1:9080$2" </dev/null || true; }
sleep 2
grep -q 'Route Not Found' <<< "$(body "$P-apisix" /api/v1/admin/tenants)" && ok "public data plane: /api/v1/admin/* not routed" || bad "public gateway routes /api/v1/admin/*"
c=$(probe "$P-apisix" /api/v1/game/profile); [[ "$c" =~ ^40[01]$ ]] && ok "public data plane: /api/v1/game/profile routed and protected ($c: bearer/tenant header required)" || bad "public /api/v1/game/profile -> $c (want 400/401)"
grep -q 'Route Not Found' <<< "$(body "$P-apisix-internal" /api/v1/game/profile)" && ok "internal data plane: player routes not routed" || bad "internal gateway routes player routes"
grep -q 'Route Not Found' <<< "$(body "$P-apisix-internal" /api/v1/admin/tenants)" && bad "internal gateway lacks /api/v1/admin/*" || ok "internal data plane: /api/v1/admin/* routed (ip-restriction answers)"

if sync sync EW_APISIX_ADMIN_KEY_FILE "$work/admin.key"; then ok "second sync (idempotent)"; else bad "second sync"; tail -20 "$work/out"; fi
labelled=$(admin_get "$P-apisix" /apisix/admin/routes | count core-api)
[ "$labelled" -gt 0 ] && ok "canary selection: $labelled public routes labelled ew-upstream=core-api" || bad "no route labelled ew-upstream=core-api"

# Canary interplay: same Admin API calls as scripts/deploy/canary.sh, then sync must refuse.
admin_send() { C -fsS -K - -X "$2" "http://$1:9180$3" -H 'Content-Type: application/json' -d "$4" <<< "header = \"X-API-KEY: $ADMIN_KEY\"" >/dev/null; }
admin_send "$P-apisix" PUT /apisix/admin/upstreams/core-api-canary '{"type":"roundrobin","nodes":{"core-api-canary:8100":1}}'
rid=$(admin_get "$P-apisix" /apisix/admin/routes | node -e 'let s="";process.stdin.on("data",(c)=>s+=c).on("end",()=>{const r=(JSON.parse(s).list??[]).map((x)=>x.value).find((v)=>v.labels?.["ew-upstream"]==="core-api");process.stdout.write(r.id)})')
admin_send "$P-apisix" PATCH "/apisix/admin/routes/$rid" '{"plugins":{"traffic-split":{"rules":[{"weighted_upstreams":[{"upstream_id":"core-api-canary","weight":10},{"weight":90}]}]}}}' \
  && ok "canary traffic-split applied on an ADC route (service_id route, label selection)" || bad "traffic-split PATCH on an ADC route"
if sync sync EW_APISIX_ADMIN_KEY_FILE "$work/admin.key"; then bad "sync must refuse while a canary split is active"
elif grep -q 'canary is in progress' "$work/out"; then ok "sync refused while the canary traffic-split is active"; else bad "sync failed for another reason"; tail -10 "$work/out"; fi
admin_send "$P-apisix" PATCH "/apisix/admin/routes/$rid" '{"plugins":{"traffic-split":null}}'
if sync sync EW_APISIX_ADMIN_KEY_FILE "$work/admin.key"; then ok "sync after the canary removed its split"; else bad "sync after the canary"; tail -10 "$work/out"; fi
exit "$fail"
