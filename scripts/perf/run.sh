#!/usr/bin/env bash
# Runs one load scenario of tests/load/scenarios.yaml and renders its perf report (F132 US1, T015, F132-AC-01…07).
#
#   scripts/perf/run.sh --run <name> [--env local|dev|staging] [--scale <f>] [--duration-scale <f>]
#                       [--scale-service <svc>=<replicas>@<offset_s>] [--id-suffix <s>] [--no-report]
#
# Steps: refuse any EW_ENV outside local|dev|staging (F132-AC-07) → on staging, TC-PERF-002/014 require every service
# at `peak` (warmup-check.mjs, TC-PERF-017) → start the load-generator CPU sampler → run k6 and/or loadbot (both for
# runs whose tool is loadbot with a k6 script, e.g. ccu_10k) tagged testid=<run_id> → optionally scale one Swarm
# service mid-run (F132-AC-05) → Prometheus checks (prom-checks.mjs) → generator saturation → meta.json (backend tag,
# image digests, sizing) → render-report.mjs. Staging reports go to docs/12-project/perf-reports/<date>-<run>.md;
# local/dev smoke reports stay in tmp/perf/<run_id>/report.md (never presented as capacity evidence).
#
# Environment (secrets come from GitHub Environment `staging`, never from the repository):
#   BASE_URL                public API base (…/api/v1)          EW_LOADBOT_PASSWORD  bot password (datagen accounts)
#   BOTS_FILE               bots.json of datagen                 EW_PROMETHEUS_URL    staging Prometheus (checks, warm-up)
#   K6_PROMETHEUS_RW_SERVER_URL  remote write of k6 metrics (F131 panels by testid)
#   EW_SWARM_STACK          stack name for image digests / scaling (docker CLI pointed at the manager via DOCKER_HOST)
#   EW_BACKEND_TAG          backend/vX.Y.Z under test            EW_SIZING_FILE       JSON of the node sizing of the run
#   K6_BIN / K6_IMAGE       k6 binary, else docker image (default grafana/k6:0.57.0)
#   LOADBOT_BIN             loadbot binary, else `go run ./tools/loadbot/cmd/loadbot`
#   EXTRA_K6_ARGS / EXTRA_LOADBOT_ARGS  passed through (e.g. -e ACTIVITY_ID=…, -activity-id …)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GEN="$ROOT/tests/load/k6/scenarios.gen.json"
RUN="" ENVIRONMENT="${EW_ENV:-}" SCALE="1" DSCALE="1" SCALE_SERVICE="" SUFFIX="" REPORT=1

while [ $# -gt 0 ]; do
  case "$1" in
    --run) RUN="$2"; shift 2 ;;
    --env) ENVIRONMENT="$2"; shift 2 ;;
    --scale) SCALE="$2"; shift 2 ;;
    --duration-scale) DSCALE="$2"; shift 2 ;;
    --scale-service) SCALE_SERVICE="$2"; shift 2 ;;
    --id-suffix) SUFFIX="-$2"; shift 2 ;;
    --no-report) REPORT=0; shift ;;
    -h|--help) sed -n '2,24p' "$0"; exit 0 ;;
    *) echo "run.sh: unknown argument $1" >&2; exit 2 ;;
  esac
done

# F132-AC-07: same rule as src/backend/tools/envguard.
ENVIRONMENT="$(printf '%s' "$ENVIRONMENT" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
case "$ENVIRONMENT" in
  local|dev|staging) ;;
  *) echo "run.sh: refused: EW_ENV='${ENVIRONMENT}' is not a test environment (local, dev, staging)" >&2; exit 3 ;;
esac
export EW_ENV="$ENVIRONMENT"
[ -n "$RUN" ] || { echo "run.sh: --run <name> is required" >&2; exit 2; }

field() { # run field of scenarios.gen.json
  node -e 'const g=require(process.argv[1]);const r=g.runs[process.argv[2]];if(!r){process.exit(4)}const v=r[process.argv[3]];process.stdout.write(v==null?"":String(v))' "$GEN" "$RUN" "$1"
}
TOOL="$(field tool)" || { echo "run.sh: run '$RUN' is not in tests/load/scenarios.yaml" >&2; exit 2; }
TC="$(field id)"
SCRIPT="$(field script)"
ARGS="$(field args)"
case "$TOOL" in
  k6|loadbot) ;;
  *) echo "run.sh: $TC ($RUN) uses tool '$TOOL': run it with its own runner (see tests/load/scenarios.yaml)" >&2; exit 2 ;;
esac

RUN_ID="$(date -u +%Y-%m-%d)-${RUN}${SUFFIX}"
DIR="$ROOT/tmp/perf/$RUN_ID"
mkdir -p "$DIR"
: "${BASE_URL:?BASE_URL is required}"
: "${BOTS_FILE:?BOTS_FILE (bots.json of datagen) is required}"
echo "run.sh: $TC $RUN on $EW_ENV, run_id $RUN_ID"

WARMUP="null"
if [ "$EW_ENV" = "staging" ] && { [ "$TC" = "TC-PERF-002" ] || [ "$TC" = "TC-PERF-014" ]; }; then
  : "${EW_PROMETHEUS_URL:?EW_PROMETHEUS_URL is required on staging}"
  if ! node "$ROOT/scripts/perf/warmup-check.mjs" --env=staging --prom="$EW_PROMETHEUS_URL" --out="$DIR/warmup.json"; then
    echo "run.sh: ops-scheduler is not at peak (TC-PERF-017); the run would be invalid — wait for the warm-up window" >&2
    exit 1
  fi
  WARMUP="$(cat "$DIR/warmup.json")"
fi

bash "$ROOT/scripts/perf/check-generator-saturation.sh" start "$DIR" 5
START="$(date -u +%s)"
STARTED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
pids=()

hostpath() { (cd "$1" && (pwd -W 2>/dev/null || pwd)); }

run_k6() {
  local script="$1" abs
  abs="$ROOT/$script"
  local envs=(-e "EW_ENV=$EW_ENV" -e "BASE_URL=$BASE_URL" -e "RUN_ID=$RUN_ID" -e "SCALE=$SCALE" -e "DURATION_SCALE=$DSCALE")
  local out=()
  if [ -n "${K6_PROMETHEUS_RW_SERVER_URL:-}" ]; then out=(-o experimental-prometheus-rw); fi
  # shellcheck disable=SC2206 # EXTRA_K6_ARGS is a list of flags on purpose
  local extra=(${EXTRA_K6_ARGS:-})
  if [ -n "${K6_BIN:-}" ] || command -v k6 >/dev/null 2>&1; then
    # The password reaches k6 as a process environment variable (__ENV), never as an argument (ps would show it).
    "${K6_BIN:-k6}" run "${envs[@]}" -e "BOTS_FILE=$BOTS_FILE" \
      --tag "testid=$RUN_ID" --summary-export "$DIR/k6-summary.json" "${out[@]}" "${extra[@]}" "$abs"
  else
    # Docker on Windows needs D:/… host paths and no MSYS conversion of the container paths.
    MSYS_NO_PATHCONV=1 docker run --rm --name "ew-f132-k6-${RUN_ID//[^a-z0-9-]/}" --network "${K6_DOCKER_NETWORK:-bridge}" \
      -v "$(hostpath "$ROOT/tests/load"):/load:ro" -v "$(hostpath "$(dirname "$BOTS_FILE")"):/data:ro" -v "$(hostpath "$DIR"):/out" \
      -e EW_LOADBOT_PASSWORD -e K6_PROMETHEUS_RW_SERVER_URL "${K6_IMAGE:-grafana/k6:0.57.0}" run "${envs[@]}" \
      -e "BOTS_FILE=/data/$(basename "$BOTS_FILE")" \
      --tag "testid=$RUN_ID" --summary-export /out/k6-summary.json "${out[@]}" "${extra[@]}" "/load/${script#tests/load/}"
  fi
}

run_loadbot() {
  # shellcheck disable=SC2206 # ARGS/EXTRA_LOADBOT_ARGS are flag lists on purpose
  local args=($ARGS ${EXTRA_LOADBOT_ARGS:-})
  if [ -n "${LOADBOT_BIN:-}" ]; then
    "$LOADBOT_BIN" -base-url "$BASE_URL" -bots "$BOTS_FILE" -summary "$DIR/loadbot-summary.json" "${args[@]}"
  else
    (cd "$ROOT/src/backend" && go run ./tools/loadbot/cmd/loadbot -base-url "$BASE_URL" -bots "$BOTS_FILE" -summary "$DIR/loadbot-summary.json" "${args[@]}")
  fi
}

status=0
if [ "$TOOL" = "loadbot" ]; then run_loadbot & pids+=($!); fi
if [ -n "$SCRIPT" ] && [ "${SCRIPT##*.}" = "js" ]; then run_k6 "$SCRIPT" & pids+=($!); fi

SCALE_AT=""
if [ -n "$SCALE_SERVICE" ]; then # F132-AC-05: scale one service mid-run, the others are left alone
  [ "$EW_ENV" = "staging" ] || [ -n "${EW_SWARM_STACK:-}" ] || { echo "run.sh: --scale-service needs EW_SWARM_STACK (a Swarm)" >&2; exit 2; }
  svc="${SCALE_SERVICE%%=*}"; rest="${SCALE_SERVICE#*=}"; replicas="${rest%%@*}"; offset="${rest#*@}"
  sleep "$offset"
  SCALE_AT="$(date -u +%s)"
  docker service scale --detach "${EW_SWARM_STACK:?EW_SWARM_STACK}_${svc}=${replicas}"
fi

for p in "${pids[@]}"; do wait "$p" || status=$?; done
END="$(date -u +%s)"
ENDED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
bash "$ROOT/scripts/perf/check-generator-saturation.sh" stop "$DIR" > /dev/null
if [ -n "${EW_PROMETHEUS_URL:-}" ] && [ "$EW_ENV" = "staging" ]; then
  # Several runner VMs: their node_exporter (job=loadgen) gives the max over every generator.
  bash "$ROOT/scripts/perf/check-generator-saturation.sh" prom "$EW_PROMETHEUS_URL" "$START" "$END" "$DIR" > /dev/null || true
fi
if [ -n "${EW_PROMETHEUS_URL:-}" ]; then
  node "$ROOT/scripts/perf/prom-checks.mjs" --run="$RUN" --prom="$EW_PROMETHEUS_URL" --start="$START" --end="$END" \
    ${SCALE_AT:+--scale-at="$SCALE_AT"} --out="$DIR/prom.json" || true
fi

IMAGES="{}"
if [ -n "${EW_SWARM_STACK:-}" ] && command -v docker >/dev/null 2>&1; then
  IMAGES="$(docker service ls --filter "label=com.docker.stack.namespace=${EW_SWARM_STACK}" --format '{{.Name}} {{.Image}}' 2>/dev/null \
    | node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>{const o={};for(const l of s.split("\n").filter(Boolean)){const [n,i]=l.split(" ");o[n]=i}process.stdout.write(JSON.stringify(o))})' || echo '{}')"
fi
SIZING="{}"
if [ -n "${EW_SIZING_FILE:-}" ] && [ -f "$EW_SIZING_FILE" ]; then SIZING="$(cat "$EW_SIZING_FILE")"; fi
GRAFANA=""
if [ -n "${EW_GRAFANA_URL:-}" ]; then GRAFANA="${EW_GRAFANA_URL%/}/d/eduworld-load-test?var-testid=${RUN_ID}&from=${START}000&to=${END}000"; fi

RUN_ID="$RUN_ID" RUN="$RUN" TC="$TC" STARTED="$STARTED" ENDED="$ENDED" COMMIT="$(git -C "$ROOT" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)" \
TAG="${EW_BACKEND_TAG:-$(git -C "$ROOT" describe --tags --match 'backend/v*' --always 2>/dev/null || echo unknown)}" \
IMAGES="$IMAGES" SIZING="$SIZING" WARMUP="$WARMUP" GRAFANA="$GRAFANA" SCALE_SERVICE="$SCALE_SERVICE" \
node -e 'const e=process.env;require("fs").writeFileSync(process.argv[1],JSON.stringify({run_id:e.RUN_ID,run:e.RUN,tc:e.TC,env:e.EW_ENV,started:e.STARTED,ended:e.ENDED,
  commit:e.COMMIT,backend_tag:e.TAG,images:JSON.parse(e.IMAGES),sizing:JSON.parse(e.SIZING),warmup:JSON.parse(e.WARMUP),grafana_url:e.GRAFANA||undefined,
  scale:e.SCALE_SERVICE||undefined},null,2))' "$DIR/meta.json"

if [ "$REPORT" = 1 ]; then
  if [ "$EW_ENV" = "staging" ]; then OUT="$ROOT/docs/12-project/perf-reports/${RUN_ID}.md"; else OUT="$DIR/report.md"; fi
  node "$ROOT/scripts/perf/render-report.mjs" --dir="$DIR" --out="$OUT" || status=$?
fi
exit "$status"
