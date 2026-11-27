#!/usr/bin/env bash
# F134-AC-12: publish-index.sh writes a schema-valid index + RSA-SHA256 signature over signed kits only, and refuses
# an unsigned kit, a tampered kit, a Windows kit or a non-https URL. Uses a throwaway key (TEST KEY, never a release key).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
script="$here/../publish-index.sh"
w=$(mktemp -d)
cleanup() { find "$w" -type f -delete 2>/dev/null || true; find "$w" -depth -type d -exec rmdir {} \; 2>/dev/null || true; }
trap cleanup EXIT
pass=0 fail=0
ok() { echo "ok   - $1"; pass=$((pass + 1)); }
ko() { echo "FAIL - $1"; fail=$((fail + 1)); }

openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$w/key.pem" 2>/dev/null
openssl pkey -in "$w/key.pem" -pubout -out "$w/pub.pem"
mkdir -p "$w/dist"
for b in g1_3 g4_5; do
  head -c 4096 /dev/urandom > "$w/dist/kit-$b.apk"
  openssl dgst -sha256 -sign "$w/key.pem" "$w/dist/kit-$b.apk" | base64 -w0 > "$w/dist/kit-$b.apk.sig"
done
kits() { # $1 = extra JSON for the second kit
  cat > "$w/kits.json" <<EOF
[{"kit_id":"android-g1-3","grade_bands":["g1_3"],"client_version":"0.4.0","min_os":"Android 8.0","file":"$w/dist/kit-g1_3.apk","url":"https://play.example.test/releases/school-kit/0.4.0/kit-g1_3.apk"},
 {"kit_id":"android-g4-5","grade_bands":["g4_5"],"client_version":"0.4.0","min_os":"Android 8.0","file":"$w/dist/kit-g4_5.apk","url":"https://play.example.test/releases/school-kit/0.4.0/kit-g4_5.apk"$1}]
EOF
}
run() { bash "$script" --channel=production --kits="$w/kits.json" --out="$w/out" --signing-key="$w/key.pem" --public-key="$w/pub.pem" >/dev/null 2>&1; }

kits ""
if run && [ -f "$w/out/school-kit/production.json" ] && [ -f "$w/out/school-kit/production.json.sig" ]; then ok "index + signature written"; else ko "index + signature written"; fi
base64 -d < "$w/out/school-kit/production.json.sig" > "$w/sig.bin"
if openssl dgst -sha256 -verify "$w/pub.pem" -signature "$w/sig.bin" "$w/out/school-kit/production.json" >/dev/null 2>&1; then ok "signature verifies with the public key"; else ko "signature verifies"; fi
want=$(sha256sum "$w/dist/kit-g1_3.apk" | cut -d' ' -f1)
if grep -q "\"sha256\":\"$want\"" "$w/out/school-kit/production.json" && grep -q '"platform":"android"' "$w/out/school-kit/production.json" \
  && ! grep -q 'dist/' "$w/out/school-kit/production.json"; then ok "sha256 of the kit, android only, no local path"; else ko "index content"; fi

mv "$w/dist/kit-g4_5.apk.sig" "$w/dist/kit-g4_5.apk.sig.off"
if run; then ko "unsigned kit refused"; else ok "unsigned kit refused"; fi
mv "$w/dist/kit-g4_5.apk.sig.off" "$w/dist/kit-g4_5.apk.sig"
printf 'x' >> "$w/dist/kit-g4_5.apk"
if run; then ko "tampered kit refused"; else ok "tampered kit refused"; fi
openssl dgst -sha256 -sign "$w/key.pem" "$w/dist/kit-g4_5.apk" | base64 -w0 > "$w/dist/kit-g4_5.apk.sig"
kits ',"platform":"windows"'
if run; then ko "windows kit refused (ADR-0101)"; else ok "windows kit refused (ADR-0101)"; fi
sed -i 's#https://play.example.test/releases/school-kit/0.4.0/kit-g4_5.apk#http://play.example.test/kit.apk#' "$w/kits.json"
sed -i 's#,"platform":"windows"##' "$w/kits.json"
if run; then ko "http URL refused"; else ok "http URL refused"; fi
kits ""
if bash "$script" --channel=beta --kits="$w/kits.json" --out="$w/out" --signing-key="$w/key.pem" --public-key="$w/pub.pem" >/dev/null 2>&1; then
  ko "unknown channel refused"; else ok "unknown channel refused"; fi

echo "publish_index_test: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
