#!/usr/bin/env bash
# F130-AC-18 (TC-SVC-013) on staging, outside school hours: Patroni switchover with a write gap < 60 s and no committed
# row lost (patroni-switchover.sh), and core-api / worker-* reconnect through pg-router/PgBouncer without a task restart.
# shellcheck source=tests/ops/staging/lib.sh
. "$(dirname "$0")/lib.sh"
svcs="core-api worker-events worker-chat worker-jobs"
declare -A before
for s in $svcs; do before[$s]=$(task_count "$s"); done
check "switchover drill (gap < 60 s, no committed row lost)" job --like=pg-1 --secret=ops_drill_dsn -- \
  patroni-switchover.sh --probe-dsn-file=/run/secrets/ops_drill_dsn --yes
sleep 60
for s in $svcs; do check "$s: 0 task restart" test "$(task_count "$s")" = "${before[$s]}"; done
finish
