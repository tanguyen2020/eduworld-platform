#!/usr/bin/env bash
# SOPS (age) → versioned Docker secrets (F130-AC-08, AC-10; DEPLOYMENT §13; SECURITY_ARCHITECTURE §6; research R1).
#   secrets-sync.sh --env=<staging|production> [--dry-run] [--keycloak-rotated] [FILE.sops.yaml…]
# Default files: deploy/secrets/<env>/*.sops.yaml (decrypted with `sops -d`, age key from SOPS_AGE_KEY of the GitHub
# Environment; a plain .json file is accepted for tests). For every key (= secret name):
#   - hash = sha256(value); if the latest <name>_v<N> carries the same label ew.sha256 → unchanged;
#   - else create <name>_v<N+1> (labels ew.secret, ew.sha256, ew.env). <name>_v<N> is KEPT: running tasks keep it
#     until the rolling update of the services that reference the name is done (deploy-service.sh resolves the latest
#     version). Old versions are listed (never removed) when no service references them any more.
#   - derived: valkey_acl is rendered from deploy/valkey/users.acl.template + the Valkey passwords (never in SOPS);
#     <binary>_nats_creds are refused when their JWT permissions differ from deploy/nats/permissions.yaml;
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

# 1. Decrypt every file into the private work dir and merge (a key in two files is an error).
i=0
for f in "${files[@]}"; do
  i=$((i + 1))
  case "$f" in
    *.sops.yaml|*.sops.yml) "$SOPS" --decrypt --output-type json "$f" > "$work/plain-$i.json" || die "cannot decrypt $(basename "$f")" ;;
    *.json) cp "$f" "$work/plain-$i.json" ;;
    *) die "unsupported secret file $(basename "$f")" ;;
  esac
done
dups=$(jq -r 'keys[] | select(. != "sops")' "$work"/plain-*.json | sort | uniq -d | tr '\n' ' ')
[ -z "$dups" ] || die "secret key(s) in more than one file: $dups"
jq -s 'add | del(.sops)' "$work"/plain-*.json > "$work/all.json"
rm -f "$work"/plain-*.json

# 2. Derived secrets (F130 T053, DEPLOYMENT §13): valkey_acl is rendered from deploy/valkey/users.acl.template and the
#    passwords (SHA-256 only in the ACL); the NATS creds must carry exactly the permissions of
#    deploy/nats/permissions.yaml (JWTs signed offline by scripts/deploy/nats-accounts.sh).
ACL=(node "$REPO_ROOT/scripts/deploy/render-acl.mjs")
[ "$(jq 'has("valkey_acl")' "$work/all.json")" = false ] || die "valkey_acl is derived from deploy/valkey/users.acl.template: remove it from SOPS"
acl_inputs=$("${ACL[@]}" valkey-inputs) || die "cannot read the Valkey ACL template"
missing_inputs=() present_inputs=0
for k in $acl_inputs; do
  if [ "$(jq --arg k "$k" 'has($k)' "$work/all.json")" = true ]; then present_inputs=$((present_inputs + 1)); else missing_inputs+=("$k"); fi
done
if [ "$present_inputs" -gt 0 ]; then
  [ ${#missing_inputs[@]} -eq 0 ] || die "a Valkey password changed but the ACL cannot be rendered without: ${missing_inputs[*]} (sync every file of $ENV)"
  "${ACL[@]}" valkey --secrets "$work/all.json" > "$work/valkey_acl" || die "cannot render valkey_acl"
  jq --rawfile v "$work/valkey_acl" '.valkey_acl = $v' "$work/all.json" > "$work/all.tmp" && mv "$work/all.tmp" "$work/all.json"
  rm -f "$work/valkey_acl"
  log info "valkey_acl rendered from deploy/valkey/users.acl.template"
fi
"${ACL[@]}" nats-verify --secrets "$work/all.json" || die "NATS creds differ from deploy/nats/permissions.yaml: rerun scripts/deploy/nats-accounts.sh"

# 3. One Docker secret version per changed value.
created=0 unchanged=0
while IFS= read -r name; do
  [[ "$name" =~ ^[a-z][a-z0-9_]{1,62}$ ]] || die "invalid secret name"
  jq -j --arg k "$name" '.[$k]' "$work/all.json" > "$work/value"
  [ -s "$work/value" ] || die "secret $name is empty"
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
done < <(jq -r 'keys[]' "$work/all.json")
rm -f "$work/all.json" "$work/value"

# Versions no service references any more (kept; removal is a separate, manual decision).
in_use=$($DOCKER service ls -q | xargs -r $DOCKER service inspect --format '{{range .Spec.TaskTemplate.ContainerSpec.Secrets}}{{.SecretName}} {{end}}' 2>/dev/null | tr ' ' '\n' | sort -u)
stale=$($DOCKER secret ls --filter label=ew.secret --format '{{.Name}}' | sort | comm -23 - <(printf '%s\n' "$in_use") || true)
[ -n "$stale" ] && log info "unreferenced secret versions (kept): $(echo "$stale" | tr '\n' ' ')"
log info "secrets-sync $ENV: $created created, $unchanged unchanged (dry_run=$DRY_RUN)"
