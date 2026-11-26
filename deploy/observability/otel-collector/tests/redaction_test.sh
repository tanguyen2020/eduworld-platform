#!/usr/bin/env bash
# Redaction of the central collector (F131 T010, F131-AC-05, TC-AUTH-060): runs otelcol-contrib in a throwaway container
# with the processors of collector.swarm.yaml unchanged (redaction + transform/redact_body), a filelog receiver on a
# fixture of slog JSON lines carrying a JWT, a bearer token, an email, a Vietnamese phone number and a password, and a
# file exporter; then checks that no sample value survives and that the masks are present. The fixture values are
# synthetic and assembled at run time (nothing secret-looking is committed). Container: ew-obs-otel-* (removed).
#   redaction_test.sh
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../../../.." && pwd)
image=${EW_OTELCOL_IMAGE:-otel/opentelemetry-collector-contrib:0.120.0}
winpath() { if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else echo "$1"; fi; }
export MSYS_NO_PATHCONV=1
work=$(mktemp -d)
name="ew-obs-otel-$$"
cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; rm -rf "$work"; }
trap cleanup EXIT
mkdir -p "$work/in" "$work/out"

b64() { printf '%s' "$1" | base64 | tr -d '=\n' | tr '/+' '_-'; }
jwt="$(b64 '{"alg":"HS256","typ":"JWT"}').$(b64 '{"sub":"synthetic-test-subject"}').$(b64 'synthetic-signature-for-test')"
email="hoc.sinh.thu$((RANDOM % 9))@example.invalid"
phone="09$((10000000 + RANDOM % 89999999))"
bearer="Bearer synthetic$(b64 'bearer-token-for-test')"
pw="synthetic-pw-$RANDOM"
cat > "$work/in/app.log" <<EOF
{"level":"INFO","msg":"login ok for $email","service":"core-api","module":"identity","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"}
{"level":"WARN","msg":"refresh rejected","service":"core-api","module":"identity","authorization":"$bearer","note":"token $jwt"}
{"level":"ERROR","msg":"sms gateway failed for $phone","service":"worker","profile":"jobs","module":"identity"}
{"level":"ERROR","msg":"bad payload {\"password\":\"$pw\"}","service":"core-api","module":"identity","password":"$pw","contact":"$email"}
EOF
chmod -R a+rwX "$work"

cp "$repo/deploy/observability/otel-collector/collector.swarm.yaml" "$work/collector.swarm.yaml"
(cd "$repo" && node --input-type=module - "$(winpath "$work")") <<'EOF'
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
const work = process.argv[2];
const yaml = createRequire(path.join(process.cwd(), 'scripts', 'package.json'))('js-yaml');
const src = path.join(work, 'collector.swarm.yaml');
const dst = path.join(work, 'config.yaml');
const prod = yaml.load(fs.readFileSync(src, 'utf8'));
const cfg = {
  receivers: { filelog: { include: ['/in/*.log'], start_at: 'beginning', operators: [{ type: 'json_parser', parse_from: 'body' }] } },
  processors: { redaction: prod.processors.redaction, 'transform/redact_body': prod.processors['transform/redact_body'] },
  exporters: { file: { path: '/out/out.json' } },
  service: { pipelines: { logs: { receivers: ['filelog'], processors: ['redaction', 'transform/redact_body'], exporters: ['file'] } } },
};
// The body is the raw JSON line; the parsed fields are attributes (as the agent produces them).
fs.writeFileSync(dst, yaml.dump(cfg));
EOF

docker run -d --name "$name" -v "$(winpath "$work/config.yaml"):/etc/otel/config.yaml:ro" -v "$(winpath "$work/in"):/in:ro" \
  -v "$(winpath "$work/out"):/out" "$image" --config=/etc/otel/config.yaml >/dev/null
for _ in $(seq 1 30); do [ -s "$work/out/out.json" ] && grep -q 'bad payload' "$work/out/out.json" && break; sleep 1; done
sleep 2
docker logs "$name" 2>&1 | grep -iE 'error|invalid' | grep -v 'level=info' >&2 || true
out=$(cat "$work/out/out.json" 2>/dev/null || true)
pass=0 fail=0
ok() { pass=$((pass + 1)); echo "ok   $*"; }
ko() { fail=$((fail + 1)); echo "FAIL $*" >&2; }
[ -n "$out" ] && ok "collector exported the fixture" || ko "no output from the collector"
for v in "$jwt" "$email" "$phone" "$pw" "${bearer#Bearer }"; do
  if grep -qF -- "$v" <<< "$out"; then ko "sample value leaked: ${v:0:12}…"; else ok "masked: ${v:0:12}…"; fi
done
grep -q '\*\*\*EMAIL\*\*\*' <<< "$out" && ok "body email mask present" || ko "body email mask missing"
grep -q '\*\*\*PHONE\*\*\*' <<< "$out" && ok "body phone mask present" || ko "body phone mask missing"
grep -q 'redaction' <<< "$out" && ok "redaction summary attributes present" || ko "redaction summary attributes missing"
grep -q '4bf92f3577b34da6a3ce929d0e0e4736' <<< "$out" && ok "trace_id kept" || ko "trace_id removed"
echo "redaction_test: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
