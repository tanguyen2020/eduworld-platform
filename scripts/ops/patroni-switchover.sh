#!/usr/bin/env bash
# Quarterly Patroni switchover drill (F130-AC-18, TC-SVC-013; BACKUP_RECOVERY §8, RB-02) — outside school hours.
#   patroni-switchover.sh [--candidate=pg-2] [--probe-dsn-file=FILE] [--max-gap=60] --yes
# Writes one row per second into infra_drill.heartbeat of database `postgres` (created idempotently; drill table,
# not business data) through PgBouncer while `patronictl switchover` runs, then reports the longest write gap and
# checks that every committed row survived. Exit 1 when the gap ≥ max-gap seconds (default 60) or a committed row is
# missing. Task restarts of core-api/worker-* during the drill are checked by tests/ops/staging/patroni_switchover.sh.
set -euo pipefail
# shellcheck source=scripts/backup/lib-backup.sh
. "$(dirname "$0")/../backup/lib-backup.sh" 2>/dev/null || . /opt/eduworld/backup/lib-backup.sh

CANDIDATE="" PROBE="${EW_DRILL_DSN_FILE:-/run/secrets/ops_drill_dsn}" MAX_GAP=60 YES=false
for arg in "$@"; do
  case "$arg" in
    --candidate=*) CANDIDATE="${arg#*=}" ;;
    --probe-dsn-file=*) PROBE="${arg#*=}" ;;
    --max-gap=*) MAX_GAP="${arg#*=}" ;;
    --yes) YES=true ;;
    *) die "unknown argument $arg" ;;
  esac
done
[ "$YES" = true ] || die "a switchover interrupts writes: pass --yes"
[ -s "$PROBE" ] || die "probe DSN file missing or empty"
PATRONICTL="${PATRONICTL_BIN:-patronictl}"
dsn=$(cat "$PROBE")
psql "$dsn" -v ON_ERROR_STOP=1 -qc "CREATE SCHEMA IF NOT EXISTS infra_drill; CREATE TABLE IF NOT EXISTS infra_drill.heartbeat (id bigserial PRIMARY KEY, at timestamptz NOT NULL DEFAULT clock_timestamp());"
work=$(mktemp -d); trap 'kill "${writer:-0}" 2>/dev/null || true; rm -rf "$work"' EXIT
( while :; do
    if id=$(psql "$dsn" -tAXqc "INSERT INTO infra_drill.heartbeat DEFAULT VALUES RETURNING id" 2>/dev/null); then echo "$id $(date +%s)" >> "$work/ok"; fi
    sleep 1
  done ) & writer=$!
sleep 5
audit patroni_switchover_started candidate="${CANDIDATE:-auto}"
args=(switchover --force)
[ -n "$CANDIDATE" ] && args+=(--candidate "$CANDIDATE")
"$PATRONICTL" -c /etc/patroni/patroni.yml "${args[@]}"
sleep 30
kill "$writer"; wait "$writer" 2>/dev/null || true
gap=$(awk 'NR>1 && $2-p>m {m=$2-p} {p=$2} END {print m+0}' "$work/ok")
ids=$(cut -d' ' -f1 "$work/ok" | paste -sd, -)
present=$(psql "$dsn" -tAXc "SELECT count(*) FROM infra_drill.heartbeat WHERE id IN ($ids)")
committed=$(wc -l < "$work/ok" | tr -d ' ')
audit patroni_switchover_finished write_gap_seconds="$gap" committed="$committed" present="$present"
[ "$gap" -lt "$MAX_GAP" ] || die "write interruption ${gap}s ≥ ${MAX_GAP}s"
[ "$present" = "$committed" ] || die "committed rows lost: $committed committed, $present present"
log_json info "switchover drill passed" write_gap_seconds="$gap" committed="$committed"
