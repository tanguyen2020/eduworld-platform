#!/usr/bin/env bash
# Builds and signs the school-kit index of one channel (F134-AC-12, DEPLOYMENT §9, specs/schemas/school-kit-index.schema.json):
#   publish-index.sh --channel=<staging|production> --kits=<kits.json> --out=<dir> \
#     --signing-key=<release private key PEM> --public-key=<release public key PEM>
# kits.json (written by the release job of F133) lists the Android school kits (ADR-0101: no Windows installer):
#   [{"kit_id":"android-g1-3-0-4-0","grade_bands":["g1_3"],"client_version":"0.4.0","min_os":"Android 8.0",
#     "file":"dist/eduworld-school-g1_3.apk","url":"https://play.<domain>/releases/school-kit/0.4.0/eduworld-school-g1_3.apk"}]
# Every kit must come with <file>.sig (base64 RSA-SHA256 by the same release key) that verifies: an unsigned or
# tampered kit stops the script (exit 1) — the index only points to signed installers. Output:
#   <out>/school-kit/<channel>.json and <channel>.json.sig (base64 RSA-SHA256 PKCS#1 v1.5 of the exact JSON bytes),
# uploaded next to the release manifest on the CDN by the release job. The keys come from files (secret store in CI,
# R3/R4); nothing secret is written to the output or the log.
set -euo pipefail

CHANNEL="" KITS="" OUT="" KEY="" PUB="" RELEASED_AT="${EW_RELEASED_AT:-}"
for a in "$@"; do
  case "$a" in
    --channel=*) CHANNEL="${a#*=}" ;;
    --kits=*) KITS="${a#*=}" ;;
    --out=*) OUT="${a#*=}" ;;
    --signing-key=*) KEY="${a#*=}" ;;
    --public-key=*) PUB="${a#*=}" ;;
    *) echo "publish-index: unknown argument $a" >&2; exit 1 ;;
  esac
done
die() { echo "publish-index: $*" >&2; exit 1; }
case "$CHANNEL" in staging|production) ;; *) die "--channel must be staging or production" ;; esac
[ -f "$KITS" ] || die "--kits=<kits.json> required"
[ -n "$OUT" ] || die "--out=<dir> required"
[ -f "$KEY" ] || die "--signing-key=<PEM> required"
[ -f "$PUB" ] || die "--public-key=<PEM> required"
command -v openssl >/dev/null || die "openssl required"
command -v node >/dev/null || die "node required"
[ -n "$RELEASED_AT" ] || RELEASED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)

tmp=$(mktemp -d)
cleanup() { rm -f -- "$tmp"/*.bin "$tmp"/lines.tsv; rmdir -- "$tmp" 2>/dev/null || true; }
trap cleanup EXIT

# One line per kit: index, file path.
node -e '
const k = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8"));
if (!Array.isArray(k) || k.length === 0 || k.length > 50) { console.error("kits.json must list 1..50 kits"); process.exit(1); }
k.forEach((x, i) => process.stdout.write(i + "\t" + (x.file || "") + "\n"));' "$KITS" > "$tmp/lines.tsv"

sums=()
while IFS=$'\t' read -r i file; do
  [ -f "$file" ] || die "kit $i: file $file not found"
  [ -f "$file.sig" ] || die "kit $i: $file is not signed ($file.sig missing)"
  base64 -d < "$file.sig" > "$tmp/kit.bin" 2>/dev/null || die "kit $i: $file.sig is not base64"
  openssl dgst -sha256 -verify "$PUB" -signature "$tmp/kit.bin" "$file" >/dev/null 2>&1 || die "kit $i: signature of $file does not verify"
  sha=$(sha256sum "$file" | cut -d' ' -f1)
  size=$(wc -c < "$file" | tr -d ' ')
  sums+=("$i:$sha:$size")
done < "$tmp/lines.tsv"

mkdir -p "$OUT/school-kit"
index="$OUT/school-kit/$CHANNEL.json"
node -e '
const fs = require("fs");
const [kitsFile, channel, released, out, ...sums] = process.argv.slice(1);
const kits = JSON.parse(fs.readFileSync(kitsFile, "utf8"));
const meta = Object.fromEntries(sums.map((s) => { const [i, sha, size] = s.split(":"); return [i, { sha, size: Number(size) }]; }));
const bands = ["g1_3", "g4_5", "g6_7", "g8_9"];
const fail = (m) => { console.error("publish-index: " + m); process.exit(1); };
const index = { schema_version: 1, channel, generated_at: released, kits: kits.map((k, i) => {
  if (!/^[a-z0-9-]{1,64}$/.test(k.kit_id || "")) fail(`kit ${i}: kit_id`);
  if (!/^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$/.test(k.client_version || "") || k.client_version.length > 32) fail(`kit ${i}: client_version`);
  if (!Array.isArray(k.grade_bands) || k.grade_bands.length < 1 || k.grade_bands.some((b) => !bands.includes(b))) fail(`kit ${i}: grade_bands`);
  if (!/^https:\/\/[^\s]+$/.test(k.url || "") || k.url.length > 2048) fail(`kit ${i}: url must be https`);
  if (!k.min_os || k.min_os.length > 32) fail(`kit ${i}: min_os`);
  if (k.platform && k.platform !== "android") fail(`kit ${i}: only android kits (ADR-0101)`);
  return { kit_id: k.kit_id, platform: "android", client_version: k.client_version, grade_bands: [...new Set(k.grade_bands)],
    url: k.url, sha256: meta[i].sha, size_bytes: meta[i].size, min_os: k.min_os, released_at: k.released_at || released };
}) };
fs.writeFileSync(out, JSON.stringify(index));' "$KITS" "$CHANNEL" "$RELEASED_AT" "$index" "${sums[@]}"

# Optional schema check with the shared validator dependencies (npm ci --prefix scripts).
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../../.." && pwd)
if [ -d "$repo/scripts/node_modules/ajv" ]; then
  node -e '
const path = require("path");
const req = require("module").createRequire(path.join(process.argv[1], "scripts", "package.json"));
const Ajv = req("ajv/dist/2020").default, addFormats = req("ajv-formats").default;
const fs = require("fs");
const ajv = new Ajv({ strict: true, allErrors: true }); addFormats(ajv);
const v = ajv.compile(JSON.parse(fs.readFileSync(path.join(process.argv[1], "specs/schemas/school-kit-index.schema.json"), "utf8")));
if (!v(JSON.parse(fs.readFileSync(process.argv[2], "utf8")))) { console.error("publish-index: schema", JSON.stringify(v.errors)); process.exit(1); }' "$repo" "$index"
fi

openssl dgst -sha256 -sign "$KEY" -out "$tmp/index.bin" "$index"
base64 -w0 < "$tmp/index.bin" > "$index.sig"
openssl dgst -sha256 -verify "$PUB" -signature "$tmp/index.bin" "$index" >/dev/null || die "the index signature does not verify with --public-key"
echo "publish-index: $index ($(wc -c < "$index" | tr -d ' ') bytes) + .sig, channel $CHANNEL, ${#sums[@]} kit(s)"
