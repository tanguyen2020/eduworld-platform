#!/usr/bin/env bash
# F123-AC-14 (FR-NTF-003, AGENTS R19): no push channel outside the game. Fails when the backend module graph
# (src/backend/go.mod: every direct and indirect requirement) or the Godot build (src/game: addons, export presets, project, scripts) pulls a push
# SDK/service: Firebase Cloud Messaging, Apple Push Notification service, OneSignal, Web Push (VAPID), Pushy, Pusher Beams,
# Expo push. Notifications reach students only over the in-game realtime connection and the inbox API.
#
# Usage: scripts/ci/check-no-push-sdk.sh [<repo root>]      (default: the repository of this script)
#        scripts/ci/check-no-push-sdk.sh --self-test         (checks that a planted SDK is detected)
set -euo pipefail

PATTERN='firebase|firebase\.google\.com|firebase-admin|cloud\.google\.com/go/messaging|fcm|apns|apns2|sideshow/apns|onesignal|webpush|web-push|vapid|pushy|pusher-beams|pusher/push-notifications|expo-server-sdk|exponent-server-sdk'

scan() {
  local root="$1" found=0
  local targets=()
  for f in "$root/src/backend/go.mod" "$root/src/game/export_presets.cfg" "$root/src/game/project.godot"; do
    [ -f "$f" ] && targets+=("$f")
  done
  [ -d "$root/src/game/addons" ] && targets+=("$root/src/game/addons")
  if [ "${#targets[@]}" -gt 0 ]; then
    # Case-insensitive, whole-word-ish match on module paths, plugin names and preset keys.
    if grep -RInEi --exclude-dir=gdUnit4 -- "(^|[^a-z0-9])($PATTERN)([^a-z0-9]|$)" "${targets[@]}"; then
      found=1
    fi
  fi
  # GDScript / Godot plugin config of the client (not the test framework): no push singleton or plugin.
  if [ -d "$root/src/game" ]; then
    if grep -RInEi --include=*.gd --include=*.cfg --include=*.gdextension --exclude-dir=gdUnit4 --exclude-dir=tests \
        -- "(Engine\.get_singleton\(\"(Firebase|OneSignal|GodotFirebase|PushNotification)|register_for_push|push_token)" "$root/src/game"; then
      found=1
    fi
  fi
  return $found
}

if [ "${1:-}" = "--self-test" ]; then
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT
  mkdir -p "$tmp/src/backend" "$tmp/src/game/addons/plugin"
  printf 'module x\n\ngo 1.24\n\nrequire github.com/google/uuid v1.6.0\n' > "$tmp/src/backend/go.mod"
  if ! scan "$tmp" > /dev/null; then echo "self-test: clean tree reported" >&2; exit 1; fi
  printf 'require firebase.google.com/go/v4 v4.15.0\n' >> "$tmp/src/backend/go.mod"
  if scan "$tmp" > /dev/null; then echo "self-test: firebase not detected" >&2; exit 1; fi
  printf 'module x\n' > "$tmp/src/backend/go.mod"
  printf '[plugin]\nname="OneSignal"\n' > "$tmp/src/game/addons/plugin/plugin.cfg"
  if scan "$tmp" > /dev/null; then echo "self-test: OneSignal addon not detected" >&2; exit 1; fi
  echo "check-no-push-sdk: self-test ok"
  exit 0
fi

root="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
if scan "$root"; then
  echo "check-no-push-sdk: no push SDK in the backend module graph nor in the Godot build (FR-NTF-003)"
else
  echo "check-no-push-sdk: a push SDK/service was found above — push outside the game is forbidden (F123-AC-14, FR-NTF-003, R19)" >&2
  exit 1
fi
