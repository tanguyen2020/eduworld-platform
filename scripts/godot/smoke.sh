#!/usr/bin/env bash
# Release smoke of the Godot client (F133 T014, F133-AC-06, TC-AUTH-001): blocks publishing when red.
#   smoke.sh --api=<api_base_url, e.g. https://api.staging.<domain>/api/v1> [--godot=<bin>] [--client-version=X.Y.Z]
#            [--platform=android|ios|web]
# 1. public `GET /game/client-config?platform=&version=` (curl): 200, a ClientConfig, and this version not refused;
# 2. headless client (GdUnit integration test tests/integration/release/test_release_smoke.gd): boot → getClientConfig →
#    gameLogin of the QA student → createGameSession → Hub (IN_WORLD).
# The QA student comes from EW_SMOKE_USERNAME / EW_SMOKE_PASSWORD (secrets of the GitHub Environment `staging`; never
# arguments, never printed — R3/R4). Without them step 2 cannot run: exit 3 ("pending"), which fails a CI job.
# Exit 0 green, 1 red, 2 usage, 3 login step not runnable (no QA credentials).
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
API="" PLATFORM="web" VERSION="" GODOT_BIN="${GODOT:-godot}"
for a in "$@"; do
  case "$a" in
    --api=*) API="${a#*=}" ;;
    --godot=*) GODOT_BIN="${a#*=}" ;;
    --client-version=*) VERSION="${a#*=}" ;;
    --platform=*) PLATFORM="${a#*=}" ;;
    *) echo "smoke: unknown argument $a" >&2; exit 2 ;;
  esac
done
[[ "$API" =~ ^https?://[^?#[:space:]]+$ ]] || { echo "smoke: --api=<http(s) api base url> required" >&2; exit 2; }
case "$PLATFORM" in android|ios|web) ;; *) echo "smoke: --platform must be android, ios or web" >&2; exit 2 ;; esac
if [[ -z "$VERSION" ]]; then
  VERSION="$(node -e 'process.stdout.write(require(process.argv[1]).client_version)' "$repo/src/game/config/env/production.json")"
fi
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || { echo "smoke: client version '$VERSION' is not SemVer" >&2; exit 2; }

body="$(mktemp)"
trap 'rm -f -- "$body"' EXIT
code="$(curl -sS -m 20 -o "$body" -w '%{http_code}' "${API%/}/game/client-config?platform=$PLATFORM&version=$VERSION" || echo 000)"
if [[ "$code" != "200" ]]; then
  echo "smoke: getClientConfig returned HTTP $code" >&2
  exit 1
fi
node -e '
const c = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8"));
if (typeof c.min_client_version !== "string" || typeof c.maintenance?.active !== "boolean") { console.error("smoke: not a ClientConfig"); process.exit(1); }
if (c.update_required === true) { console.error(`smoke: the server requires an update of ${process.argv[2]} (min ${c.min_client_version})`); process.exit(1); }
console.log(`smoke: getClientConfig OK (min_client_version ${c.min_client_version}, maintenance ${c.maintenance.active})`);' "$body" "$VERSION"

if [[ -z "${EW_SMOKE_USERNAME:-}" || -z "${EW_SMOKE_PASSWORD:-}" ]]; then
  echo "smoke: PENDING login → session → Hub: EW_SMOKE_USERNAME / EW_SMOKE_PASSWORD (QA student of the environment) not set" >&2
  exit 3
fi
# A fresh checkout needs the import (global class cache) before GdUnit can load the suite.
"$GODOT_BIN" --headless --path "$repo/src/game" --import >/dev/null 2>&1 || true
log="$(mktemp)"
status=0
EDUWORLD_IT_API_BASE_URL="$API" EDUWORLD_IT_CLIENT_VERSION="$VERSION" EDUWORLD_SMOKE_USERNAME="$EW_SMOKE_USERNAME" \
  EDUWORLD_IT_STUDENT_PASSWORD="$EW_SMOKE_PASSWORD" \
  "$GODOT_BIN" --headless --path "$repo/src/game" -s res://addons/gdUnit4/bin/GdUnitCmdTool.gd --ignoreHeadlessMode -c \
  -a res://tests/integration/release/test_release_smoke.gd -rd res://reports/smoke >"$log" 2>&1 || status=$?
grep -E 'Statistics|FAILED|PASSED' "$log" | sed 's/\x1b\[[0-9;]*m//g' || true
if [[ $status -ne 0 ]] || grep -q 'skipped' <(grep 'Statistics' "$log" | sed 's/\x1b\[[0-9;]*m//g' | grep -v ' 0 skipped'); then
  rm -f -- "$log"
  echo "smoke: login → session → Hub FAILED" >&2
  exit 1
fi
rm -f -- "$log"
echo "smoke: boot → client-config → login → session → Hub OK"
