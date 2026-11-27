#!/usr/bin/env bash
# School APK kit (F133 T039, F133-AC-17, PERFORMANCE_BUDGET §10.1, DEPLOYMENT §9, ADR-0101: Android only, no Windows
# installer). One kit per grade band group: the release APK + the public zone packs of those bands, preinstalled under
# assets/preinstalled/ (read on the device as res://preinstalled, ContentDownloader source `preinstalled`).
#   build-kit.sh --apk=<release APK> --index=<bundle index json> --bundles-dir=<dir of <sha256>.pck> \
#     --bands=g1_3[,g4_5] --client-version=X.Y.Z --url-base=https://play.<domain>/releases/school-kit/<X.Y.Z> \
#     --public-key=<release public key PEM> --release-key=<release private key PEM> --out=<dist> \
#     [--previous=<packs.json of the last kit>] [--unsigned]
# Steps: verify the signed bundle index (+ client_version) → pick the packs of the bands, check SHA-256 and that no
# pack holds tenant data, a secret or question/answer-key content → pack the kit APK → scan it (scan-artifact.sh
# --platform=android: Keystore plugin + backup exclusion) → size gate (check-build-size, `build_size` Android) →
# APK signature with apksigner (keystore from the signing job env: EW_ANDROID_KEYSTORE_PATH, EW_ANDROID_KEYSTORE_PASS,
# EW_ANDROID_KEY_ALIAS — never arguments, R4) → `<kit>.apk.sig` (base64 RSA-SHA256 by the release key, the format
# publish-index.sh of F134 verifies) → append the kit to <out>/kits.json for publish-index.sh.
# --unsigned (local proof only) skips apksigner and the release signature and never writes kits.json.
set -euo pipefail

APK="" INDEX="" BUNDLES="" BANDS="" VERSION="" URL_BASE="" PUB="" RKEY="" OUT="" PREV="" UNSIGNED=0
for a in "$@"; do
  case "$a" in
    --apk=*) APK="${a#*=}" ;;
    --index=*) INDEX="${a#*=}" ;;
    --bundles-dir=*) BUNDLES="${a#*=}" ;;
    --bands=*) BANDS="${a#*=}" ;;
    --client-version=*) VERSION="${a#*=}" ;;
    --url-base=*) URL_BASE="${a#*=}" ;;
    --public-key=*) PUB="${a#*=}" ;;
    --release-key=*) RKEY="${a#*=}" ;;
    --out=*) OUT="${a#*=}" ;;
    --previous=*) PREV="${a#*=}" ;;
    --unsigned) UNSIGNED=1 ;;
    *) echo "build-kit: unknown argument $a" >&2; exit 2 ;;
  esac
done
die() { echo "build-kit: $*" >&2; exit 1; }
[ -f "$APK" ] || die "--apk=<release APK> required"
[ -f "$INDEX" ] && [ -f "$INDEX.sig" ] || die "--index=<bundle index json> with its .sig required"
[ -d "$BUNDLES" ] || die "--bundles-dir=<dir> required"
[[ "$BANDS" =~ ^(g1_3|g4_5|g6_7|g8_9)(,(g1_3|g4_5|g6_7|g8_9)){0,3}$ ]] || die "--bands must list g1_3, g4_5, g6_7, g8_9"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || die "--client-version must be SemVer"
[[ "$URL_BASE" =~ ^https://[^?#[:space:]]+$ ]] || die "--url-base must be an https URL"
[ -f "$PUB" ] || die "--public-key=<PEM> required"
[ -n "$OUT" ] || die "--out=<dir> required"
if [ "$UNSIGNED" -eq 0 ]; then [ -f "$RKEY" ] || die "--release-key=<PEM> required (or --unsigned for a local proof)"; fi

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../../.." && pwd)
tools="$repo/scripts/godot"

node "$tools/sign-manifest.mjs" verify --in="$INDEX" --pub="$PUB" >/dev/null || die "the bundle index signature does not verify"
node -e 'const i = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8")); if (i.client_version !== process.argv[2]) { console.error(`build-kit: index is for ${i.client_version}, not ${process.argv[2]}`); process.exit(1); }' "$INDEX" "$VERSION"

mkdir -p "$OUT"
tag="${BANDS//,/-}"
name="eduworld-school-$tag.apk"
kit="$OUT/$name"
work="$OUT/.$tag.unsigned.apk"
packs="$OUT/eduworld-school-$tag.packs.json"
prev_arg=()
if [ -n "$PREV" ]; then prev_arg=("$PREV"); fi
node "$tools/lib/school-kit.mjs" "$APK" "$INDEX" "$INDEX.sig" "$BUNDLES" "$BANDS" "$work" "$packs" "${prev_arg[@]}"

bash "$tools/scan-artifact.sh" --platform=android "$work"
node "$tools/check-build-size.mjs" --android="$work" --report="$OUT/eduworld-school-$tag.size.md" >/dev/null || die "kit $name is over build_size (PERFORMANCE_BUDGET §11)"

if [ "$UNSIGNED" -eq 1 ]; then
  mv -f "$work" "$OUT/eduworld-school-$tag-unsigned.apk"
  echo "build-kit: $OUT/eduworld-school-$tag-unsigned.apk (UNSIGNED local proof — not publishable, no kits.json)"
  exit 0
fi

command -v apksigner >/dev/null && command -v zipalign >/dev/null || die "apksigner and zipalign (Android build-tools) required"
[ -n "${EW_ANDROID_KEYSTORE_PATH:-}" ] && [ -n "${EW_ANDROID_KEYSTORE_PASS:-}" ] && [ -n "${EW_ANDROID_KEY_ALIAS:-}" ] \
  || die "EW_ANDROID_KEYSTORE_PATH / EW_ANDROID_KEYSTORE_PASS / EW_ANDROID_KEY_ALIAS (signing job env) required"
zipalign -P 16 -f 4 "$work" "$kit"
rm -f -- "$work"
apksigner sign --ks "$EW_ANDROID_KEYSTORE_PATH" --ks-pass env:EW_ANDROID_KEYSTORE_PASS --ks-key-alias "$EW_ANDROID_KEY_ALIAS" "$kit"
apksigner verify "$kit" >/dev/null || die "apksigner verify failed for $name"
node "$tools/sign-manifest.mjs" sign --in="$kit" --key="$RKEY" --pub="$PUB" >/dev/null

node "$tools/lib/kits-list.mjs" add "$OUT/kits.json" "$kit" "$BANDS" "$VERSION" "$URL_BASE"
echo "build-kit: $kit + .sig, listed in $OUT/kits.json"
