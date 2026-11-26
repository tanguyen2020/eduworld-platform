#!/usr/bin/env bash
# Static validation of the Swarm stacks of staging/production (F130-AC-01, AC-07, AC-08, AC-09, AC-24) — job `swarm`
# of .github/workflows/deploy-config.yml and `make swarm-validate`. Needs no swarm:
#   1. `docker stack config` of every stack file (compose schema + interpolation; placeholder values for SECRET_VER_*,
#      CFG_*, BACKEND_VERSION), skipped with a notice when the docker CLI is missing;
#   2. check-stacks.mjs (images, placement, resources, update/rollback, healthcheck, drain, networks, ports);
#   3. check-secret-refs.mjs (secret ownership per module/binary, versioned external secrets, no secret value);
#   4. check-scale-schedule.mjs (schema + cross-field rules), and `ops-scheduler validate` when Go is available.
#   validate-stacks.sh [--env=staging|production] (default: both)
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
envs="staging production"
for arg in "$@"; do
  case "$arg" in
    --env=*) envs="${arg#*=}" ;;
    *) echo "unknown argument $arg" >&2; exit 2 ;;
  esac
done
rc=0
dirs=()
for env in $envs; do
  dir="$root/deploy/swarm/$env"
  [ -d "$dir" ] || { echo "missing $dir" >&2; exit 1; }
  dirs+=("$dir")
  if command -v docker >/dev/null 2>&1 && docker stack config --help >/dev/null 2>&1; then
    for f in "$dir"/*.stack.yml; do
      (
        set -a
        # shellcheck disable=SC1091
        . "$dir/stack.env"
        set +a
        export BACKEND_VERSION=v0.0.0-ci
        for v in $(grep -ho '\${[A-Z][A-Z0-9_]*' "$f" | tr -d '${' | sort -u); do
          [ -n "${!v:-}" ] || export "$v=ci-placeholder"
        done
        for v in $(grep -ho '\${SECRET_VER_[A-Z0-9_]*' "$f" | tr -d '${' | sort -u); do export "$v=1"; done
        cd "$dir" && docker stack config -c "$(basename "$f")" > /dev/null
      ) || { echo "docker stack config failed: $f" >&2; rc=1; }
    done
    echo "docker stack config: $env ok"
  else
    echo "notice: docker CLI with 'stack config' not available, compose schema check skipped" >&2
  fi
done
node "$root/scripts/deploy/check-stacks.mjs" "${dirs[@]}" || rc=1
node "$root/scripts/deploy/check-secret-refs.mjs" "${dirs[@]}" || rc=1
node "$root/scripts/deploy/check-scale-schedule.mjs" "${dirs[@]}" || rc=1
if command -v go >/dev/null 2>&1 && [ "${EW_SKIP_GO:-}" != 1 ]; then
  for d in "${dirs[@]}"; do
    stacks=$(printf '%s,' "$d"/*.stack.yml); stacks=${stacks%,}
    (cd "$root/src/backend" && go run ./tools/ops-scheduler/cmd/ops-scheduler validate --stacks="$stacks" "$d/scale-schedule.yaml") || rc=1
  done
fi
exit "$rc"
