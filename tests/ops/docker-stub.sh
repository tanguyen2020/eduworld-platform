#!/usr/bin/env bash
# TEST DOUBLE of the docker CLI for tests/ops (F130, R17): emulates the few Swarm calls of scripts/deploy/* and
# scripts/ops/run-ops-job.sh against files in $STUB_DIR. Every call is appended to $STUB_LOG by the wrapper.
#   STUB_MANAGERS          manager statuses, one per line (default Leader/Reachable/Reachable)
#   STUB_DIR/secrets       existing secret names; STUB_DIR/labels/<name> = ew.sha256 label; STUB_DIR/values/<name> = value
#   STUB_FAIL_MIGRATE      module whose migrate job ends Failed; STUB_HANG_MIGRATE: module whose job never completes
#   STUB_DIGEST_<IMAGE>    registry digest of an image (default sha256:staging-<image>)
#   STUB_STACK_RC          exit code of `stack deploy` (default 0)
set -u
d="${STUB_DIR:?}"
mkdir -p "$d/labels" "$d/values"
touch "$d/secrets"
sub="$1 ${2:-}"
case "$sub" in
  "node ls")
    printf '%b\n' "${STUB_MANAGERS:-Leader\nReachable\nReachable}" ;;
  "secret ls")
    filter=""
    for a in "$@"; do case "$a" in label=ew.secret=*) filter="${a#label=ew.secret=}" ;; esac; done
    if [ -n "$filter" ]; then grep -E "^${filter}_v[0-9]+$" "$d/secrets" || true; else cat "$d/secrets"; fi ;;
  "secret inspect")
    cat "$d/labels/$3" 2>/dev/null || true ;;
  "secret create")
    name="" sha=""
    shift 2
    while [ $# -gt 0 ]; do
      case "$1" in
        --label) case "$2" in ew.sha256=*) sha="${2#ew.sha256=}" ;; esac; shift 2 ;;
        -) shift ;;
        *) name="$1"; shift ;;
      esac
    done
    cat > "$d/values/$name"
    echo "$name" >> "$d/secrets"; printf '%s' "$sha" > "$d/labels/$name"; echo "id-$name" ;;
  "secret rm")
    echo "FORBIDDEN secret rm in tests" >&2; exit 99 ;;
  "service create")
    name=""; prev=""
    for a in "$@"; do [ "$prev" = "--name" ] && name="$a"; prev="$a"; done
    echo "${name:-job-unnamed}"
    echo "$name" >> "$d/created" ;;
  "service ps")
    name="$3"
    state=Complete
    if [ -n "${STUB_FAIL_MIGRATE:-}" ] && [[ "$name" == *"migrate-${STUB_FAIL_MIGRATE}-"* ]]; then state=Failed; fi
    if [ -n "${STUB_HANG_MIGRATE:-}" ] && [[ "$name" == *"migrate-${STUB_HANG_MIGRATE}-"* ]]; then state=Running; fi
    echo "$state 1 second ago" ;;
  "service logs") echo "job log line" ;;
  "service inspect")
    if [[ "$*" == *ContainerSpec.Image* ]]; then echo "${STUB_RUNNING_IMAGE:-ghcr.io/o/eduworld/core-api:v1.0.0@sha256:x}"; exit 0; fi
    if [ -f "$d/spec.json" ]; then cat "$d/spec.json"; else echo '{"TaskTemplate":{"ContainerSpec":{"Image":"ghcr.io/o/eduworld/core-api:v1.0.0@sha256:x","Env":["EW_ENV=staging"],"Secrets":[{"SecretName":"learning_db_dsn_v3","File":{"Name":"learning_db_dsn"}}]},"Networks":[{"Target":"edge"},{"Target":"data"}],"Placement":{"Constraints":["node.labels.role == app"]}}}'; fi ;;
  "service ls") cat "$d/services" 2>/dev/null || true ;;
  "service update"|"service rollback"|"service rm"|"service scale") : ;;
  "stack deploy") exit "${STUB_STACK_RC:-0}" ;;
  "buildx imagetools")
    img="$4"; base="${img##*/}"; base="${base%%:*}"; var="STUB_DIGEST_$(printf '%s' "$base" | tr '[:lower:]-' '[:upper:]_')"
    printf '{"digest":"%s"}\n' "${!var:-sha256:staging-$base}" ;;
  "network inspect") exit 1 ;;
  "network create") : ;;
  *) echo "docker-stub: unhandled: $*" >&2; exit 98 ;;
esac
