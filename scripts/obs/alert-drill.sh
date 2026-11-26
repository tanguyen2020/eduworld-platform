#!/usr/bin/env bash
# Alert drill (F131 T022, F131-AC-11; DEPLOYMENT §15): fires a synthetic copy of each critical alert of
# deploy/observability/prometheus/rules (or one --alert) on the Alertmanager of an environment, with the static labels
# of its rule (so team=safety goes to safety-oncall) plus drill="true", measures the time until Alertmanager reports a
# successful notification (alertmanager_notifications_total) and until the on-call person confirms receipt on the
# channel, resolves the alert, and appends one row per alert to docs/11-operations/alert-drills/<YYYY-MM>.md.
# Runs from the VPN (Alertmanager is only on network admin): EW_ALERTMANAGER_URL (e.g. the internal gateway route).
#   alert-drill.sh --env=staging [--alert=ServiceDown] [--yes]   (--yes: no interactive confirmation, delivery time only)
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
ENV="" ALERT="" YES=false
for arg in "$@"; do
  case "$arg" in
    --env=*) ENV="${arg#*=}" ;;
    --alert=*) ALERT="${arg#*=}" ;;
    --yes) YES=true ;;
    *) echo "unknown argument $arg" >&2; exit 2 ;;
  esac
done
[[ "$ENV" =~ ^(staging|production)$ ]] || { echo "--env=staging|production required" >&2; exit 2; }
AM="${EW_ALERTMANAGER_URL:?EW_ALERTMANAGER_URL required (Alertmanager of $ENV, reachable on the VPN)}"
CURL="${CURL_BIN:-curl}"
actor="${EW_ACTOR:-${USER:-unknown}}"
report="${EW_DRILL_REPORT:-$repo/docs/11-operations/alert-drills/$(date -u +%Y-%m).md}"

# Critical alerts with the static labels of their rule (JSON lines: {"alertname":…, …}).
alerts=$(cd "$repo" && node --input-type=module -e '
  const { loadRules } = await import("./scripts/obs/lib-monitoring-md.mjs");
  const only = process.argv[1];
  const seen = new Set();
  for (const { rule } of loadRules()) {
    if (!rule.alert || rule.labels?.severity !== "critical" || seen.has(rule.alert)) continue;
    if (only && rule.alert !== only) continue;
    seen.add(rule.alert);
    console.log(JSON.stringify({ alertname: rule.alert, ...rule.labels, drill: "true", service: rule.labels.service ?? "drill" }));
  }' "$ALERT")
[ -n "$alerts" ] || { echo "no critical alert matches ${ALERT:-*}" >&2; exit 1; }

sent_total() { "$CURL" -fsS "$AM/metrics" | awk '/^alertmanager_notifications_total\{/ { s += $NF } END { print s + 0 }'; }
post() { "$CURL" -fsS -XPOST -H 'Content-Type: application/json' "$AM/api/v2/alerts" -d "$1" >/dev/null; }

if [ ! -f "$report" ]; then
  mkdir -p "$(dirname "$report")"
  cat > "$report" <<EOF
# Diễn tập alert $(date -u +%Y-%m)

> Sinh bởi \`scripts/obs/alert-drill.sh\` (F131-AC-11). Mẫu và cách chạy: [README](README.md).

| Alert | Cách kích hoạt (synthetic / điều kiện thật) | Thời điểm kích hoạt (UTC) | Thời điểm nhận ở kênh trực | Độ trễ | Người nhận | Kết quả (đạt ≤ 2 phút?) | Vấn đề / việc tiếp theo |
|---|---|---|---|---|---|---|---|
EOF
fi

fail=0
while IFS= read -r labels; do
  name=$(node -e 'console.log(JSON.parse(process.argv[1]).alertname)' "$labels")
  before=$(sent_total)
  start=$(date -u +%s); started=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  post "[{\"labels\":$labels,\"annotations\":{\"summary\":\"DRILL — $name (alert-drill.sh, $ENV)\",\"runbook_url\":\"docs/11-operations/runbooks/$name.md\"},\"startsAt\":\"$started\"}]"
  delivered=""
  for _ in $(seq 1 60); do
    sleep 2
    if [ "$(sent_total)" -gt "$before" ]; then delivered=$(date -u +%s); break; fi
  done
  received="$delivered"
  if [ "$YES" != true ]; then
    read -r -p "[$name] Đã nhận trên kênh trực? Nhấn Enter khi thấy thông báo (hoặc gõ n nếu không nhận): " ans < /dev/tty
    [ "${ans:-}" = n ] && received="" || received=$(date -u +%s)
  fi
  post "[{\"labels\":$labels,\"startsAt\":\"$started\",\"endsAt\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}]"   # resolve
  if [ -n "$received" ]; then
    lag=$((received - start))
    ok=$([ "$lag" -le 120 ] && echo "đạt" || echo "KHÔNG đạt")
    [ "$lag" -le 120 ] || fail=1
    echo "| \`$name\` | synthetic (alert-drill.sh) | $started | $(date -u -d "@$received" +%H:%M:%SZ 2>/dev/null || echo "+${lag}s") | ${lag} s | $actor | $ok | |" >> "$report"
    echo "$name: ${lag} s ($ok)"
  else
    fail=1
    echo "| \`$name\` | synthetic (alert-drill.sh) | $started | không nhận | — | $actor | KHÔNG đạt | kiểm tra receiver |" >> "$report"
    echo "$name: not received" >&2
  fi
done <<< "$alerts"
echo "report: $report"
exit "$fail"
