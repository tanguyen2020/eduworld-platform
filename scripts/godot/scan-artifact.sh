#!/usr/bin/env bash
# CI entry of `godot / scan-artifact` (F133 T012, F133-AC-03, TC-AUTH-010; F133-AC-24/25 through
# check-secure-storage.mjs — ADR-0068, ADR-0086, AGENTS R4, R6, R31):
#   1. the pattern scan of src/game/tools/scan_artifact.sh (one list of forbidden content, R33): the
#      eduworld-game-backend client, a Keycloak token endpoint, an OAuth intent filter / redirect URI, a private
#      signing key, question/answer-key content, the mock configuration — archives (APK, AAB, IPA, ZIP) are
#      unpacked entry by entry;
#   2. with --platform, the secure storage check of that target (Android Keystore plugin + backup exclusion, iOS
#      Keychain adapter + no Keychain Sharing/iCloud, Web MemoryOnly + no browser storage).
# Usage: bash scripts/godot/scan-artifact.sh [--platform=<android|ios|web>] <artifact|dir> [...]
#   → exit 0 clean, 1 forbidden content / secure storage problem, 2 usage error. No Windows target (ADR-0101).
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
platform=""
args=()
for a in "$@"; do
  case "$a" in
    --platform=*) platform="${a#*=}" ;;
    *) args+=("$a") ;;
  esac
done
case "$platform" in
  ""|android|ios|web) ;;
  *) echo "scan-artifact: --platform must be android, ios or web (ADR-0101: no Windows)" >&2; exit 2 ;;
esac
if [[ ${#args[@]} -eq 0 ]]; then
  echo "usage: scan-artifact.sh [--platform=<android|ios|web>] <artifact|dir> [...]" >&2
  exit 2
fi

status=0
bash "$repo/src/game/tools/scan_artifact.sh" "${args[@]}" || status=$?
if [[ $status -eq 2 ]]; then exit 2; fi
if [[ -n "$platform" ]]; then
  for target in "${args[@]}"; do
    s=0
    node "$here/check-secure-storage.mjs" --platform="$platform" "$target" || s=$?
    if [[ $s -eq 2 ]]; then exit 2; fi
    if [[ $s -ne 0 ]]; then status=1; fi
  done
fi
exit "$status"
