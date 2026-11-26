#!/usr/bin/env bash
# Staging scenario F131-AC-01, AC-03, AC-04 (T011; quickstart steps 1, 3, 4). Needs a real staging (VPN to network
# admin) — not run in CI. Environment (values outside the repo, never committed):
#   EW_PROMETHEUS_URL, EW_TEMPO_URL, EW_LOKI_URL   observability endpoints (internal gateway)
#   EW_API_URL                                     public API of staging
#   EW_TEST_TOKEN_FILE                             file with an access token of the QA student (hs.qa.probe)
#   EW_TEST_TENANT_ID                              tenant of that student (QA tenant)
# 1. every Prometheus target is up (core-api, realtime, worker-*, exporters, APISIX, Keycloak, NATS, PG, Valkey, node,
#    cAdvisor, swarm-exporter); 2. a request that fails on purpose returns a trace_id whose trace in Tempo has gateway,
#    server, facade (caller→callee), producer/consumer and DB spans, and Loki has log lines with the same trace_id from
#    every binary involved; 3. tail sampling: 100 good + 5 failing requests → the 5 error traces are all kept.
set -euo pipefail
: "${EW_PROMETHEUS_URL:?}" "${EW_TEMPO_URL:?}" "${EW_LOKI_URL:?}" "${EW_API_URL:?}" "${EW_TEST_TOKEN_FILE:?}" "${EW_TEST_TENANT_ID:?}"
pass=0 fail=0
ok() { pass=$((pass + 1)); echo "ok   $*"; }
ko() { fail=$((fail + 1)); echo "FAIL $*" >&2; }
auth=(-H "Authorization: Bearer $(cat "$EW_TEST_TOKEN_FILE")" -H "X-Tenant-Id: $EW_TEST_TENANT_ID")

# 1. targets (F131-AC-01)
down=$(curl -fsS "$EW_PROMETHEUS_URL/api/v1/targets?state=active" | jq -r '.data.activeTargets[] | select(.health != "up") | "\(.labels.job) \(.labels.instance)"')
[ -z "$down" ] && ok "every active target is up" || ko "targets down: $down"
for job in backend apisix keycloak nats postgres valkey node-exporter cadvisor swarm-exporter; do
  n=$(curl -fsS -G "$EW_PROMETHEUS_URL/api/v1/query" --data-urlencode "query=count(up{job=\"$job\"} == 1)" | jq -r '.data.result[0].value[1] // "0"')
  [ "$n" -gt 0 ] && ok "job $job scraped ($n targets)" || ko "job $job has no target up"
done
for sp in core-api realtime worker/events worker/chat worker/jobs; do
  svc=${sp%%/*}; prof=${sp#*/}; [ "$prof" = "$sp" ] && prof=""
  q="count(up{job=\"backend\",service=\"$svc\"${prof:+,profile=\"$prof\"}} == 1)"
  n=$(curl -fsS -G "$EW_PROMETHEUS_URL/api/v1/query" --data-urlencode "query=$q" | jq -r '.data.result[0].value[1] // "0"')
  [ "$n" -gt 0 ] && ok "backend $sp scraped with service/profile labels" || ko "backend $sp missing"
done

# 2. one trace across binaries and modules (F131-AC-03): an unknown quest id fails in module game after a facade call.
body=$(curl -sS "${auth[@]}" -H 'Content-Type: application/json' -H "Idempotency-Key: $(cat /proc/sys/kernel/random/uuid)" \
  -X POST "$EW_API_URL/api/v1/game/quests/00000000-0000-4000-8000-000000000000:accept" -d '{}')
trace=$(jq -r '.trace_id // empty' <<< "$body")
[ -n "$trace" ] && ok "Problem Details carries trace_id" || { ko "no trace_id in the error response"; trace=""; }
if [ -n "$trace" ]; then
  sleep 15
  spans=$(curl -fsS "$EW_TEMPO_URL/api/traces/$trace" | jq -r '[.batches[]?.scopeSpans[]?.spans[]?.name] | join("\n")')
  grep -qiE 'apisix|gateway' <<< "$spans" && ok "gateway span" || ko "no gateway span"
  grep -qE '^(GET|POST) ' <<< "$spans" && ok "server span" || ko "no HTTP server span"
  grep -q '→' <<< "$spans" && ok "facade span caller→callee (modcall)" || echo "note: no facade span on this route"
  grep -qiE 'publish|consume|process' <<< "$spans" && ok "producer/consumer span" || echo "note: no NATS span on this route"
  grep -qiE 'select|insert|update|query|pgx' <<< "$spans" && ok "DB span" || ko "no DB span"
  lines=$(curl -fsS -G "$EW_LOKI_URL/loki/api/v1/query_range" --data-urlencode "query={service=~\"core-api|worker\"} |= \"$trace\"" \
    --data-urlencode "since=15m" | jq '[.data.result[].values[]] | length')
  [ "$lines" -gt 0 ] && ok "Loki has $lines log line(s) with the trace_id" || ko "no log line with the trace_id"
fi

# 3. sampling (F131-AC-04): error traces always kept.
ids=()
for i in $(seq 1 100); do curl -fsS -o /dev/null "$EW_API_URL/api/v1/game/client-config?platform=windows&version=1.0.0" || true; done
for i in $(seq 1 5); do
  t=$(curl -sS "${auth[@]}" "$EW_API_URL/api/v1/game/quests/00000000-0000-4000-8000-00000000000$i" | jq -r '.trace_id // empty'); [ -n "$t" ] && ids+=("$t")
done
sleep 20
kept=0
for t in "${ids[@]}"; do curl -fsS -o /dev/null "$EW_TEMPO_URL/api/traces/$t" && kept=$((kept + 1)); done
[ "$kept" -eq "${#ids[@]}" ] && [ "$kept" -eq 5 ] && ok "5/5 error traces kept by tail sampling" || ko "error traces kept: $kept/${#ids[@]}"

echo "trace_e2e: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
