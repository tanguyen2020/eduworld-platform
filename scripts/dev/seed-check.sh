#!/usr/bin/env bash
# make seed-check (LOCAL_DEVELOPMENT §5): after `make seed` and `make up-app`, wait until the worker relayed the events
# of the identity seed and the identity projections of the other modules hold every seeded student: each effective student
# (status active, not soft-deleted; ADR-0095: no enrollment) of module identity must be in ref.students of module learning and module game (projections are
# loaded through events only, never seeded). Read-only: SELECT as the <module>_platform roles inside the postgres
# container. Honors COMPOSE_PROJECT_NAME; PW is the local role password (default of deploy/postgres/init).
set -euo pipefail
cd "$(dirname "$0")/../.."
PW="${PW:-${EW_DB_ROLE_PASSWORD:-change-me-local}}"
TIMEOUT="${SEED_CHECK_TIMEOUT:-180}"
dc=(docker compose -f deploy/compose/docker-compose.yml)

q() { # q <module> <sql>
  MSYS_NO_PATHCONV=1 "${dc[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -Atq \
    "postgresql://$1_platform:${PW}@localhost:5432/eduworld_$1" -c "$2"
}

ids=$(q identity "SELECT string_agg(DISTINCT student_id::text, ',') FROM org.students
  WHERE status = 'active' AND deleted_at IS NULL AND tenant_id IN (SELECT tenant_id FROM platform.tenants WHERE tenant_type <> 'common' AND deleted_at IS NULL)")
if [ -z "$ids" ]; then
  echo "seed-check: module identity has no effective student: run make seed first" >&2
  exit 1
fi
want=$(tr ',' '\n' <<<"$ids" | wc -l | tr -d ' ')
deadline=$(( $(date +%s) + TIMEOUT ))
while :; do
  ok=1
  report="identity=$want"
  for m in learning game; do
    n=$(q "$m" "SELECT count(*) FROM ref.students WHERE student_id = ANY('{$ids}'::uuid[]) AND deleted_at IS NULL")
    report="$report $m=$n"
    [ "$n" -ge "$want" ] || ok=0
  done
  if [ "$ok" = 1 ]; then
    echo "seed-check: ref.students complete ($report)"
    exit 0
  fi
  if [ "$(date +%s)" -ge "$deadline" ]; then
    echo "seed-check: projections incomplete after ${TIMEOUT}s ($report): is the worker (make up-app) running?" >&2
    exit 1
  fi
  sleep 3
done
