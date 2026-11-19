#!/usr/bin/env bash
# Scans exported game artifacts (.pck, .exe, .apk, .aab, or a folder of them) for content that must never ship
# (F060-AC-03 TC-AUTH-010, F060-AC-21, AGENTS R3/R4/R17, ADR-0068):
#   - client secrets / OIDC configuration keys or a Keycloak token endpoint URL;
#   - the mock configuration (config/env/mock.json, "is_mock": true, the Prism mock port 4011).
# Usage: bash src/game/tools/scan_artifact.sh <artifact|dir> [...]   → exit 1 when anything is found.
set -euo pipefail

PATTERNS=(
  '"client_secret"[[:space:]]*:'
  '"oidc_issuer"[[:space:]]*:'
  '"oidc_client_id"[[:space:]]*:'
  '/realms/[A-Za-z0-9_.-]+/protocol/openid-connect/token'
  'config/env/mock\.json'
  '"is_mock"[[:space:]]*:[[:space:]]*true'
  'localhost:4011'
  '127\.0\.0\.1:4011'
)

if [[ $# -eq 0 ]]; then
  echo "usage: scan_artifact.sh <artifact|dir> [...]" >&2
  exit 2
fi

files=()
for arg in "$@"; do
  if [[ -d "$arg" ]]; then
    while IFS= read -r -d '' f; do files+=("$f"); done < <(find "$arg" -type f \( -name '*.pck' -o -name '*.exe' -o -name '*.apk' -o -name '*.aab' -o -name '*.zip' \) -print0)
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
    *.apk|*.aab|*.zip) unzip -p "$1" 2>/dev/null || true ;;
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
