#!/usr/bin/env bash
# F004 T034 — boots each rendered gateway (APISIX standalone, same image/mounts as compose) without
# its dependencies and probes it: the configuration loads (custom plugins, shared dict, #END) and the
# route table is the expected one. Upstreams/Keycloak are absent, so only routing is asserted.
#   bash deploy/apisix/scripts/validate-standalone.sh
set -euo pipefail

# Docker bind mounts need a Windows path under Git Bash (pwd -W); Linux CI falls back to pwd.
ROOT="$(cd "$(dirname "$0")/.." && { pwd -W 2>/dev/null || pwd; })"
IMAGE="${APISIX_IMAGE:-apache/apisix:3.13.0-debian}"
fail=0

probe() { curl -s -o /dev/null -w '%{http_code}' -X "$2" "http://127.0.0.1:$1$3" || true; }
body() { curl -s -X "$2" "http://127.0.0.1:$1$3" || true; }

run_gateway() {
  local gw="$1" port="$2" name="ew-apisix-validate-$1"
  docker rm -f "$name" >/dev/null 2>&1 || true
  MSYS_NO_PATHCONV=1 docker run -d --name "$name" -p "127.0.0.1:${port}:9080" \
    -e APISIX_STAND_ALONE=true -e EW_APISIX_LOGIN_KEY_SECRET=validate-only \
    -v "$ROOT/config.yaml:/usr/local/apisix/conf/config.yaml:ro" \
    -v "$ROOT/standalone/$gw/apisix.yaml:/usr/local/apisix/conf/apisix.yaml:ro" \
    -v "$ROOT/plugins:/usr/local/apisix/custom/apisix/plugins:ro" \
    "$IMAGE" >/dev/null
  for _ in $(seq 1 60); do
    [ "$(probe "$port" GET /__ew_ready)" = "404" ] && return 0
    if [ "$(docker inspect -f '{{.State.Running}}' "$name")" != "true" ]; then break; fi
    sleep 1
  done
  echo "::error::$gw gateway did not start"; docker logs "$name" 2>&1 | tail -40; return 1
}

expect() { # gateway port method path predicate description
  local got; got="$(probe "$2" "$3" "$4")"
  if ! eval "[ \"$got\" $5 ]"; then echo "::error::$1 $3 $4 -> $got, want $5 ($6)"; fail=1; else echo "ok $1 $3 $4 -> $got"; fi
}
expect_no_route() {
  if body "$2" "$3" "$4" | grep -q 'Route Not Found'; then echo "ok $1 $3 $4 -> no route"; else echo "::error::$1 $3 $4 is routed ($5)"; fail=1; fi
}
expect_route() {
  if body "$2" "$3" "$4" | grep -q 'Route Not Found'; then echo "::error::$1 $3 $4 has no route ($5)"; fail=1; else echo "ok $1 $3 $4 -> routed"; fi
}

cleanup() { docker rm -f ew-apisix-validate-public ew-apisix-validate-internal >/dev/null 2>&1 || true; }
trap cleanup EXIT

run_gateway public 19080
run_gateway internal 19180

# Public: internal prefixes absent, player/login routes present, tenant header enforced before OIDC.
for p in /api/v1/admin/tenants /api/v1/sis/connections /api/v1/game-admin/quests /internal/v1/x /metrics; do
  expect_no_route public 19080 GET "$p" "internal prefix on the public gateway"
done
expect_route public 19080 GET /api/v1/game/client-config "public client config"
expect public 19080 GET /api/v1/game/profile "= 400" "missing X-Tenant-Id → 400 TENANT_HEADER_REQUIRED"
expect public 19080 POST /api/v1/auth/game/login "!= 404" "login route"
# F133: CORS preflight of the Web build (play origin of routes/env/local.env EW_CORS_GAME_ORIGINS) is answered on
# the game routes with the game headers, and never on the portal cookie route.
acao() { # port path origin [host] → Access-Control-Allow-Origin of a preflight
  curl -s -o /dev/null -D - -X OPTIONS "http://127.0.0.1:$1$2" -H "Origin: $3" ${4:+-H "Host: $4"} \
    -H 'Access-Control-Request-Method: POST' \
    -H 'Access-Control-Request-Headers: content-type,x-client-version,x-client-platform,x-device-id' \
    | tr -d '\r' | sed -n 's/^[Aa]ccess-[Cc]ontrol-[Aa]llow-[Oo]rigin: //p' || true
}
cors_expect() { # description got want
  if [ "$2" = "$3" ]; then echo "ok cors $1 -> '${2}'"; else echo "::error::cors $1 -> '${2}', want '${3}'"; fail=1; fi
}
play="$(sed -n 's/^EW_CORS_GAME_ORIGINS=//p' "$ROOT/routes/env/local.env" | cut -d, -f1)"
cors_expect "game login from the Web build" "$(acao 19080 /api/v1/auth/game/login "$play")" "$play"
cors_expect "client-config from the Web build" "$(acao 19080 /api/v1/game/client-config "$play")" "$play"
cors_expect "player route from the Web build" "$(acao 19080 /api/v1/learning/my-assignments "$play")" "$play"
cors_expect "portal login from the Web build" "$(acao 19080 /api/v1/auth/portal/login "$play" localhost)" ""
cors_expect "game login from an unknown origin" "$(acao 19080 /api/v1/auth/game/login https://evil.example)" ""
# Internal: platform prefixes present, player routes absent.
expect_route internal 19180 GET /api/v1/admin/tenants "admin on the internal gateway"
expect_no_route internal 19180 GET /api/v1/game/profile "player route on the internal gateway"
expect_no_route internal 19180 POST /api/v1/auth/game/login "login route on the internal gateway"

if docker logs ew-apisix-validate-public 2>&1 | grep -E 'failed to load plugin|failed to check|invalid configuration' ; then
  echo "::error::public gateway reported configuration errors"; fail=1
fi
exit "$fail"
