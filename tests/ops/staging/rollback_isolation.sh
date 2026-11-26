#!/usr/bin/env bash
# F130-AC-03 (TC-SVC-015) on staging: a core-api image whose /livez fails is rolled back automatically by Swarm
# (failure_action rollback after `monitor`), no task of realtime/worker-* restarts, and a rollback of one service
# completes in ≤ 15 minutes.   BROKEN_VERSION=<core-api build with /livez failing>  GOOD_VERSION=<current tag>
# shellcheck source=tests/ops/staging/lib.sh
. "$(dirname "$0")/lib.sh"
: "${BROKEN_VERSION:?}" "${GOOD_VERSION:?}"
others="realtime worker-events worker-chat worker-jobs"
declare -A before
for s in $others; do before[$s]=$(task_count "$s"); done
EW_DEPLOY_REASON=tc-svc-015 bash "$root/scripts/deploy/deploy-service.sh" --env=staging --stack=app --version="$BROKEN_VERSION" \
  --service=core-api --skip-migrate >/dev/null 2>&1
state=$(docker service inspect "${STACK}_core-api" --format '{{if .UpdateStatus}}{{.UpdateStatus.State}}{{end}}')
check "Swarm rolled core-api back by itself (UpdateStatus=$state)" test "$state" = rollback_completed
for s in $others; do check "$s: 0 restart during the core-api deploy" test "$(task_count "$s")" = "${before[$s]}"; done
bash "$root/scripts/deploy/deploy-service.sh" --env=staging --stack=app --version="$GOOD_VERSION" --service=worker-chat --skip-migrate >/dev/null
t0=$(date +%s)
bash "$root/scripts/deploy/rollback.sh" --env=staging --service=worker-chat --reason=tc-svc-015 >/dev/null
elapsed=$(( $(date +%s) - t0 ))
check "rollback of worker-chat in ≤ 900 s (${elapsed} s)" test "$elapsed" -le 900
finish
