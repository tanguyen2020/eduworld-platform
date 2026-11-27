#!/usr/bin/env bash
# k6 lint + smoke of the F132 load scripts (T010, T012–T014) against the TEST STAND-IN tests/load/stub/stub-server.mjs
# (AGENTS R17: proves the scripts, thresholds and report pipeline work; never capacity evidence). Everything runs in
# throwaway containers named ew-f132-load-* on network ew-f132-load-net, removed at the end.
#   1. k6 inspect every script of tests/load/k6 (init context + options must evaluate, EW_ENV guard included);
#   2. EW_ENV=production must be refused by a script;
#   3. baseline_api against STUB_MODE=ok → exit 0 (thresholds pass);
#   4. baseline_api against STUB_MODE=slow → exit 99 (read p95 > NFR-PERF-001: thresholds must FAIL);
#   5. login_overload against STUB_MODE=queue → exit 0 (429 LOGIN_QUEUE_BUSY seen, Retry-After honoured, nobody gives up).
# Usage: bash scripts/perf/k6-smoke.sh [--lint-only]     (K6_IMAGE, NODE_IMAGE override the images)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HOST_ROOT="$(cd "$ROOT" && (pwd -W 2>/dev/null || pwd))" # docker on Windows needs D:/… paths
export MSYS_NO_PATHCONV=1
K6_IMAGE="${K6_IMAGE:-grafana/k6:0.57.0}"
NODE_IMAGE="${NODE_IMAGE:-node:20-bookworm-slim}"
NET=ew-f132-load-net
STUB=ew-f132-load-stub
LINT_ONLY=0
[ "${1:-}" = "--lint-only" ] && LINT_ONLY=1
pass=0 fail=0

cleanup() {
  docker rm -f "$STUB" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
}
trap cleanup EXIT

ok() { echo "PASS $*"; pass=$((pass + 1)); }
ko() { echo "FAIL $*"; fail=$((fail + 1)); }

k6() { # extra docker args… -- k6 args…
  local dargs=()
  while [ "$1" != "--" ]; do dargs+=("$1"); shift; done
  shift
  docker run --rm --network "$NET" -v "$HOST_ROOT/tests/load:/load:ro" --cpus 1 "${dargs[@]}" "$K6_IMAGE" "$@"
}

common=(-e EW_ENV=local -e BASE_URL="http://$STUB:8080/api/v1" -e BOTS_FILE=/load/stub/bots.sample.json
  -e STAFF_FILE=/load/stub/staff.sample.json -e CHANNELS_FILE=/load/stub/channels.sample.json
  -e SCALE=0.0005 -e DURATION_SCALE=0.005 -e THINK_SCALE=0.01 -e RUN_ID=smoke)

docker network create "$NET" >/dev/null

for s in "$ROOT"/tests/load/k6/*.js; do
  name="$(basename "$s")"
  if k6 -- inspect "${common[@]}" "/load/k6/$name" >/dev/null 2>&1; then ok "inspect $name"; else ko "inspect $name"; k6 -- inspect "${common[@]}" "/load/k6/$name" 2>&1 | tail -5; fi
done
if k6 -- inspect -e EW_ENV=production -e BOTS_FILE=/load/stub/bots.sample.json /load/k6/baseline_api.js >/dev/null 2>&1; then
  ko "EW_ENV=production accepted"
else
  ok "EW_ENV=production refused (F132-AC-07)"
fi

if [ "$LINT_ONLY" = 0 ]; then
  start_stub() {
    docker rm -f "$STUB" >/dev/null 2>&1 || true
    docker run -d --name "$STUB" --network "$NET" --cpus 0.5 -e "STUB_MODE=$1" -v "$HOST_ROOT/tests/load/stub:/stub:ro" "$NODE_IMAGE" node /stub/stub-server.mjs >/dev/null
    sleep 2
  }
  run_case() { # mode script want_exit label
    start_stub "$1"
    set +e
    k6 -e EW_LOADBOT_PASSWORD=stub-only -- run --quiet "${common[@]}" "/load/k6/$2" > /dev/null 2>&1
    local code=$?
    set -e
    if [ "$code" = "$3" ]; then ok "$4 (exit $code)"; else ko "$4: exit $code, want $3"; fi
  }
  run_case ok baseline_api.js 0 "baseline_api vs stub ok: thresholds pass"
  run_case slow baseline_api.js 99 "baseline_api vs stub slow: thresholds fail (T010)"
  run_case queue login_overload.js 0 "login_overload vs stub queue: 429 + Retry-After honoured"
fi

echo "k6-smoke: $pass passed, $fail failed"
[ "$fail" = 0 ]
