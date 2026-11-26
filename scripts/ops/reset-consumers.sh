#!/usr/bin/env bash
# Reset the durable JetStream consumers of ONE module to a point in time T after its database was restored to T
# (F130-AC-17, AC-21; BACKUP_RECOVERY §3.5, §5b, RB-01 step 7, RB-03 step 4).
#   reset-consumers.sh --module=<identity|learning|game|competition|chat> --since=<RFC 3339 UTC> [--streams="…"] [--dry-run] --yes
# The deliver policy of a durable cannot be updated, so each consumer named <module>_* is recreated with the same
# configuration and deliver_policy=by_start_time, opt_start_time=T (consumers are idempotent: processed_events).
# The binaries keep that start position when they start again (pkg/outbox existingStart). Pause the consumers
# first (RB-01 scales worker-events / worker-chat to 0 for the reset window; JetStream keeps the messages).
# When T is older than the first message kept by a stream (event_stream_retention, 7 days), the script stops with
# exit code 3: replay from the producer's outbox (outbox_retention, 30 days) is required instead.
# Runs where the `nats` CLI and an ops NATS context exist (natsio/nats-box job on network `app`, run-ops-job.sh).
set -euo pipefail
# shellcheck source=scripts/backup/lib-backup.sh
. "$(dirname "$0")/../backup/lib-backup.sh" 2>/dev/null || . /opt/eduworld/backup/lib-backup.sh

MODULE="" SINCE="" STREAMS="" DRY_RUN=false YES=false
for arg in "$@"; do
  case "$arg" in
    --module=*) MODULE="${arg#*=}" ;;
    --since=*) SINCE="${arg#*=}" ;;
    --streams=*) STREAMS="${arg#*=}" ;;
    --dry-run) DRY_RUN=true ;;
    --yes) YES=true ;;
    *) die "unknown argument $arg" ;;
  esac
done
[[ "$MODULE" =~ ^(identity|learning|game|competition|chat)$ ]] || die "--module=<identity|learning|game|competition|chat> required"
[[ "$SINCE" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] || die "--since must be RFC 3339 UTC (YYYY-MM-DDTHH:MM:SSZ)"
[ "$DRY_RUN" = true ] || [ "$YES" = true ] || die "refusing to change consumers without --yes (or use --dry-run)"
NATS="${NATS_BIN:-nats}"
creds="${EW_NATS_CREDS_FILE:-/run/secrets/ops_nats_creds}"
[ -s "$creds" ] && export NATS_CREDS="$creds"
[ -n "$STREAMS" ] || STREAMS="EVT_IDENTITY EVT_LEARNING EVT_GAME EVT_COMPETITION EVT_CHAT$([ "$MODULE" = chat ] && echo " CHAT_MESSAGES" || true)"
since_epoch=$(date -u -d "$SINCE" +%s)

work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
count=0
for stream in $STREAMS; do
  [[ "$stream" =~ ^[A-Z_]{1,32}$ ]] || die "invalid stream name"
  first=$("$NATS" stream info "$stream" --json | jq -r '.state.first_ts // empty')
  if [ -n "$first" ] && [ "$(date -u -d "$first" +%s)" -gt "$since_epoch" ]; then
    log_json error "T is older than the first message kept by the stream: replay from the producer outbox" stream="$stream" first="$first"
    exit 3
  fi
  for name in $("$NATS" consumer ls "$stream" --names); do
    case "$name" in "${MODULE}"_*) ;; *) continue ;; esac
    "$NATS" consumer info "$stream" "$name" --json | jq --arg t "$SINCE" '.config
      | del(.name) | .deliver_policy = "by_start_time" | .opt_start_time = $t | del(.opt_start_seq)' > "$work/$name.json"
    if [ "$DRY_RUN" = true ]; then
      log_json info "would reset consumer" stream="$stream" consumer="$name" since="$SINCE"
    else
      "$NATS" consumer rm "$stream" "$name" -f
      "$NATS" consumer add "$stream" --config "$work/$name.json"
      audit consumer_reset module="$MODULE" stream="$stream" consumer="$name" since="$SINCE"
    fi
    count=$((count + 1))
  done
done
log_json info "consumers reset" module="$MODULE" count="$count" dry_run="$DRY_RUN"
