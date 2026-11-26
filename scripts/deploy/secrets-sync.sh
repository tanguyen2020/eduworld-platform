#!/usr/bin/env bash
# SOPS (age) → versioned Docker secrets (F130-AC-08, AC-10; DEPLOYMENT §13; SECURITY_ARCHITECTURE §6; research R1).
#   secrets-sync.sh --env=<staging|production> [--dry-run] [--keycloak-rotated] [FILE.sops.yaml…]
# Default files: deploy/secrets/<env>/*.sops.yaml (decrypted with `sops -d`, age key from SOPS_AGE_KEY of the GitHub
# Environment; a plain .json file is accepted for tests). For every key (= secret name):
#   - hash = sha256(value); if the latest <name>_v<N> carries the same label ew.sha256 → unchanged;
#   - else create <name>_v<N+1> (labels ew.secret, ew.sha256, ew.env). <name>_v<N> is KEPT: running tasks keep it
#     until the rolling update of the services that reference the name is done (deploy-service.sh resolves the latest
#     version). Old versions are listed (never removed) when no service references them any more.
#   - Keycloak client secrets (kc_*) change only with --keycloak-rotated: the new secret must already be valid in
#     Keycloak while the old one still is (client secret rotation policy), so both work during the rollout.
# Values never reach argv, logs or the audit line (stdin of `docker secret create`).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=scripts/deploy/lib-deploy.sh
. "$here/lib-deploy.sh"

ENV="" DRY_RUN=false KC_ROTATED=false files=()
for arg in "$@"; do
  case "$arg" in
    --env=*) ENV="${arg#*=}" ;;
    --dry-run) DRY_RUN=true ;;
    --keycloak-rotated) KC_ROTATED=true ;;
    -*) die "unknown argument $arg" ;;
    *) files+=("$arg") ;;
  esac
done
check_env "$ENV"
if [ ${#files[@]} -eq 0 ]; then
  shopt -s nullglob; files=("$REPO_ROOT/deploy/secrets/$ENV/"*.sops.yaml); shopt -u nullglob
fi
[ ${#files[@]} -gt 0 ] || die "no secret file for $ENV"
SOPS="${SOPS_BIN:-sops}"
work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
chmod 700 "$work"

created=0 unchanged=0
for f in "${files[@]}"; do
  case "$f" in
    *.sops.yaml|*.sops.yml) "$SOPS" --decrypt --output-type json "$f" > "$work/plain.json" || die "cannot decrypt $(basename "$f")" ;;
    *.json) cp "$f" "$work/plain.json" ;;
    *) die "unsupported secret file $(basename "$f")" ;;
  esac
  while IFS= read -r name; do
    [[ "$name" =~ ^[a-z][a-z0-9_]{1,62}$ ]] || die "invalid secret name in $(basename "$f")"
    jq -j --arg k "$name" '.[$k]' "$work/plain.json" > "$work/value"
    [ -s "$work/value" ] || die "secret $name is empty in $(basename "$f")"
    hash=$(sha256sum < "$work/value" | cut -d' ' -f1)
    latest=$($DOCKER secret ls --filter "label=ew.secret=$name" --format '{{.Name}}' | sed -n "s/^${name}_v\([0-9][0-9]*\)$/\1/p" | sort -n | tail -1)
    n=${latest:-0}
    if [ "$n" -gt 0 ]; then
      cur=$($DOCKER secret inspect "${name}_v$n" --format '{{index .Spec.Labels "ew.sha256"}}')
      if [ "$cur" = "$hash" ]; then unchanged=$((unchanged + 1)); continue; fi
      if [[ "$name" == kc_* ]] && [ "$KC_ROTATED" = false ]; then
        die "$name changed: set it in Keycloak first with the old secret still valid (rotation policy), then rerun with --keycloak-rotated"
      fi
    fi
    next="${name}_v$((n + 1))"
    if [ "$DRY_RUN" = true ]; then
      log info "would create $next"
    else
      $DOCKER secret create --label "ew.secret=$name" --label "ew.sha256=$hash" --label "ew.env=$ENV" "$next" - < "$work/value" >/dev/null \
        || die "cannot create $next"
      audit_line secret_version_created secret="$name" version="$((n + 1))" previous_kept="$([ "$n" -gt 0 ] && echo "${name}_v$n" || echo none)"
    fi
    created=$((created + 1))
  done < <(jq -r 'keys[] | select(. != "sops")' "$work/plain.json")
  rm -f "$work/plain.json" "$work/value"
done

# Versions no service references any more (kept; removal is a separate, manual decision).
in_use=$($DOCKER service ls -q | xargs -r $DOCKER service inspect --format '{{range .Spec.TaskTemplate.ContainerSpec.Secrets}}{{.SecretName}} {{end}}' 2>/dev/null | tr ' ' '\n' | sort -u)
stale=$($DOCKER secret ls --filter label=ew.secret --format '{{.Name}}' | sort | comm -23 - <(printf '%s\n' "$in_use") || true)
[ -n "$stale" ] && log info "unreferenced secret versions (kept): $(echo "$stale" | tr '\n' ' ')"
log info "secrets-sync $ENV: $created created, $unchanged unchanged (dry_run=$DRY_RUN)"
