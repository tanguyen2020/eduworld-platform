#!/usr/bin/env bash
# Alertmanager routing tests (F131 T021, F131-AC-09, AC-10): render alertmanager.yml with sample environment values,
# `amtool check-config`, `amtool config routes test` for every route, then (--live) start a throwaway Alertmanager,
# post alerts through the API and check the inhibition rules (ServiceDown mutes the same service/profile; critical
# mutes its warning twin; Watchdog is never muted). Containers: ew-obs-am-* (removed on exit).
#   routes_test.sh [--live]
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
am_dir=$(cd "$here/.." && pwd)
image=${EW_ALERTMANAGER_IMAGE:-prom/alertmanager:v0.28.1}
live=0
[ "${1:-}" = "--live" ] && live=1
winpath() { if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else echo "$1"; fi; }
export MSYS_NO_PATHCONV=1

work=$(mktemp -d)
name="ew-obs-am-$$"
cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; rm -rf "$work"; }
trap cleanup EXIT
mkdir -p "$work/secrets" "$work/templates"
cp "$am_dir/templates/"*.tmpl "$work/templates/"
sed -e 's|__EW_ALERT_SMTP_SMARTHOST__|smtp.example.invalid:587|' -e 's|__EW_ALERT_SMTP_FROM__|alerts@example.invalid|' \
    -e 's|__EW_ALERT_SMTP_USERNAME__|alerts|' -e 's|__EW_ALERT_EMAIL_TO__|oncall@example.invalid|' \
    -e 's|/etc/alertmanager/templates/|/cfg/templates/|' "$am_dir/alertmanager.yml" > "$work/alertmanager.yml"
for s in heartbeat_url chat_webhook_url sms_webhook_url safety_webhook_url; do
  printf 'http://127.0.0.1:9/%s\n' "$s" > "$work/secrets/ops_alert_$s"   # MOCK endpoint: nothing listens
done
printf 'not-a-secret' > "$work/secrets/ops_alert_smtp_password"
chmod -R a+rX "$work"

amtool() { docker run --rm -v "$(winpath "$work"):/cfg:ro" --entrypoint amtool "$image" "$@"; }

pass=0 fail=0
ok() { pass=$((pass + 1)); echo "ok   $*"; }
ko() { fail=$((fail + 1)); echo "FAIL $*" >&2; }

if amtool check-config /cfg/alertmanager.yml >/dev/null; then ok "amtool check-config"; else ko "amtool check-config"; fi

# label set → expected receiver
while IFS='|' read -r labels want; do
  [ -z "$labels" ] && continue
  # shellcheck disable=SC2086
  got=$(amtool config routes test --config.file=/cfg/alertmanager.yml $labels 2>/dev/null | tr -d '\r' | tail -1)
  if [ "$got" = "$want" ]; then ok "route {$labels} → $want"; else ko "route {$labels} → got '$got', want '$want'"; fi
done <<'EOF'
alertname=Watchdog severity=warning heartbeat=true|heartbeat
alertname=ServiceDown severity=critical service=worker profile=chat|oncall-critical
alertname=ModuleHighErrorRate severity=critical service=core-api module=game|oncall-critical
alertname=APILatencyP95High severity=warning service=core-api module=game|ops-warning
alertname=ModerationP1Unclaimed severity=critical module=chat team=safety|safety-oncall
alertname=SchoolSafetyNotificationNoRecipient severity=warning module=identity team=safety|safety-oncall
alertname=SomethingUnlabelled|ops-warning
EOF

if [ "$live" = 1 ]; then
  port=${EW_AM_TEST_PORT:-19093}
  docker run -d --name "$name" -p "127.0.0.1:$port:9093" -v "$(winpath "$work"):/cfg:ro" -v "$(winpath "$work/secrets"):/run/secrets:ro" \
    "$image" --config.file=/cfg/alertmanager.yml --storage.path=/tmp --cluster.listen-address= >/dev/null
  base="http://127.0.0.1:$port"
  for _ in $(seq 1 30); do curl -fsS "$base/-/ready" >/dev/null 2>&1 && break; sleep 1; done
  now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  post() { curl -fsS -XPOST -H 'Content-Type: application/json' "$base/api/v2/alerts" -d "$1" >/dev/null; }
  post "[
    {\"labels\":{\"alertname\":\"ServiceDown\",\"severity\":\"critical\",\"service\":\"worker\",\"profile\":\"chat\",\"swarm_service\":\"worker-chat\"},\"startsAt\":\"$now\"},
    {\"labels\":{\"alertname\":\"ChatPersistLagHigh\",\"severity\":\"warning\",\"service\":\"worker\",\"profile\":\"chat\",\"module\":\"chat\"},\"startsAt\":\"$now\"},
    {\"labels\":{\"alertname\":\"OutboxLagHigh\",\"severity\":\"critical\",\"service\":\"worker\",\"profile\":\"events\",\"module\":\"game\"},\"startsAt\":\"$now\"},
    {\"labels\":{\"alertname\":\"SwarmReplicasBelowDesired\",\"severity\":\"warning\",\"service\":\"core-api\",\"swarm_service\":\"core-api\"},\"startsAt\":\"$now\"},
    {\"labels\":{\"alertname\":\"SwarmReplicasBelowDesired\",\"severity\":\"critical\",\"service\":\"core-api\",\"swarm_service\":\"core-api\"},\"startsAt\":\"$now\"},
    {\"labels\":{\"alertname\":\"Watchdog\",\"severity\":\"warning\",\"heartbeat\":\"true\"},\"startsAt\":\"$now\"}
  ]"
  sleep 3
  alerts=$(curl -fsS "$base/api/v2/alerts")
  state() { node -e '
    const a = JSON.parse(process.argv[1]); const [name, sev, svc, prof] = process.argv.slice(2);
    const m = a.find((x) => x.labels.alertname === name && x.labels.severity === sev && (x.labels.service ?? "") === svc && (x.labels.profile ?? "") === prof);
    console.log(m ? m.status.state : "missing");' "$alerts" "$@"; }
  check() { local got; got=$(state "$1" "$2" "$3" "$4"); if [ "$got" = "$5" ]; then ok "inhibit $1/$2/$3/$4 → $5"; else ko "inhibit $1/$2/$3/$4 → got $got, want $5"; fi; }
  check ServiceDown critical worker chat active
  check ChatPersistLagHigh warning worker chat suppressed          # same service+profile as ServiceDown
  check OutboxLagHigh critical worker events active                # other profile of the same binary
  check SwarmReplicasBelowDesired warning core-api "" suppressed   # critical twin
  check SwarmReplicasBelowDesired critical core-api "" active
  check Watchdog warning "" "" active
fi

echo "routes_test: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
