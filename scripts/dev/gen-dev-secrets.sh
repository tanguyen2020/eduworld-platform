#!/usr/bin/env bash
# Generates the LOCAL development secrets that must never be committed (AGENTS R3/R4) into deploy/compose/secrets/
# (ignored by git through `secrets/` in .gitignore). Idempotent: an existing file is kept. Staging/production get the
# same secrets from SOPS → Docker secrets (DEPLOYMENT §13), never from this script.
#
#   efficacy_pseudonym_key  HMAC-SHA256 key of the pseudonymous participant codes of the efficacy dataset
#                           (F035-AC-33, module learning, EW_EFFICACY_PSEUDONYM_KEY_FILE): 32 random bytes, base64.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DIR="${EW_DEV_SECRETS_DIR:-$ROOT/deploy/compose/secrets}"
mkdir -p "$DIR"
chmod 700 "$DIR" 2>/dev/null || true

gen() { # gen <file> — 32 random bytes in base64, written once
  local f="$DIR/$1"
  if [ -s "$f" ]; then
    echo "gen-dev-secrets: $1 exists, kept"
    return
  fi
  umask 077
  head -c 32 /dev/urandom | base64 | tr -d '\n' > "$f"
  echo "gen-dev-secrets: $1 created"
}

gen efficacy_pseudonym_key
