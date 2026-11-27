#!/usr/bin/env bash
# Exports one release target of the Godot client (F133 T011, F133-AC-01, F133-AC-02, GODOT_ARCHITECTURE §11):
#   export.sh <android|ios|web> [--debug] [--godot=<bin>] [--project=<dir>] [--skip-scan]
# Targets (ADR-0101: Web, Android, iOS only — no Windows/macOS/Linux):
#   android → build/android/EduWorld.apk (MDM) and EduWorld.aab (Play), preset "Android" (export_format 0 then 1).
#             A release export is signed by Godot from GODOT_ANDROID_KEYSTORE_RELEASE_{PATH,USER,PASSWORD} (secret env
#             of the signing job, never in export_presets.cfg — R4); without them use --debug (debug keystore).
#   ios     → build/ios/EduWorld.ipa (Xcode project + IPA; needs Xcode, so the macOS runner only — exit 3 elsewhere).
#   web     → build/web/index.html (+ wasm/pck/js), the shell stylesheet next to it, check_web_export.sh.
# Before the export, scripts/godot/gen-config.mjs stamps client_version (client/vX.Y.Z tag, else 0.0.0-ci.<run>) and
# build_number (github.run_number) into config/env/production.json — the only env file the presets pack. On a tag
# the URLs and the manifest public key come from the GitHub Environment (EW_GAME_*); otherwise the committed URLs are
# kept. The committed file (and the preset) are restored on exit, so a local run leaves the tree clean.
# Then scan-artifact.sh --platform=<target> runs on the output (skip with --skip-scan in unit tests only).
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
target="" mode="--export-release" project="$repo/src/game" scan=1
GODOT_BIN="${GODOT:-godot}"
for a in "$@"; do
  case "$a" in
    android|ios|web) target="$a" ;;
    --debug) mode="--export-debug" ;;
    --godot=*) GODOT_BIN="${a#*=}" ;;
    --project=*) project="${a#*=}" ;;
    --skip-scan) scan=0 ;;
    *) echo "export: unknown argument or target '$a' (targets: android, ios, web — ADR-0101: no Windows)" >&2; exit 2 ;;
  esac
done
[[ -n "$target" ]] || { echo "usage: export.sh <android|ios|web> [--debug] [--godot=<bin>]" >&2; exit 2; }

if [[ "$target" == "ios" && "${EW_FORCE_UNAME:-$(uname -s)}" != "Darwin" ]]; then
  echo "export: the iOS IPA needs Xcode on the macOS runner (godot / export-ios; pending F133 T019 locally)" >&2
  exit 3
fi
if [[ "$target" == "android" && "$mode" == "--export-release" && -z "${GODOT_ANDROID_KEYSTORE_RELEASE_PATH:-}" ]]; then
  echo "export: an Android release export needs GODOT_ANDROID_KEYSTORE_RELEASE_PATH/_USER/_PASSWORD from the signing job (or --debug)" >&2
  exit 1
fi

cfg="$project/config/env/production.json"
presets="$project/export_presets.cfg"
backup="$(mktemp -d)"
cp "$cfg" "$backup/production.json"
cp "$presets" "$backup/export_presets.cfg"
restore() {
  cp "$backup/production.json" "$cfg"
  cp "$backup/export_presets.cfg" "$presets"
  rm -f -- "$backup/production.json" "$backup/export_presets.cfg"
  rmdir -- "$backup" 2>/dev/null || true
}
trap restore EXIT

gen=(node "$here/gen-config.mjs" --env=production --out="$cfg")
if [[ "${GITHUB_REF:-}" != refs/tags/client/v* ]]; then gen+=(--keep-urls); fi
"${gen[@]}"
version="$(node -e 'process.stdout.write(require(process.argv[1]).client_version)' "$cfg")"
build="$(node -e 'process.stdout.write(String(require(process.argv[1]).build_number ?? 0))' "$cfg")"

godot() { "$GODOT_BIN" --headless --path "$project" "$@"; }
out_dir="$project/build/$target"
mkdir -p "$out_dir"
case "$target" in
  android)
    if [[ ! -d "$project/android/build" ]]; then godot --install-android-build-template; fi
    sed -i 's/^gradle_build\/export_format=.*/gradle_build\/export_format=0/' "$presets"
    godot "$mode" "Android" build/android/EduWorld.apk
    sed -i 's/^gradle_build\/export_format=.*/gradle_build\/export_format=1/' "$presets"
    godot "$mode" "Android" build/android/EduWorld.aab
    ;;
  ios)
    godot "$mode" "iOS" build/ios/EduWorld.ipa
    ;;
  web)
    godot "$mode" "Web" build/web/index.html
    cp "$project/web/shell_tokens.css" "$out_dir/"
    bash "$repo/src/game/tools/ci/check_web_export.sh" "$out_dir"
    ;;
esac
printf '{"platform":"%s","client_version":"%s","build_number":%s}\n' "$target" "$version" "$build" >"$out_dir/build-info.json"
echo "export: $target $version (build $build) → $out_dir"
if [[ $scan -eq 1 ]]; then
  bash "$here/scan-artifact.sh" --platform="$target" "$out_dir"
fi
