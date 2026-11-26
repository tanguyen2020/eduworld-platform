#!/usr/bin/env bash
# NATS operator-mode credentials from deploy/nats/permissions.yaml (F130 T053, OQ F090-2; DEPLOYMENT §8b, §13).
# Runs OFFLINE on the machine of the DevOps lead holding the nsc store (operator + account signing keys never in Git,
# CI or Swarm). For every user of permissions.yaml it creates the user (or rewrites its permissions in place: same
# nkey), signs a fresh JWT and writes <secret>.creds; it also renders deploy/nats/accounts.template.conf into
# nats_accounts.conf. The outputs are secret values: put them into deploy/secrets/<env>/*.sops.yaml with
# `sops --set` (keys realtime_nats_creds, core_nats_creds, worker_nats_creds, ops_nats_creds, nats_accounts), then
# run secrets-sync (it refuses creds whose JWT permissions differ from permissions.yaml).
#   nats-accounts.sh --store=DIR --out=DIR [--init] [--revoke-previous]
#     --init             create operator EDUWORLD (+ system account SYS) and account EDUWORLD (JetStream enabled) when
#                        the store has none (first setup of an environment; one store per environment);
#     --revoke-previous  revoke every JWT issued before now for the users (after a permission removal): the account JWT
#                        changes, so nats_accounts must be redeployed together with the new creds.
# nsc: $NSC_BIN when set, else the pinned nats-box image (local Docker engine: DOCKER_HOST is ignored).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT="${EW_REPO_ROOT:-$(cd "$here/../.." && pwd)}"
NATS_BOX_IMAGE="${NATS_BOX_IMAGE:-natsio/nats-box:0.16.0}"
OPERATOR=EDUWORLD ACCOUNT=EDUWORLD
STORE="" OUT="" INIT=false REVOKE=false
die() { echo "nats-accounts: $*" >&2; exit 1; }
for arg in "$@"; do
  case "$arg" in
    --store=*) STORE="${arg#*=}" ;;
    --out=*) OUT="${arg#*=}" ;;
    --init) INIT=true ;;
    --revoke-previous) REVOKE=true ;;
    *) die "unknown argument $arg" ;;
  esac
done
[ -n "$STORE" ] && [ -n "$OUT" ] || die "--store=DIR and --out=DIR required"
mkdir -p "$STORE" "$OUT"; chmod 700 "$STORE" "$OUT"
STORE=$(cd "$STORE" && pwd); OUT=$(cd "$OUT" && pwd)
command -v node >/dev/null || die "node is required (scripts/deploy/render-acl.mjs)"
ACL=(node "$REPO_ROOT/scripts/deploy/render-acl.mjs")

# nsc runs with every directory (config, data, keys) inside the store; outputs go to $OUT.
if [ -n "${NSC_BIN:-}" ]; then
  nsc() { "$NSC_BIN" -H "$STORE" "$@" </dev/null; }   # never read the stdin of the user loop
  OUTP="$OUT"
else
  host_path() { if command -v cygpath >/dev/null; then cygpath -w "$1"; else printf '%s' "$1"; fi; }
  nsc() {
    MSYS_NO_PATHCONV=1 env -u DOCKER_HOST -u DOCKER_TLS_VERIFY -u DOCKER_CERT_PATH docker run --rm \
      -v "$(host_path "$STORE"):/nsc" -v "$(host_path "$OUT"):/out" "$NATS_BOX_IMAGE" nsc -H /nsc "$@" </dev/null
  }
  OUTP=/out
fi

if ! nsc describe operator --json >/dev/null 2>&1; then
  [ "$INIT" = true ] || die "no operator in $STORE (first setup: --init)"
  nsc add operator --name "$OPERATOR" --sys >/dev/null 2>&1
  nsc add account --name "$ACCOUNT" >/dev/null 2>&1
  # JetStream limits: unlimited at account level, bounded by the servers (max_file_store, DEPLOYMENT §8b).
  nsc edit account --name "$ACCOUNT" --js-mem-storage -1 --js-disk-storage -1 --js-streams -1 --js-consumer -1 >/dev/null 2>&1
  echo "nats-accounts: created operator $OPERATOR, account $ACCOUNT" >&2
fi

umask 077
# One line per user: name, secret, publish (comma list), subscribe (comma list), allow_responses.
while IFS=$'\t' read -r user secret pub sub resp; do
  [[ "$secret" =~ ^[a-z][a-z0-9_]{1,62}$ ]] || die "invalid secret name for user $user"
  if nsc describe user --account "$ACCOUNT" --name "$user" --json > "$OUT/.user.json" 2>/dev/null; then
    if [ "$REVOKE" = true ]; then nsc revocations add-user --account "$ACCOUNT" --name "$user" >/dev/null 2>&1; fi
    # Rewrite the permissions in place (same nkey): remove every current allow/deny, then add the declared ones.
    old=$("${ACL[@]}" nats-current --user-json "$OUT/.user.json")
    if [ -n "$old" ]; then nsc edit user --account "$ACCOUNT" --name "$user" --rm "$old" >/dev/null 2>&1; fi
    nsc edit user --account "$ACCOUNT" --name "$user" --rm-response-perms >/dev/null 2>&1 || true
  else
    nsc add user --account "$ACCOUNT" --name "$user" >/dev/null 2>&1
  fi
  args=(--account "$ACCOUNT" --name "$user")
  [ -n "$pub" ] && args+=(--allow-pub "$pub")
  [ -n "$sub" ] && args+=(--allow-sub "$sub")
  [ "$resp" = true ] && args+=(--allow-pub-response)
  nsc edit user "${args[@]}" >/dev/null 2>&1 || die "nsc edit user $user failed"
  rm -f "${OUT:?}/${secret:?}.creds"   # nsc refuses to overwrite
  nsc generate creds --account "$ACCOUNT" --name "$user" --output-file "$OUTP/$secret.creds" >/dev/null 2>&1 \
    || die "nsc generate creds $user failed"
  [ -s "$OUT/$secret.creds" ] || die "no creds written for $user"
  echo "nats-accounts: $user → $secret.creds" >&2
done < <("${ACL[@]}" nats-expand --tsv)
rm -f "$OUT/.user.json"

# Resolver config: public operator/account JWTs only.
field() { nsc describe "$@" 2>/dev/null | tr -d '\r'; }
jwt() { field "$@" --raw | grep -v '^-' | tr -d '\n '; }
pubkey() { field "$@" --field sub | tr -d '"\n '; }
printf '{"operator_jwt":"%s","system_account":"%s","system_account_jwt":"%s","account":"%s","account_jwt":"%s"}' \
  "$(jwt operator)" "$(pubkey account --name SYS)" "$(jwt account --name SYS)" "$(pubkey account --name "$ACCOUNT")" \
  "$(jwt account --name "$ACCOUNT")" > "$OUT/.values.json"
"${ACL[@]}" nats-accounts --values "$OUT/.values.json" > "$OUT/nats_accounts.conf" || { rm -f "$OUT/.values.json"; die "cannot render nats_accounts.conf"; }
rm -f "$OUT/.values.json"
# Self-check: the JWTs just signed carry exactly the expanded permissions.
"${ACL[@]}" nats-verify --creds-dir "$OUT" || die "signed JWTs differ from deploy/nats/permissions.yaml"
echo "nats-accounts: wrote $(find "$OUT" -maxdepth 1 -name '*.creds' | wc -l) creds + nats_accounts.conf to $OUT (secret values: sops --set, then shred)" >&2
