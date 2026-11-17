#!/usr/bin/env bash
# F001 T020 — smoke test of the local stack: F001-AC-01 (infra healthy, s3-init exited 0),
# F001-AC-02 (.env files ignored by git), F001-AC-07 (Grafana datasources provisioned when obs is up).
# Requires `make up` (and `make up-obs` for the AC-07 part; skipped with a notice when obs is down).
set -euo pipefail
cd "$(dirname "$0")/../.."
DC=(docker compose -f deploy/compose/docker-compose.yml -f deploy/compose/docker-compose.app.yml -f deploy/compose/docker-compose.observability.yml)
fail=0

for s in postgres valkey nats keycloak apisix mailpit floci cdn sis-mock; do
  h=$("${DC[@]}" ps --format '{{.Service}} {{.Health}}' "$s" 2>/dev/null | awk '{print $2}')
  if [ "$h" = healthy ]; then echo "ok   - $s healthy"; else echo "FAIL - $s health=${h:-missing}"; fail=1; fi
done
code=$(docker inspect --format '{{.State.ExitCode}}' eduworld-s3-init-1 2>/dev/null || echo missing)
if [ "$code" = 0 ]; then echo "ok   - s3-init exited 0"; else echo "FAIL - s3-init exit=$code"; fail=1; fi

for f in .env src/backend/modules/identity/.env; do
  if git check-ignore -q --no-index "$f"; then echo "ok   - $f ignored by git"; else echo "FAIL - $f not ignored"; fail=1; fi
done

if "${DC[@]}" --profile obs ps --services --status running 2>/dev/null | grep -qx grafana; then
  ds=$(node -e 'fetch("http://localhost:3000/api/datasources").then(r=>r.json()).then(d=>process.stdout.write(d.map(x=>x.type).sort().join(","))).catch(()=>{})')
  if [ "$ds" = "loki,prometheus,tempo" ]; then echo "ok   - grafana datasources: $ds"; else echo "FAIL - grafana datasources: ${ds:-none}"; fail=1; fi
else
  echo "skip - obs stack not running (make up-obs) — F001-AC-07 not checked"
fi
exit $fail
