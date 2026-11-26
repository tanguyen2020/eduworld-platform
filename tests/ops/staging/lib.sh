#!/usr/bin/env bash
# Shared helpers of the STAGING acceptance scenarios (F130 T010, T012, T028, T035–T037, T045). They run from the
# self-hosted runner `deploy` against the real staging swarm (DOCKER_HOST = manager over TLS) and print PASS/FAIL lines
# to paste into docs/11-operations/restore-tests/<YYYY-MM>.md. Never run them against production.
set -uo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(cd "$here/../../.." && pwd)
STACK="${EW_STACK:-eduworld}"
[ "${EW_ENV_TARGET:-staging}" = staging ] || { echo "staging scenarios only" >&2; exit 2; }
pass=0 fail=0
ok() { pass=$((pass + 1)); echo "PASS $*"; }
ko() { fail=$((fail + 1)); echo "FAIL $*" >&2; }
check() { local n=$1; shift; if "$@"; then ok "$n"; else ko "$n"; fi; }
# task_count SERVICE — number of tasks ever created for a service (a restart creates a new task)
task_count() { docker service ps "${STACK}_$1" -q --no-trunc | wc -l | tr -d ' '; }
# job ARGS… — one-off ops job on staging (scripts/ops/run-ops-job.sh)
job() { bash "$root/scripts/ops/run-ops-job.sh" --env=staging "$@"; }
finish() { echo "scenario $(basename "$0"): $pass passed, $fail failed"; [ "$fail" = 0 ]; }
