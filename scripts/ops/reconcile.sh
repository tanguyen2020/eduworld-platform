#!/usr/bin/env bash
# Job one-off reconcile-<module> (F134 US1, BACKUP_RECOVERY §3.5, RB-01 step 7): runs `worker reconcile --module=<m>` as a
# Swarm replicated-job (restart none) copying the image, networks, environment and secrets of worker-jobs (platform DSNs of
# the five module databases), through scripts/ops/run-ops-job.sh. The command only reads ids (facades
# listOutboxEventIds / listProcessedEventIds, in process) and writes one audit ops.reconcile.completed; it never
# compensates. The JSON report (specs/schemas/ops/reconcile-report.schema.json) is the single stdout line of the job log,
# the summary table goes to stderr; the job state is Complete for exit 0 only.
#   reconcile.sh --env=<staging|production> --module=<identity|learning|game|competition|chat> \
#     --restore-point=<T RFC 3339 UTC> --from=<RFC 3339> [--to=<RFC 3339>] [--settle=10m]
# Exit code of the command (BACKUP_RECOVERY §8): 0 no unexplained mismatch, 2 unexplained, 3 a module unreadable,
# 1 bad arguments (window > outbox_retention, from ≥ to, T outside the window) — nothing is called then.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)

ENV="" MOD="" T="" FROM="" TO="" SETTLE=""
for a in "$@"; do
  case "$a" in
    --env=*) ENV="${a#*=}" ;;
    --module=*) MOD="${a#*=}" ;;
    --restore-point=*) T="${a#*=}" ;;
    --from=*) FROM="${a#*=}" ;;
    --to=*) TO="${a#*=}" ;;
    --settle=*) SETTLE="${a#*=}" ;;
    *) echo "reconcile: unknown argument $a" >&2; exit 1 ;;
  esac
done
case "$MOD" in identity|learning|game|competition|chat) ;; *) echo "reconcile: --module=<identity|learning|game|competition|chat>" >&2; exit 1 ;; esac
rfc='^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(Z|[+-][0-9]{2}:[0-9]{2})$'
[[ "$T" =~ $rfc ]] || { echo "reconcile: --restore-point=<RFC 3339> required" >&2; exit 1; }
[[ "$FROM" =~ $rfc ]] || { echo "reconcile: --from=<RFC 3339> required" >&2; exit 1; }
[ -z "$TO" ] || [[ "$TO" =~ $rfc ]] || { echo "reconcile: --to must be RFC 3339" >&2; exit 1; }
[ -z "$SETTLE" ] || [[ "$SETTLE" =~ ^[0-9]+(s|m|h)$ ]] || { echo "reconcile: --settle like 10m" >&2; exit 1; }

cmd=(reconcile "--module=$MOD" "--restore-point=$T" "--from=$FROM" --out=-)
[ -z "$TO" ] || cmd+=("--to=$TO")
[ -z "$SETTLE" ] || cmd+=("--settle=$SETTLE")
if [ "${RECONCILE_DRY_RUN:-}" = 1 ]; then # tests: print the job command instead of creating it
  printf '%s\n' "run-ops-job.sh --env=$ENV --like=worker-jobs -- ${cmd[*]}"
  exit 0
fi
exec bash "$here/run-ops-job.sh" --env="$ENV" --like=worker-jobs -- "${cmd[@]}"
