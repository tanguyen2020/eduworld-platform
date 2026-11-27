#!/usr/bin/env bash
# Zone bundles + signed bundle index (F133 T013, F133-AC-05, TC-CNT-009, ADR-0044, DEPLOYMENT §9):
#   build-bundles.sh --list=<bundles.json> --client-version=X.Y.Z --base-url=<https CDN of ew-game-bundles> \
#     --out=<dir> --signing-key=<release private key PEM> --public-key=<release public key PEM> \
#     [--upload=s3://<bucket>]
# Output: <out>/bundles/<sha256>.pck (content-addressed copies) and <out>/index/<client_version>.json + .json.sig
# (base64 RSA-SHA256 PKCS#1 v1.5, sign-manifest.mjs). With --upload, each object goes to the bucket through the AWS
# CLI (`aws s3api`, endpoint from EW_S3_ENDPOINT, credentials from the CI secret env — never arguments): an object that
# already exists is skipped (immutable hash key, no overwrite); an existing index of the same client_version with other
# bytes stops the job. The list holds public zone packs only (no tenant data, question or answer key — R31; the
# scan-artifact step runs on every pack before this). Nothing secret is printed.
set -euo pipefail

LIST="" VERSION="" BASE="" OUT="" KEY="" PUB="" UPLOAD=""
for a in "$@"; do
  case "$a" in
    --list=*) LIST="${a#*=}" ;;
    --client-version=*) VERSION="${a#*=}" ;;
    --base-url=*) BASE="${a#*=}" ;;
    --out=*) OUT="${a#*=}" ;;
    --signing-key=*) KEY="${a#*=}" ;;
    --public-key=*) PUB="${a#*=}" ;;
    --upload=*) UPLOAD="${a#*=}" ;;
    *) echo "build-bundles: unknown argument $a" >&2; exit 2 ;;
  esac
done
die() { echo "build-bundles: $*" >&2; exit 1; }
[ -f "$LIST" ] || die "--list=<bundles.json> required"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || die "--client-version must be SemVer X.Y.Z[-pre]"
[ -n "$OUT" ] || die "--out=<dir> required"
[ -f "$KEY" ] || die "--signing-key=<PEM> required"
[ -f "$PUB" ] || die "--public-key=<PEM> required"
if [ -n "$UPLOAD" ]; then
  [[ "$UPLOAD" =~ ^s3://[a-z0-9.-]{3,63}$ ]] || die "--upload must be s3://<bucket>"
  command -v aws >/dev/null || die "aws CLI required for --upload"
fi
here=$(cd "$(dirname "$0")" && pwd)
released="${EW_RELEASED_AT:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

mkdir -p "$OUT/bundles" "$OUT/index"
index="$OUT/index/$VERSION.json"
copies="$OUT/index/.copies.tsv"
node "$here/lib/bundle-index.mjs" "$LIST" "$VERSION" "$BASE" "$released" "$index" "$copies"

n=0
while IFS=$'\t' read -r file sha; do
  [ -n "$file" ] || continue
  dest="$OUT/bundles/$sha.pck"
  [ -f "$dest" ] || cp "$file" "$dest"
  n=$((n + 1))
done < "$copies"
rm -f -- "$copies"

node "$here/sign-manifest.mjs" sign --in="$index" --key="$KEY" --pub="$PUB" >/dev/null
echo "build-bundles: $index ($n bundle(s)) + .sig"

if [ -n "$UPLOAD" ]; then
  bucket="${UPLOAD#s3://}"
  endpoint=()
  if [ -n "${EW_S3_ENDPOINT:-}" ]; then endpoint=(--endpoint-url "$EW_S3_ENDPOINT"); fi
  exists() { aws s3api head-object --bucket "$bucket" --key "$1" "${endpoint[@]}" >/dev/null 2>&1; }
  put() { # put <key> <file> <cache-control> <content-type>
    aws s3api put-object --bucket "$bucket" --key "$1" --body "$2" --cache-control "$3" --content-type "$4" "${endpoint[@]}" >/dev/null
  }
  immutable="public, max-age=31536000, immutable"
  for f in "$OUT"/bundles/*.pck; do
    [ -f "$f" ] || continue
    key="bundles/$(basename "$f")"
    if exists "$key"; then echo "build-bundles: $key already present (no-op)"; else put "$key" "$f" "$immutable" "application/octet-stream"; echo "build-bundles: uploaded $key"; fi
  done
  key="index/$VERSION.json"
  if exists "$key"; then
    tmp=$(mktemp)
    aws s3api get-object --bucket "$bucket" --key "$key" "${endpoint[@]}" "$tmp" >/dev/null
    if cmp -s "$tmp" "$index"; then rm -f -- "$tmp"; echo "build-bundles: $key already present (no-op)"
    else rm -f -- "$tmp"; die "$key already exists with other content (an index is immutable per client_version)"; fi
  else
    put "$key.sig" "$index.sig" "$immutable" "text/plain"
    put "$key" "$index" "$immutable" "application/json"
    echo "build-bundles: uploaded $key + .sig"
  fi
fi
