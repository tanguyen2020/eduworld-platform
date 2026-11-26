#!/usr/bin/env bash
# F130-AC-06, AC-07 (TC-SVC-007) on staging: backend healthchecks use /livez, drain grace periods, datastores publish no
# port, and a task attached ONLY to `edge` cannot reach PostgreSQL, PgBouncer, Valkey, NATS or object storage.
# shellcheck source=tests/ops/staging/lib.sh
. "$(dirname "$0")/lib.sh"
for s in core-api realtime; do
  hc=$(docker service inspect "${STACK}_$s" --format '{{json .Spec.TaskTemplate.ContainerSpec.Healthcheck.Test}}')
  check "$s healthcheck uses /livez ($hc)" grep -q -- '--endpoint=/livez' <<< "$hc"
done
check "core-api stop_grace_period 40s" test "$(docker service inspect "${STACK}_core-api" --format '{{.Spec.TaskTemplate.ContainerSpec.StopGracePeriod}}')" = 40000000000
check "realtime stop_grace_period 150s" test "$(docker service inspect "${STACK}_realtime" --format '{{.Spec.TaskTemplate.ContainerSpec.StopGracePeriod}}')" = 150000000000
for s in pg-1 pg-2 pgbouncer pg-router valkey-cache valkey-ratelimit nats-1 storage-s3 etcd-pg-1; do
  ports=$(docker service inspect "${STACK}_$s" --format '{{json .Endpoint.Ports}}' 2>/dev/null)
  case "${ports:-null}" in null|"[]") ok "$s publishes no port" ;; *) ko "$s publishes $ports" ;; esac
done
probe="${STACK}_edge-probe-$(date +%s)"
docker service create --detach=false --mode replicated-job --restart-condition none --name "$probe" --network edge alpine:3.20 \
  sh -c 'for t in pg-router:5432 pgbouncer:6432 valkey-cache:6379 nats-1:4222 storage-s3:8333; do nc -z -w 3 "${t%:*}" "${t#*:}" && echo "REACHED $t"; done; true' >/dev/null
out=$(docker service logs --raw "$probe" 2>&1)
docker service rm "$probe" >/dev/null
check "edge-only task reaches no datastore" test -z "$(grep REACHED <<< "$out")"
finish
