#!/usr/bin/env bash
# F001 T044 — F001-AC-18: `up-app EXCEPT=core-api` starts only realtime + worker;
# F001-AC-19: /version of the three binaries share version and commit; worker reports its profile;
# F001-AC-06: APISIX has no route to the admin endpoints.
# Requires `make up` and built images (make up-app once).
set -euo pipefail
cd "$(dirname "$0")/../.."
DC=(docker compose -f deploy/compose/docker-compose.yml -f deploy/compose/docker-compose.app.yml)

http() { node -e 'fetch(process.argv[1]).then(async r=>{process.stdout.write(r.status+" "+(await r.text()))}).catch(e=>{process.stdout.write("000 "+e.message)})' "$1"; }

# 1) EXCEPT=core-api → only realtime and worker run in containers (same filter as the Makefile).
"${DC[@]}" --profile app stop core-api >/dev/null 2>&1 || true
"${DC[@]}" --profile app rm -f core-api >/dev/null 2>&1 || true
services=$(printf '%s\n' core-api realtime worker | grep -vx core-api | tr '\n' ' ')
# shellcheck disable=SC2086
"${DC[@]}" --profile app up -d --wait $services >/dev/null
running=$("${DC[@]}" --profile app ps --services --status running)
echo "$running" | grep -qx realtime || { echo "realtime not running" >&2; exit 1; }
echo "$running" | grep -qx worker || { echo "worker not running" >&2; exit 1; }
if echo "$running" | grep -qx core-api; then echo "core-api must not run with EXCEPT=core-api" >&2; exit 1; fi
echo "ok   - EXCEPT=core-api runs realtime + worker only"

# 2) bring core-api back and compare /version of the three binaries
"${DC[@]}" --profile app up -d --wait core-api >/dev/null
v_core=$(http http://localhost:9100/version); v_rt=$(http http://localhost:9106/version); v_wk=$(http http://localhost:9200/version)
field() { node -e 'const j=JSON.parse(process.argv[1].replace(/^\d+ /,""));process.stdout.write(String(j[process.argv[2]]??""))' "$1" "$2"; }
for f in version commit; do
  a=$(field "$v_core" $f); b=$(field "$v_rt" $f); c=$(field "$v_wk" $f)
  [ -n "$a" ] && [ "$a" = "$b" ] && [ "$b" = "$c" ] || { echo "$f differs: core-api=$a realtime=$b worker=$c" >&2; exit 1; }
done
[ "$(field "$v_core" binary)" = core-api ] && [ "$(field "$v_rt" binary)" = realtime ] && [ "$(field "$v_wk" binary)" = worker ] \
  || { echo "binary names wrong" >&2; exit 1; }
[ "$(field "$v_wk" profile)" = all ] || { echo "worker profile missing" >&2; exit 1; }
echo "ok   - /version: version=$(field "$v_core" version) commit=$(field "$v_core" commit), worker profile=all"

# 3) readiness of the three binaries
for p in 9100 9106 9200; do
  [ "$(http http://localhost:$p/readyz | cut -d' ' -f1)" = 200 ] || { echo "readyz on $p not 200" >&2; exit 1; }
done
echo "ok   - /readyz 200 on 9100, 9106, 9200"

# 4) admin endpoints are not reachable through APISIX
[ "$(http http://localhost:9080/readyz | cut -d' ' -f1)" = 404 ] || { echo "APISIX exposes /readyz" >&2; exit 1; }
echo "ok   - APISIX :9080/readyz → 404"
