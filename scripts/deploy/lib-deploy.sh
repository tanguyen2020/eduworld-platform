#!/usr/bin/env bash
# Shared helpers of the Swarm deploy scripts (F130 US1/US2, DEPLOYMENT §10–§13). Sourced, never executed.
# They run on the self-hosted runner `deploy` with DOCKER_HOST pointing at a Swarm manager over TLS (client cert of CI).
# Every action writes one audit JSON line (actor, env, service, tag, action, reason) to stdout (plan "Ghi nhận").

# shellcheck disable=SC2034 # STACK, BACKEND_SERVICES, MODULES are used by the scripts sourcing this file
STACK="${EW_STACK:-eduworld}"
DOCKER="${DOCKER_BIN:-docker}"
REPO_ROOT="${EW_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
BACKEND_SERVICES="core-api realtime worker-events worker-chat worker-jobs"
MODULES="identity learning game competition chat"

_esc() { local s=${1//\/\\}; s=${s//\"/\\\"}; printf '%s' "${s//$'\n'/ }"; }
log() { printf '{"ts":"%s","level":"%s","script":"%s","msg":"%s"}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" "$(basename "$0")" "$(_esc "$2")" >&2; }
die() { log error "$*"; exit 1; }

# audit_line ACTION [k=v...]
audit_line() {
  local action=$1 kv out; shift
  out="{\"ts\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"audit\":true,\"action\":\"$(_esc "$action")\",\"actor\":\"$(_esc "${EW_ACTOR:-${GITHUB_ACTOR:-${USER:-unknown}}}")\",\"env\":\"$(_esc "${ENV:-}")\""
  for kv in "$@"; do out+=",\"$(_esc "${kv%%=*}")\":\"$(_esc "${kv#*=}")\""; done
  printf '%s}\n' "$out"
  if [ -n "${EW_AUDIT_FILE:-}" ]; then printf '%s}\n' "$out" >> "$EW_AUDIT_FILE"; fi
}

check_env() { [[ "${1:-}" =~ ^(staging|production)$ ]] || die "--env=staging|production required"; }
check_service() { [[ " $BACKEND_SERVICES " == *" ${1:-} "* ]] || die "unknown backend Swarm service '${1:-}' (one of: $BACKEND_SERVICES)"; }
check_version() { [[ "${1:-}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]{1,32})?$ ]] || die "BACKEND_VERSION must be an immutable vX.Y.Z tag (never latest), got '${1:-}'"; }

# require_quorum — fail early when the managers have no quorum (edge case: never deploy partially).
require_quorum() {
  local total reachable
  total=$($DOCKER node ls --filter role=manager --format '{{.ManagerStatus}}' 2>/dev/null | grep -c . || true)
  reachable=$($DOCKER node ls --filter role=manager --format '{{.ManagerStatus}}' 2>/dev/null | grep -cE '^(Leader|Reachable)$' || true)
  [ "$total" -gt 0 ] || die "cannot list Swarm managers: no quorum or no access to the manager API"
  [ $((reachable * 2)) -gt "$total" ] || die "Swarm manager quorum lost ($reachable/$total reachable): not deploying"
}

# latest_secret NAME → NAME_v<N> with the highest N present in the swarm.
latest_secret() {
  local name=$1 best
  best=$($DOCKER secret ls --format '{{.Name}}' | sed -n "s/^${name}_v\([0-9][0-9]*\)$/\1/p" | sort -n | tail -1)
  [ -n "$best" ] || die "secret $name has no version in the swarm (run secrets-sync first)"
  echo "${name}_v$best"
}

# export_secret_versions FILE... — SECRET_VER_<NAME>=<N> for every versioned secret referenced by the stack files.
export_secret_versions() {
  local f var name n
  for var in $(grep -ho 'SECRET_VER_[A-Z0-9_]*' "$@" | sort -u); do
    name=$(printf '%s' "${var#SECRET_VER_}" | tr '[:upper:]' '[:lower:]')
    n=$(latest_secret "$name"); n=${n##*_v}
    export "$var=$n"
  done
}

# export_config_versions FILE... — CFG_<BASENAME>=<sha256[0:12]> of every config file referenced (immutable configs).
export_config_versions() {
  local stack dir file rel var
  for stack in "$@"; do
    dir=$(dirname "$stack")
    while IFS= read -r rel; do
      file="$dir/$rel"
      [ -f "$file" ] || die "config file $rel referenced by $(basename "$stack") is missing"
      var="CFG_$(basename "$rel" | tr '[:lower:].-' '[:upper:]__')"
      export "$var=$(sha256sum "$file" | cut -c1-12)"
    done < <(grep -oE '[{,] *file: *[^,}]+' "$stack" | sed -E 's/^[{,] *file: *//; s/ *$//')
  done
}

# load_stack_env ENV — non-secret deploy variables (GHCR_ORG, OPS_VERSION…) of deploy/swarm/<env>/stack.env.
load_stack_env() {
  local f="$REPO_ROOT/deploy/swarm/$1/stack.env"
  [ -f "$f" ] || die "missing $f"
  set -a; # shellcheck disable=SC1090
  . "$f"; set +a
}

# clone_service_args SPEC_JSON — `docker service create` arguments copying networks, env, secrets, constraints,
# reservations/limits and healthcheck of a service spec (canary, one-off ops jobs).
clone_service_args() {
  local spec=$1 n e name target c
  CLONE_ARGS=()
  while IFS= read -r n; do [ -n "$n" ] && CLONE_ARGS+=(--network "$n"); done < <(jq -r '.TaskTemplate.Networks[]?.Target' <<< "$spec")
  while IFS= read -r e; do [ -n "$e" ] && CLONE_ARGS+=(--env "$e"); done < <(jq -r '.TaskTemplate.ContainerSpec.Env[]?' <<< "$spec")
  while IFS=$'\t' read -r name target; do
    [ -n "$name" ] && CLONE_ARGS+=(--secret "source=$name,target=$target")
  done < <(jq -r '.TaskTemplate.ContainerSpec.Secrets[]? | [.SecretName, .File.Name] | @tsv' <<< "$spec")
  while IFS= read -r c; do [ -n "$c" ] && CLONE_ARGS+=(--constraint "$c"); done < <(jq -r '.TaskTemplate.Placement.Constraints[]?' <<< "$spec")
}

# in_maintenance_window [EPOCH] — production deploy window 22:00–06:00 ICT or weekend (maintenance_window,
# CONFIG_DEFAULTS; DEPLOYMENT §10.3). Returns 0 inside the window.
in_maintenance_window() {
  local now=${1:-${EW_NOW_EPOCH:-$(date +%s)}} hour dow
  hour=$(TZ=Asia/Ho_Chi_Minh date -d "@$now" +%H); dow=$(TZ=Asia/Ho_Chi_Minh date -d "@$now" +%u)
  [ "$dow" -ge 6 ] || [ "$((10#$hour))" -ge 22 ] || [ "$((10#$hour))" -lt 6 ]
}

# image_digest IMAGE:TAG — registry digest of a tag (sha256:…).
image_digest() {
  $DOCKER buildx imagetools inspect "$1" --format '{{json .Manifest}}' | jq -r '.digest'
}
