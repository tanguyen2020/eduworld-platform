#!/usr/bin/env bash
# F001 T032 / F001-AC-13 — prove .gitleaks.toml blocks a real-looking secret and allows only the local
# placeholder. The fake secret is generated at runtime so no secret-like string is ever committed
# (a committed fixture would turn security.yml red on main).
set -euo pipefail
cd "$(dirname "$0")/../.."
IMG="zricethezav/gitleaks:v8.28.0"
root=$(pwd -W 2>/dev/null || pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
wdir=$( (cd "$work" && (pwd -W 2>/dev/null || pwd)) )

scan() { MSYS_NO_PATHCONV=1 docker run --rm -v "$wdir:/scan" -v "$root/.gitleaks.toml:/cfg/.gitleaks.toml:ro" "$IMG" \
  dir /scan --config /cfg/.gitleaks.toml --no-banner --redact >/dev/null 2>&1; }

# 1) placeholder only → clean
printf 'EW_S3_SECRET_KEY=change-me-local\nPOSTGRES_PASSWORD=change-me-local\n' > "$work/ok.env"
if ! scan; then echo "gitleaks: placeholder change-me-local was flagged (allowlist broken)" >&2; exit 1; fi

# 2) generated AWS-style key → must fail
rand() { local set="$1" n="$2" out=""; for ((i = 0; i < n; i++)); do out+="${set:RANDOM % ${#set}:1}"; done; printf %s "$out"; }
key="AKIA$(rand ABCDEFGHIJKLMNOPQRSTUVWXYZ234567 16)"
secret="$(rand ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 40)"
printf 'aws_access_key_id = %s\naws_secret_access_key = %s\n' "$key" "$secret" > "$work/leak.ini"
if scan; then echo "gitleaks: generated secret was NOT detected" >&2; exit 1; fi

echo "gitleaks: ok (secret detected, local placeholder allowed)"
