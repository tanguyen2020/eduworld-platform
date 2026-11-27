#!/usr/bin/env bash
# Chaos runner (F132 T018–T021, T040; F132-AC-08…AC-14, AC-25, AC-26; TEST_STRATEGY §2, §3). For each scenario of
# tests/chaos/scenarios/<name>.yaml it injects the fault, observes Prometheus (expectations + expected F131 alerts),
# ALWAYS recovers (trap: scale back, undrain, re-allow the database, restore env, remove chaos services; a recovery that
# does not converge is forced with `docker service update --force` and the scenario is failed), reconciles the bot
# ledger with PostgreSQL (0 lost) and appends the result to docs/12-project/chaos-reports/<YYYY-MM-DD>.md.
#
#   run-chaos.sh --scenario=<name|all> [--target=<node|integration|nats|pg-router>] [--dry-run]
#
# Environment (never production — F132-AC-07):
#   EW_ENV                 local | dev | staging (required; anything else is refused)
#   EW_CHAOS_MODE          swarm (default for dev/staging: DOCKER_HOST = manager over TLS from the self-hosted runner
#                          `deploy`, as every F130 ops script) | compose (default for local: throwaway containers named
#                          $EW_CHAOS_CONTAINER_PREFIX<service>, never eduworld-*)
#   EW_PROMETHEUS_URL      Prometheus of the environment (expectations, ALERTS)
#   EW_STACK               Swarm stack (default eduworld); EW_CHAOS_PG_SERVICE Patroni service of the ops job (pg-1)
#   EW_CHAOS_PG_CONTAINER  compose mode: PostgreSQL container of the throwaway stack
#   EW_CHAOS_LEDGER_DIR    bot ledgers written by loadbot: <scenario>-attempts.txt / <scenario>-chat.txt (one id/line)
#   EW_CHAOS_RECONCILE_DSN_LEARNING_FILE / EW_CHAOS_RECONCILE_DSN_CHAT_FILE  read-only DSNs (bastion tunnel)
#   EW_CHAOS_SIS_UPSTREAM  host:port of the SIS sandbox (external-fault --target=sis)
#   EW_CHAOS_REPORT_DIR    default docs/12-project/chaos-reports; EW_CHAOS_SETTLE seconds before `after` checks (120)
#   EW_CHAOS_PUMBA_IMAGE / EW_CHAOS_TOXIPROXY_IMAGE  images of the one-off chaos services
# Exit: 0 every scenario passed, 1 a scenario failed, 2 usage/refused.
set -uo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(cd "$here/../.." && pwd)
SCN_DIR="${EW_CHAOS_SCENARIOS:-$here/scenarios}"
REPORT_DIR="${EW_CHAOS_REPORT_DIR:-$root/docs/12-project/chaos-reports}"
STACK="${EW_STACK:-eduworld}"
DOCKER="${DOCKER_BIN:-docker}"
NODE="${NODE_BIN:-node}"
PROM="${EW_PROMETHEUS_URL:-}"
SETTLE="${EW_CHAOS_SETTLE:-120}"
PUMBA_IMAGE="${EW_CHAOS_PUMBA_IMAGE:-gaiaadm/pumba:0.10.1}"
TOXI_IMAGE="${EW_CHAOS_TOXIPROXY_IMAGE:-ghcr.io/shopify/toxiproxy:2.9.0}"
PREFIX="${EW_CHAOS_CONTAINER_PREFIX:-ew-f132-chaos-}"
LIB="$here/lib/scenario.mjs"

log() { printf '%s chaos: %s\n' "$(date -u +%H:%M:%S)" "$*" >&2; }
die() { log "ERROR: $*"; exit 2; }

SCENARIO="" TARGET="" DRY=false
for arg in "$@"; do
  case "$arg" in
    --scenario=*) SCENARIO="${arg#*=}" ;;
    --target=*) TARGET="${arg#*=}" ;;
    --dry-run) DRY=true ;;
    *) die "unknown argument $arg" ;;
  esac
done
[ -n "$SCENARIO" ] || die "--scenario=<name|all> required"

# envguard (same rule as src/backend/tools/envguard): never production, never an unnamed environment.
ENVNAME=$(printf '%s' "${EW_ENV:-}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
case "$ENVNAME" in
  local|dev|staging) ;;
  *) die "refused: EW_ENV='${EW_ENV:-}' is not a test environment (local, dev, staging)" ;;
esac
MODE="${EW_CHAOS_MODE:-}"
if [ -z "$MODE" ]; then [ "$ENVNAME" = local ] && MODE=compose || MODE=swarm; fi
case "$MODE" in swarm|compose) ;; *) die "EW_CHAOS_MODE must be swarm or compose" ;; esac
if [ "$MODE" = compose ]; then
  case "$PREFIX" in eduworld*|"") die "refused: compose prefix '$PREFIX' would reach the owner's eduworld-* stack" ;; esac
fi

# ---------- target helpers ----------
name_of() { if [ "$MODE" = swarm ]; then printf '%s_%s' "$STACK" "$1"; else printf '%s%s' "$PREFIX" "$1"; fi; }

desired_of() { # desired replicas of a service (compose: 1)
  if [ "$MODE" = compose ]; then echo 1; return; fi
  $DOCKER service inspect "$(name_of "$1")" --format '{{.Spec.Mode.Replicated.Replicas}}'
}

scale() { # scale SERVICE N
  if [ "$MODE" = swarm ]; then $DOCKER service scale -d "$(name_of "$1")=$2" >/dev/null
  elif [ "$2" = 0 ]; then $DOCKER stop "$(name_of "$1")" >/dev/null
  else $DOCKER start "$(name_of "$1")" >/dev/null; fi
}

converged() { # converged SERVICE N — running replicas == N
  if [ "$MODE" = compose ]; then
    local r; r=$($DOCKER inspect -f '{{.State.Running}}' "$(name_of "$1")" 2>/dev/null)
    if [ "$2" = 0 ]; then [ "$r" != true ]; else [ "$r" = true ]; fi
    return
  fi
  local reps; reps=$($DOCKER service ls --filter "name=$(name_of "$1")" --format '{{.Replicas}}' | head -1)
  [ "${reps%%/*}" = "$2" ] && [ "${reps#*/}" = "$2" ]
}

sql_admin() { # superuser statement on the PostgreSQL cluster (staging: ops job on the Patroni leader via pg-router)
  if [ "$MODE" = compose ]; then
    [ -n "${EW_CHAOS_PG_CONTAINER:-}" ] || { log "EW_CHAOS_PG_CONTAINER required in compose mode"; return 1; }
    case "$EW_CHAOS_PG_CONTAINER" in eduworld*) log "refused: $EW_CHAOS_PG_CONTAINER belongs to the owner's stack"; return 1 ;; esac
    $DOCKER exec "$EW_CHAOS_PG_CONTAINER" psql -U postgres -d postgres -v ON_ERROR_STOP=1 -tAc "$1"
    return
  fi
  # The admin pgpass secret is mounted read-only 0444; psql ignores a pgpass that is not 0600, so it is copied first.
  # shellcheck disable=SC2016 # expanded by the job's shell, not here
  bash "$root/scripts/ops/run-ops-job.sh" --env="$ENVNAME" --like="${EW_CHAOS_PG_SERVICE:-pg-1}" --admin --timeout=300 -- \
    sh -c 'f=$(mktemp); cat /run/secrets/ops_pg_admin_pgpass > "$f"; chmod 600 "$f"; PGPASSFILE="$f" psql -h "${EW_PG_ADMIN_HOST:-pg-router}" -U "${EW_PG_ADMIN_USER:-postgres}" -d postgres -v ON_ERROR_STOP=1 -tAc "$1"; rc=$?; rm -f "$f"; exit $rc' \
    sh "$1"
}

chaos_job() { # chaos_job NAME [docker service create options…] -- IMAGE ARGS… — one-off job, removed by recovery
  local name="$1"; shift
  $DOCKER service create --detach --quiet --mode replicated-job --restart-condition none --name "$name" \
    --label ew.chaos=true --label "ew.actor=${EW_ACTOR:-${GITHUB_ACTOR:-unknown}}" "$@" >/dev/null
}

prom() { # prom QUERY → value (NaN when Prometheus is not configured or answers nothing)
  [ -n "$PROM" ] || { echo NaN; return; }
  curl -fsS --max-time 20 --get --data-urlencode "query=$1" "${PROM%/}/api/v1/query" 2>/dev/null | $NODE "$LIB" value
}

# ---------- recovery stack ----------
RECOVER=()
push_recover() { RECOVER+=("$(printf '%q ' "$@")"); }
run_recover() { # runs the recovery actions in reverse order; returns 1 if one failed
  local i rc=0
  for ((i = ${#RECOVER[@]} - 1; i >= 0; i--)); do
    log "recover: ${RECOVER[i]}"
    eval "${RECOVER[i]}" || { log "recovery action failed: ${RECOVER[i]}"; rc=1; }
  done
  RECOVER=()
  return $rc
}

HEALTH=() # converged checks after recovery: "SERVICE N"
force_recover() { # last resort: restart the backend services and re-apply the expected scales
  log "forcing recovery"
  local s
  if [ "$MODE" = swarm ]; then
    for s in core-api realtime worker-events worker-chat worker-jobs; do $DOCKER service update --detach --force "$(name_of "$s")" >/dev/null || true; done
  fi
  for s in "${HEALTH[@]}"; do scale "${s% *}" "${s#* }" || true; done
}

wait_healthy() { # every HEALTH entry converged within SC_RECOVERY_TIMEOUT
  local t=0 s ok
  while :; do
    ok=true
    for s in "${HEALTH[@]}"; do converged "${s% *}" "${s#* }" || { ok=false; break; }; done
    $ok && return 0
    [ "$t" -ge "$SC_RECOVERY_TIMEOUT" ] && return 1
    sleep 10; t=$((t + 10))
  done
}

# ---------- injection per fault type ----------
inject() {
  local s n db node stamp
  stamp=$(date -u +%Y%m%d%H%M%S)
  case "$SC_TYPE" in
    module_db_down)
      db="$SC_TARGET"
      [[ "$db" =~ ^eduworld_(identity|learning|game|competition|chat)$ ]] || { log "bad database $db"; return 1; }
      push_recover sql_admin "ALTER DATABASE $db ALLOW_CONNECTIONS true"
      sql_admin "ALTER DATABASE $db ALLOW_CONNECTIONS false" >/dev/null || return 1
      sql_admin "SELECT count(pg_terminate_backend(pid)) FROM pg_stat_activity WHERE datname = '$db' AND pid <> pg_backend_pid()" >/dev/null || return 1
      ;;
    nats_down|valkey_down|swarm_service_down)
      IFS=, read -r -a svcs <<< "$SC_TARGET"
      for s in "${svcs[@]}"; do
        n=$(desired_of "$s") || return 1
        [[ "$n" =~ ^[0-9]+$ ]] || { log "no replica count for $s"; return 1; }
        push_recover scale "$s" "$n"
        HEALTH+=("$s $n")
        scale "$s" 0 || return 1
      done
      ;;
    replica_kill)
      HEALTH+=("$SC_TARGET $(desired_of "$SC_TARGET")")
      if [ "$MODE" = compose ]; then
        push_recover scale "$SC_TARGET" 1
        $DOCKER kill "$(name_of "$SC_TARGET")" >/dev/null || return 1
      else
        node=$($DOCKER service ps "$(name_of "$SC_TARGET")" --filter desired-state=running --format '{{.Node}}' | head -1)
        [ -n "$node" ] || { log "no running task of $SC_TARGET"; return 1; }
        push_recover "$DOCKER" service rm "${STACK}_chaos-kill-$stamp"
        chaos_job "${STACK}_chaos-kill-$stamp" --constraint "node.hostname==$node" \
          --mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock \
          "$PUMBA_IMAGE" --random kill --signal SIGKILL "re2:^$(name_of "$SC_TARGET")\\." || return 1
      fi
      ;;
    node_drain)
      [ "$MODE" = swarm ] || { log "node_drain needs a Swarm (staging)"; return 1; }
      [ -n "$SC_TARGET" ] || { log "--target=<app node> required"; return 1; }
      push_recover "$DOCKER" node update --availability active "$SC_TARGET"
      $DOCKER node update --availability drain "$SC_TARGET" >/dev/null || return 1
      ;;
    db_failover)
      [ "$MODE" = swarm ] || { log "db_failover needs Patroni (staging)"; return 1; }
      # The drill itself fails (exit 1) on a write gap ≥ max_gap_seconds or a lost committed row (F130-AC-18).
      if bash "$root/scripts/ops/run-ops-job.sh" --env="$ENVNAME" --like="${EW_CHAOS_PG_SERVICE:-pg-1}" --secret=ops_drill_dsn -- \
        patroni-switchover.sh --probe-dsn-file=/run/secrets/ops_drill_dsn --max-gap="${SC_P_MAX_GAP_SECONDS:-60}" --yes; then
        record during "write interruption < ${SC_P_MAX_GAP_SECONDS:-60} s, no committed row lost (patroni-switchover.sh)" ok "exit 0" "== 0"
      else
        record during "write interruption < ${SC_P_MAX_GAP_SECONDS:-60} s, no committed row lost (patroni-switchover.sh)" FAIL "exit 1" "== 0"
      fi
      ;;
    external_fault)
      [ "$MODE" = swarm ] || { log "external_fault needs the staging adapters"; return 1; }
      local var="" upstream="" listen="" newval="" orig=""
      case "$SC_TARGET" in
        keycloak-admin) var=EW_KC_ADMIN_BASE_URL upstream=keycloak:8080 listen=18080 ;;
        object-storage) var=EW_S3_ENDPOINT upstream=storage-s3:8333 listen=18333 ;;
        sis) upstream="${EW_CHAOS_SIS_UPSTREAM:-}" listen=18083
             [ -n "$upstream" ] || { log "precondition: EW_CHAOS_SIS_UPSTREAM and a perf-* SIS connection on chaos-toxiproxy:18083"; return 1; } ;;
        *) log "--target=keycloak-admin|sis|object-storage required"; return 1 ;;
      esac
      push_recover "$DOCKER" service rm "${STACK}_chaos-toxiproxy"
      $DOCKER service create --detach --quiet --name "${STACK}_chaos-toxiproxy" --label ew.chaos=true \
        --network name=data,alias=chaos-toxiproxy "$TOXI_IMAGE" -host=0.0.0.0 >/dev/null || return 1
      push_recover "$DOCKER" service rm "${STACK}_chaos-toxi-cfg-$stamp" "${STACK}_chaos-toxi-add-$stamp"
      chaos_job "${STACK}_chaos-toxi-cfg-$stamp" --network data --entrypoint /toxiproxy-cli "$TOXI_IMAGE" \
        -h chaos-toxiproxy:8474 create -l "0.0.0.0:$listen" -u "$upstream" ext || return 1
      chaos_job "${STACK}_chaos-toxi-add-$stamp" --network data --entrypoint /toxiproxy-cli "$TOXI_IMAGE" \
        -h chaos-toxiproxy:8474 toxic add -t "${SC_P_TOXIC:-timeout}" -a "timeout=${SC_P_TIMEOUT_MS:-0}" ext || return 1
      if [ -n "$var" ]; then
        orig=$($DOCKER service inspect "$(name_of core-api)" --format '{{json .Spec.TaskTemplate.ContainerSpec.Env}}' |
          $NODE -e 'const v=process.argv[1];const e=JSON.parse(require("fs").readFileSync(0,"utf8"))||[];const l=e.find(x=>x.startsWith(v+"="));process.stdout.write(l?l.slice(v.length+1):"")' "$var")
        [ -n "$orig" ] || { log "core-api has no $var in its spec"; return 1; }
        newval=$(printf '%s' "$orig" | sed -E "s#^(https?://)[^/]+#\\1chaos-toxiproxy:$listen#")
        push_recover "$DOCKER" service update --detach=false --env-add "$var=$orig" "$(name_of core-api)"
        $DOCKER service update --detach=false --env-add "$var=$newval" "$(name_of core-api)" >/dev/null || return 1
      fi
      ;;
    latency)
      [ "$MODE" = swarm ] || { log "latency (Pumba netem) needs the Swarm nodes"; return 1; }
      local targets
      case "$SC_TARGET" in nats) targets="nats-1 nats-2 nats-3" ;; pg-router) targets="pg-router" ;; *) log "--target=nats|pg-router required"; return 1 ;; esac
      for s in $targets; do
        while IFS= read -r node; do
          [ -n "$node" ] || continue
          n="${STACK}_chaos-netem-$s-$node-$stamp"
          push_recover "$DOCKER" service rm "$n"
          chaos_job "$n" --constraint "node.hostname==$node" \
            --mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock "$PUMBA_IMAGE" \
            netem --duration "${SC_DURATION}s" delay --time "${SC_P_DELAY_MS:-200}" --jitter "${SC_P_JITTER_MS:-50}" \
            "re2:^$(name_of "$s")\\." || return 1
        done < <($DOCKER service ps "$(name_of "$s")" --filter desired-state=running --format '{{.Node}}' | sort -u)
      done
      ;;
    *) log "unknown fault type $SC_TYPE"; return 1 ;;
  esac
}

# ---------- results ----------
ROWS=() FAILS=()
record() { # record PHASE NAME RESULT VALUE CONDITION
  ROWS+=("| $1 | $2 | $4 | $5 | $3 |")
  [ "$3" = ok ] || FAILS+=("$1: $2")
}
check_phase() { # check_phase during|after
  local name op value q v
  while IFS=$'\t' read -r name op value q; do
    [ -n "$name" ] || continue
    v=$(prom "$q")
    if $NODE "$LIB" compare "$v" "$op" "$value"; then record "$1" "$name" ok "$v" "$op $value"
    else record "$1" "$name" FAIL "$v" "$op $value"; fi
  done < <($NODE "$LIB" expect "$FILE" "$1")
}
ALERT_ROWS=()
check_alerts() { # expected F131 alerts seen pending or firing during the fault (informational for alerts whose `for:`
  local name v seen                                     # exceeds the window; a missing alert is reported, not failed)
  while IFS=$'\t' read -r name _; do
    [ -n "$name" ] || continue
    v=$(prom "max(ALERTS{alertname=\"$name\"})")
    if [ "$v" = 1 ]; then seen=yes; else seen="no ($v)"; fi
    ALERT_ROWS+=("| $name | $seen | [runbook](../../11-operations/runbooks/$name.md) |")
  done < <($NODE "$LIB" alerts "$FILE")
}

RECON="not required"
reconcile() {
  local kind="$SC_RECONCILE" ledger dsnf sql out ids
  [ "$kind" = none ] && return 0
  case "$kind" in
    attempts) ledger="${EW_CHAOS_LEDGER_DIR:-}/$SC_NAME-attempts.txt" dsnf="${EW_CHAOS_RECONCILE_DSN_LEARNING_FILE:-}" sql="$here/reconcile/attempts.sql" ;;
    chat) ledger="${EW_CHAOS_LEDGER_DIR:-}/$SC_NAME-chat.txt" dsnf="${EW_CHAOS_RECONCILE_DSN_CHAT_FILE:-}" sql="$here/reconcile/chat_messages.sql" ;;
  esac
  if [ -z "${EW_CHAOS_LEDGER_DIR:-}" ] || [ ! -f "$ledger" ] || [ -z "$dsnf" ] || [ ! -s "$dsnf" ]; then
    RECON="NOT RUN (bot ledger or read-only DSN missing)"; FAILS+=("reconciliation not run"); return 1
  fi
  ids=$(grep -E '^[0-9a-fA-F-]{36}$' "$ledger" | paste -sd, -)
  out=$(psql "$(cat "$dsnf")" -X -q -A -t -F '|' -v ON_ERROR_STOP=1 -v tenant_prefix=perf- -v from="$T_START" -v to="$T_END" \
    -v ids="$ids" -f "$sql" 2>&1 | grep '|' | tail -1) || true
  IFS='|' read -r a b c d <<< "$out"
  if [ "$kind" = attempts ]; then
    RECON="db_submitted=${a:-?} bot_acked=${b:-?} missing=${c:-?}"
    [ "${c:-x}" = 0 ] && [ "${a:-0}" -ge "${b:-1}" ] 2>/dev/null && return 0
  else
    RECON="db_messages=${a:-?} bot_acked=${b:-?} missing=${c:-?} duplicates=${d:-?}"
    [ "${c:-x}" = 0 ] && [ "${d:-x}" = 0 ] && return 0
  fi
  FAILS+=("reconciliation: $RECON"); return 1
}

report() { # append the section of the scenario to today's report
  local day file status="$1" r
  day=$(date -u +%Y-%m-%d); file="$REPORT_DIR/$day.md"
  mkdir -p "$REPORT_DIR"
  if [ ! -f "$file" ]; then
    {
      echo "# Chaos report $day"
      echo
      echo "> Generated by tests/chaos/run-chaos.sh (F132-AC-14). Environment: \`$ENVNAME\` ($MODE) · commit \`$(git -C "$root" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)\` · backend \`${EW_BACKEND_VERSION:-unknown}\` · runner \`${EW_ACTOR:-${GITHUB_ACTOR:-unknown}}\`."
      echo "> Scenario definitions: [tests/chaos/scenarios](../../../tests/chaos/scenarios). Template and rules: [README](README.md)."
    } > "$file"
  fi
  {
    echo
    echo "## $SC_NAME ($SC_ID) — $status"
    echo
    echo "| Field | Value |"
    echo "|---|---|"
    echo "| Acceptance | $SC_ACCEPTANCE |"
    echo "| Load | $SC_LOAD |"
    echo "| Fault | \`$SC_TYPE\` on \`$SC_TARGET\` for ${SC_DURATION}s |"
    echo "| Injected / recovered (UTC) | $T_START / ${T_END:-n/a} |"
    echo "| Recovery | $RECOVERY |"
    echo "| Reconciliation | $RECON |"
    echo
    echo "| Phase | Expectation | Value | Condition | Result |"
    echo "|---|---|---|---|---|"
    for r in "${ROWS[@]}"; do echo "$r"; done
    echo
    echo "| Expected alert | Seen | Runbook |"
    echo "|---|---|---|"
    for r in "${ALERT_ROWS[@]}"; do echo "$r"; done
    if [ ${#FAILS[@]} -gt 0 ]; then
      echo
      echo "Failures:"
      for r in "${FAILS[@]}"; do echo "- $r"; done
    fi
  } >> "$file"
  log "report: $file ($status)"
}

# shellcheck disable=SC2317 # invoked by the trap
on_signal() { log "interrupted: recovering"; FAILS+=("interrupted"); run_recover || true; RECOVERY="interrupted, recovery attempted"; T_END=$(date -u +%Y-%m-%dT%H:%M:%SZ); report failed; exit 1; }

run_one() {
  FILE="$SCN_DIR/$1.yaml"
  [ -f "$FILE" ] || die "scenario $1 not found in $SCN_DIR"
  local envs; envs=$($NODE "$LIB" env "$FILE" "$TARGET") || die "cannot parse $FILE"
  eval "$envs"
  ROWS=() FAILS=() ALERT_ROWS=() RECOVER=() HEALTH=() RECON="not required" RECOVERY="" T_END=""
  if $DRY; then
    log "dry-run: $SC_NAME ($SC_ID, $SC_ACCEPTANCE) fault=$SC_TYPE target=$SC_TARGET duration=${SC_DURATION}s mode=$MODE env=$ENVNAME"
    return 0
  fi
  local lead=$((SC_DURATION / 2)); [ "$lead" -gt 60 ] && lead=60
  log "scenario $SC_NAME: injecting $SC_TYPE on $SC_TARGET"
  trap on_signal INT TERM
  T_START=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  local injected=true
  inject || { injected=false; FAILS+=("injection failed"); }
  if $injected; then
    sleep $((SC_DURATION - lead))
    check_phase during
    check_alerts
    sleep "$lead"
  fi
  if run_recover && wait_healthy; then
    RECOVERY="clean (all recovery actions applied, services converged)"
  else
    force_recover
    if wait_healthy; then RECOVERY="FORCED (did not converge within ${SC_RECOVERY_TIMEOUT}s)"; else RECOVERY="FAILED (still not converged after force)"; fi
    FAILS+=("recovery needed: $RECOVERY")
  fi
  trap - INT TERM
  T_END=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  if $injected; then
    local settle="$SETTLE"; [ "$settle" -gt "$SC_RECOVERY_TIMEOUT" ] && settle="$SC_RECOVERY_TIMEOUT"
    sleep "$settle"
    check_phase after
    reconcile || true
  fi
  if [ ${#FAILS[@]} -eq 0 ]; then report passed; return 0; fi
  report failed; return 1
}

if [ "$SCENARIO" = all ]; then
  mapfile -t names < <(find "$SCN_DIR" -maxdepth 1 -name '*.yaml' -exec basename {} .yaml \; | sort)
else
  names=("$SCENARIO")
fi
rc=0
for n in "${names[@]}"; do
  # Scenarios needing a target are skipped by `all` unless --target is given (one node, integration or network target).
  if [ "$SCENARIO" = all ] && [ -z "$TARGET" ] && grep -qF "\${TARGET}" "$SCN_DIR/$n.yaml"; then log "skip $n (needs --target)"; continue; fi
  run_one "$n" || rc=1
done
exit $rc
