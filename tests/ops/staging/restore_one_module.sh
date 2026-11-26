#!/usr/bin/env bash
# F130-AC-17 (TC-SVC-014) on staging: eduworld_learning restored to T into a new database and swapped in (old copy
# kept), consumers of module learning reset to T, ref_* projections rebuilt, reconcile run; other modules keep serving.
#   TARGET="<YYYY-MM-DD HH:MM:SS+07>"  ACTOR=…  [RECONCILE_CMD="reconcile --module=learning"]  (reconcile: F134)
# shellcheck source=tests/ops/staging/lib.sh
. "$(dirname "$0")/lib.sh"
: "${TARGET:?}" "${ACTOR:?}"
since=$(date -u -d "$TARGET" +%Y-%m-%dT%H:%M:%SZ)
check "rb01 restore → new database" job --like=backup-tools --restore --admin -- \
  rb01-pitr-restore.sh --phase=restore --db=eduworld_learning --target="$TARGET" --actor="$ACTOR" --purpose=TC-SVC-014
list_sql="SELECT max(datname) FROM pg_database WHERE datname LIKE 'eduworld_learning_restored_%'"
restored=$(job --like=backup-tools --admin -- sh -c "PGPASSFILE=/run/secrets/ops_pg_admin_pgpass psql -h pg-router -U postgres -tAXc \"$list_sql\"" 2>/dev/null | tail -1)
check "rb01 swap ($restored)" job --like=backup-tools --admin -- \
  rb01-pitr-restore.sh --phase=swap --db=eduworld_learning --restored="$restored" --actor="$ACTOR" --purpose=TC-SVC-014 --yes
docker service scale --detach=false "${STACK}_worker-events=0" >/dev/null
check "consumers of learning reset to T" job --like=backup-tools --network=app --secret=ops_nats_creds -- \
  reset-consumers.sh --module=learning --since="$since" --yes
docker service scale --detach=false "${STACK}_worker-events=2" >/dev/null
check "projection rebuild (learning ← identity)" job --like=worker-jobs -- projection rebuild --module=learning --source=identity
if [ -n "${RECONCILE_CMD:-}" ]; then
  # shellcheck disable=SC2086 # RECONCILE_CMD is a command line
  check "reconcile: 0 unexplained difference" job --like=worker-jobs -- $RECONCILE_CMD
else
  ko "reconcile-<module> not available yet (command owned by F134)"
fi
probe="${STACK}_ready-probe-$(date +%s)"
docker service create --detach=false --mode replicated-job --restart-condition none --name "$probe" --network edge alpine:3.20 \
  sh -c 'wget -qO- http://core-api:9090/readyz' >/dev/null 2>&1
out=$(docker service logs --raw "$probe" 2>&1); docker service rm "$probe" >/dev/null
check "core-api ready, other modules serving" grep -q '"identity"' <<< "$out"
finish
