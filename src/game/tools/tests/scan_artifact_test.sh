#!/usr/bin/env bash
# F060-AC-03 (TC-AUTH-010) and F060-AC-21: scan_artifact.sh fails on secrets, OIDC keys, a Keycloak token
# endpoint or the mock configuration, and passes on a clean artifact. With GODOT set, also exports the release
# PCK of the real presets (must pass) and a PCK that wrongly includes mock.json (must fail).
# Usage: bash src/game/tools/tests/scan_artifact_test.sh   [GODOT=<godot binary>]
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
game="$(cd "$here/../.." && pwd)"
scan="$game/tools/scan_artifact.sh"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
pass=0
fail=0

expect() { # expect <exit code> <description> <args...>
  local want="$1" desc="$2"
  shift 2
  local got=0
  bash "$scan" "$@" >/dev/null 2>&1 || got=$?
  if [[ "$got" == "$want" ]]; then
    pass=$((pass + 1)); echo "ok   - $desc"
  else
    fail=$((fail + 1)); echo "FAIL - $desc (exit $got, want $want)"
  fi
}

printf 'GDPC....res://scenes/bootstrap/Boot.tscn {"env":"production","api_base_url":"https://api.eduworld.example/api/v1"}' >"$work/clean.pck"
printf 'GDPC....{"env":"production","client_secret": "x"}' >"$work/secret.pck"
printf 'GDPC....{"oidc_issuer": "https://auth.example/realms/eduworld"}' >"$work/oidc.pck"
printf 'GDPC....https://auth.example/realms/eduworld/protocol/openid-connect/token' >"$work/token_url.pck"
printf 'GDPC....res://config/env/mock.json' >"$work/mock_path.pck"
printf 'GDPC....oidc_client_id=eduworld-game' >"$work/oidc_key.pck"
printf 'GDPC....<data android:scheme="eduworld" android:host="oauth2redirect"/>' >"$work/intent.pck"
printf 'GDPC....https://auth.example/protocol/openid-connect/token?redirect_uri=eduworld://cb' >"$work/redirect.pck"
printf 'GDPC....{"env":"mock","is_mock": true}' >"$work/mock_flag.pck"
printf 'GDPC....{"api_base_url":"http://localhost:4011"}' >"$work/mock_url.pck"
(cd "$work" && mkdir -p apk/assets && cp mock_flag.pck apk/assets/game.pck && cd apk && zip -q -r ../mock.apk . ) 2>/dev/null || true

expect 0 "clean artifact passes" "$work/clean.pck"
expect 1 "client secret is found" "$work/secret.pck"
expect 1 "OIDC issuer key is found" "$work/oidc.pck"
expect 1 "Keycloak token endpoint is found" "$work/token_url.pck"
expect 1 "mock.json path is found" "$work/mock_path.pck"
expect 1 "oidc_ key is found (F061-AC-11)" "$work/oidc_key.pck"
expect 1 "OAuth intent filter is found (F061-AC-11)" "$work/intent.pck"
expect 1 "redirect URI / token endpoint is found (F061-AC-11)" "$work/redirect.pck"
expect 1 "is_mock flag is found" "$work/mock_flag.pck"
expect 1 "mock URL is found" "$work/mock_url.pck"
if [[ -f "$work/mock.apk" ]]; then
  expect 1 "mock config inside an APK is found" "$work/mock.apk"
fi
expect 2 "missing artifact is an error" "$work/nope.pck"
# F133: a Web export folder is scanned with its page (index.html), not only the pack.
mkdir -p "$work/web_clean" "$work/web_leak"
cp "$work/clean.pck" "$work/web_clean/index.pck"
printf '<html><script>const GODOT_CONFIG = {"executable":"index"};</script></html>' >"$work/web_clean/index.html"
cp "$work/clean.pck" "$work/web_leak/index.pck"
printf '<html><script>location.href="https://auth.example/protocol/openid-connect/token?redirect_uri=x"</script></html>' >"$work/web_leak/index.html"
expect 0 "clean Web export folder passes (F133)" "$work/web_clean"
expect 1 "OAuth redirect in the Web page is found (F133)" "$work/web_leak"

if [[ -n "${GODOT:-}" ]]; then
  # A Windows Godot started from WSL needs a Windows path; the export path is relative to the project folder.
  game_arg="$game"
  if [[ "$GODOT" == *.exe ]] && command -v wslpath >/dev/null 2>&1; then game_arg="$(wslpath -w "$game")"; fi
  mkdir -p "$game/build/scan_test"
  "$GODOT" --headless --path "$game_arg" --export-pack "Android" build/scan_test/release.pck >"$work/export.log" 2>&1 || true
  mv -f "$game/build/scan_test/release.pck" "$work/release.pck" 2>/dev/null || true
  if [[ -f "$work/release.pck" ]]; then
    expect 0 "release preset PCK is clean" "$work/release.pck"
    contains() { LC_ALL=C grep -a -F -q -- "$1" "$work/release.pck"; }
    for wanted in "config/env/production.json" "scenes/bootstrap/Boot.tscn"; do
      if contains "$wanted"; then pass=$((pass + 1)); echo "ok   - release PCK contains $wanted"
      else fail=$((fail + 1)); echo "FAIL - release PCK lacks $wanted"; fi
    done
    for unwanted in "config/env/local.json" "scenes/ui/debug/UiGallery" "res://tests/" "[!! " "addons/gdUnit4"; do
      if contains "$unwanted"; then fail=$((fail + 1)); echo "FAIL - release PCK ships $unwanted"
      else pass=$((pass + 1)); echo "ok   - release PCK excludes $unwanted"; fi
    done
    # A PCK built from the mock environment must be rejected (simulates a preset that forgot the exclusion).
    cp "$game/config/env/mock.json" "$work/mock.json"
    cat "$work/release.pck" "$work/mock.json" >"$work/leaky.pck"
    expect 1 "release PCK with mock.json content is rejected" "$work/leaky.pck"
  else
    fail=$((fail + 1)); echo "FAIL - export of the release PCK (see log)"; tail -20 "$work/export.log"
  fi
fi

echo "scan_artifact_test: $pass passed, $fail failed"
[[ $fail -eq 0 ]]
