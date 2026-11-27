#!/usr/bin/env bash
# Scans exported game artifacts (.pck, .apk, .aab, .ipa, .zip, the Web page .html, or a folder of them — Android, iOS, Web;
# no Windows .exe since ADR-0101) for content that must never ship
# (F060-AC-03 TC-AUTH-010, F060-AC-21, AGENTS R3/R4/R17, ADR-0068):
#   - client secrets / OIDC configuration keys or a Keycloak token endpoint URL;
#   - an OAuth callback (redirect URI / AppAuth intent filter) — the game logs in through its own form (F061-AC-11);
#   - the mock configuration (config/env/mock.json, "is_mock": true, the Prism mock port 4011);
#   - F133-AC-03 (AGENTS R4, R6, R31): the confidential client eduworld-game-backend, a private signing key (PEM), and
#     question/answer-key content (JSON fields answer_key, scoring_rule, correct_option_id(s), accepted_answers with a
#     value — the client only lists these names to strip them, so a name followed by ':' means shipped content).
# CI calls it through scripts/godot/scan-artifact.sh, which adds the secure storage check per platform.
# Usage: bash src/game/tools/scan_artifact.sh <artifact|dir> [...]   → exit 1 when anything is found.
set -euo pipefail

PATTERNS=(
  '"client_secret"[[:space:]]*:'
  '"oidc_issuer"[[:space:]]*:'
  '"oidc_client_id"[[:space:]]*:'
  '/realms/[A-Za-z0-9_.-]+/protocol/openid-connect/token'
  '/protocol/openid-connect/token'
  'oidc_(issuer|client_id|redirect)'
  'redirect_uri'
  'oauth2redirect'
  'net.openid.appauth'
  'config/env/mock\.json'
  '"is_mock"[[:space:]]*:[[:space:]]*true'
  'localhost:4011'
  '127\.0\.0\.1:4011'
  'eduworld-game-backend'
  '-----BEGIN ([A-Z0-9]+ )?PRIVATE KEY-----'
  '"(answer_key|scoring_rule|accepted_answers)"[[:space:]]*:'
  '"correct_option_ids?"[[:space:]]*:'
)

if [[ $# -eq 0 ]]; then
  echo "usage: scan_artifact.sh <artifact|dir> [...]" >&2
  exit 2
fi

files=()
for arg in "$@"; do
  if [[ -d "$arg" ]]; then
    while IFS= read -r -d '' f; do files+=("$f"); done < <(find "$arg" -type f \( -name '*.pck' -o -name '*.apk' -o -name '*.aab' -o -name '*.ipa' -o -name '*.zip' -o -name '*.html' \) -print0)
  elif [[ -f "$arg" ]]; then
    files+=("$arg")
  else
    echo "scan_artifact: $arg not found" >&2
    exit 2
  fi
done
if [[ ${#files[@]} -eq 0 ]]; then
  echo "scan_artifact: no artifact to scan" >&2
  exit 2
fi

# Archives (APK/AAB/ZIP) are scanned entry by entry after decompression; other files as raw bytes.
content() {
  case "$1" in
    *.apk|*.aab|*.ipa|*.zip) unzip -p "$1" 2>/dev/null || true ;;
    *) cat "$1" ;;
  esac
}

found=0
for f in "${files[@]}"; do
  for p in "${PATTERNS[@]}"; do
    if content "$f" | LC_ALL=C grep -a -E -q -- "$p"; then
      echo "scan_artifact: FORBIDDEN '$p' in $f" >&2
      found=1
    fi
  done
done

if [[ $found -ne 0 ]]; then
  exit 1
fi
echo "scan_artifact: OK (${#files[@]} artifact(s), ${#PATTERNS[@]} patterns)"
