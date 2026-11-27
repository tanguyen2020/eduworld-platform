// T010 (F133-AC-05, TC-CNT-009, ADR-0044): build-bundles.sh writes a signed bundle index that follows
// bundle-index.schema.json, stores every pack under its hash key, and an upload of an object that already exists is a
// no-op (never an overwrite); a changed index for the same client_version is refused. The S3 upload is exercised
// against a fake `aws` on PATH that keeps objects in a folder (no network). Throwaway keys only.
// Run: node --test scripts/godot/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { validateDoc, verifyBytes } from '../sign-manifest.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const script = path.join(here, '..', 'build-bundles.sh');
const w = fs.mkdtempSync(path.join(os.tmpdir(), 'bundles-'));
const posix = (p) => p.replaceAll('\\', '/');
const { privateKey, publicKey } = crypto.generateKeyPairSync('rsa', { modulusLength: 3072 });
fs.writeFileSync(path.join(w, 'key.pem'), privateKey.export({ type: 'pkcs8', format: 'pem' }));
fs.writeFileSync(path.join(w, 'pub.pem'), publicKey.export({ type: 'spki', format: 'pem' }));
fs.mkdirSync(path.join(w, 'src'));
fs.writeFileSync(path.join(w, 'src', 'math.pck'), crypto.randomBytes(3000));
fs.writeFileSync(path.join(w, 'src', 'science.pck'), crypto.randomBytes(2000));
const sha = (f) => crypto.createHash('sha256').update(fs.readFileSync(f)).digest('hex');
const list = (extra = []) => {
  fs.writeFileSync(path.join(w, 'bundles.json'), JSON.stringify([
    { bundle_id: 'zone.math_g3', version: 3, grade_bands: ['g1_3'], file: posix(path.join(w, 'src', 'math.pck')) },
    { bundle_id: 'zone.science_g4', version: 1, grade_bands: ['g4_5', 'g6_7'], file: posix(path.join(w, 'src', 'science.pck')) },
    ...extra]));
};

// Fake aws CLI: `s3api head-object|put-object|get-object --bucket B --key K [--body F] [outfile]`, objects in $FAKE_S3.
const bin = path.join(w, 'bin');
fs.mkdirSync(bin);
fs.writeFileSync(path.join(bin, 'aws'), `#!/usr/bin/env bash
echo "aws $*" >> "$FAKE_S3/calls.log"
op="$2"; shift 2; key=""; body=""; out=""
while [ $# -gt 0 ]; do case "$1" in --bucket) shift;; --key) key="$2"; shift;; --body) body="$2"; shift;; --endpoint-url|--content-type|--cache-control) shift;; *) out="$1";; esac; shift; done
obj="$FAKE_S3/objects/$key"
case "$op" in
  head-object) [ -f "$obj" ] || exit 254 ;;
  get-object) cp "$obj" "$out" ;;
  put-object) mkdir -p "$(dirname "$obj")"; cp "$body" "$obj" ;;
esac
`, { mode: 0o755 });
const s3 = path.join(w, 's3');
fs.mkdirSync(path.join(s3, 'objects'), { recursive: true });

const run = (...args) => spawnSync('bash', [script, ...args], { encoding: 'utf8', env: { ...process.env,
  PATH: `${posix(bin)}${path.delimiter}${process.env.PATH}`, FAKE_S3: posix(s3), EW_RELEASED_AT: '2026-11-28T02:00:00Z' } });
const base = () => ['--client-version=0.9.0', '--base-url=https://bundles.example.test', `--list=${posix(path.join(w, 'bundles.json'))}`,
  `--out=${posix(path.join(w, 'out'))}`, `--signing-key=${posix(path.join(w, 'key.pem'))}`, `--public-key=${posix(path.join(w, 'pub.pem'))}`];

test('index follows the schema, keys by hash, carries no local path and its signature verifies', () => {
  list();
  const r = run(...base());
  assert.equal(r.status, 0, r.stderr);
  const file = path.join(w, 'out', 'index', '0.9.0.json');
  const index = JSON.parse(fs.readFileSync(file, 'utf8'));
  assert.deepEqual(validateDoc('bundle-index', index), []);
  const math = index.bundles.find((b) => b.bundle_id === 'zone.math_g3');
  assert.equal(math.sha256, sha(path.join(w, 'src', 'math.pck')));
  assert.equal(math.url, `https://bundles.example.test/bundles/${math.sha256}.pck`);
  assert.ok(fs.existsSync(path.join(w, 'out', 'bundles', `${math.sha256}.pck`)));
  assert.doesNotMatch(fs.readFileSync(file, 'utf8'), /src\/|\.\.\/|tmp/i);
  assert.ok(verifyBytes(fs.readFileSync(file), fs.readFileSync(`${file}.sig`, 'utf8'), fs.readFileSync(path.join(w, 'pub.pem'), 'utf8')));
});

test('upload puts new objects and re-uploading the same build is a no-op (no overwrite)', () => {
  list();
  let r = run(...base(), '--upload=s3://ew-game-bundles');
  assert.equal(r.status, 0, r.stderr);
  const puts1 = fs.readFileSync(path.join(s3, 'calls.log'), 'utf8').split('\n').filter((l) => l.includes('put-object')).length;
  assert.equal(puts1, 4, 'two packs + index + signature');
  fs.writeFileSync(path.join(s3, 'calls.log'), '');
  r = run(...base(), '--upload=s3://ew-game-bundles');
  assert.equal(r.status, 0, r.stderr);
  assert.doesNotMatch(fs.readFileSync(path.join(s3, 'calls.log'), 'utf8'), /put-object/);
  assert.match(r.stdout, /already present/);
});

test('a different index for an already published client_version is refused (immutable)', () => {
  fs.writeFileSync(path.join(w, 'src', 'extra.pck'), crypto.randomBytes(100));
  list([{ bundle_id: 'zone.extra', version: 1, grade_bands: ['g8_9'], file: posix(path.join(w, 'src', 'extra.pck')) }]);
  const r = run(...base(), '--upload=s3://ew-game-bundles');
  assert.equal(r.status, 1);
  assert.match(r.stderr, /index\/0\.9\.0\.json already exists with other content/);
});

test('bad entries are refused: duplicate id, bad band, missing file, non-https base, Windows-style secret path', () => {
  const cases = [
    [{ bundle_id: 'zone.math_g3', version: 1, grade_bands: ['g1_3'], file: posix(path.join(w, 'src', 'math.pck')) }, /duplicate/],
    [{ bundle_id: 'zone.x', version: 1, grade_bands: ['g10'], file: posix(path.join(w, 'src', 'math.pck')) }, /grade_bands/],
    [{ bundle_id: 'zone.y', version: 1, grade_bands: ['g1_3'], file: posix(path.join(w, 'src', 'nope.pck')) }, /not found/],
    [{ bundle_id: 'Zone Y', version: 1, grade_bands: ['g1_3'], file: posix(path.join(w, 'src', 'math.pck')) }, /bundle_id/],
  ];
  for (const [entry, re] of cases) {
    list([entry]);
    const r = run(...base());
    assert.equal(r.status, 1, JSON.stringify(entry));
    assert.match(r.stderr, re);
  }
  list();
  const args = base().map((a) => (a.startsWith('--base-url=') ? '--base-url=http://bundles.example.test' : a));
  assert.equal(run(...args).status, 1);
});

test('an empty list (zones still ship in the core build, OQ-16) gives a valid signed empty index', () => {
  fs.writeFileSync(path.join(w, 'bundles.json'), '[]');
  const out = posix(path.join(w, 'out-empty'));
  const r = run(...base().map((a) => (a.startsWith('--out=') ? `--out=${out}` : a)));
  assert.equal(r.status, 0, r.stderr);
  const index = JSON.parse(fs.readFileSync(path.join(out, 'index', '0.9.0.json'), 'utf8'));
  assert.deepEqual(index.bundles, []);
  assert.ok(fs.existsSync(path.join(out, 'index', '0.9.0.json.sig')));
});

test.after(() => fs.rmSync(w, { recursive: true, force: true }));
