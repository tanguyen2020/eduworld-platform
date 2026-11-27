// T034 (F133-AC-17, PERFORMANCE_BUDGET §10.1, ADR-0101): build-kit.sh packs the release APK with the public zone packs
// of the kit's grade bands only, refuses a pack with tenant data / secrets / answer keys, a bad index signature or an
// index of another version, scans the kit (secure storage of Android) and checks its size; the signed kit is listed in
// kits.json that F134's publish-index.sh accepts (F134-AC-12 end to end); the delta against the previous kit is
// reported. Throwaway keys, fake apksigner/zipalign on PATH (the real ones run in the signing job).
// Run: node --test deploy/installer/school-kit/tests/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { readZip, writeZip } from '../../../../scripts/godot/lib/zip.mjs';
import { signBytes, verifyBytes } from '../../../../scripts/godot/sign-manifest.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const script = path.join(here, '..', 'build-kit.sh');
const publish = path.join(here, '..', 'publish-index.sh');
const posix = (p) => p.replaceAll('\\', '/');
const w = fs.mkdtempSync(path.join(os.tmpdir(), 'kit-'));
const p = (...x) => posix(path.join(w, ...x));
const { privateKey, publicKey } = crypto.generateKeyPairSync('rsa', { modulusLength: 3072 });
const priv = privateKey.export({ type: 'pkcs8', format: 'pem' });
const pub = publicKey.export({ type: 'spki', format: 'pem' });
fs.writeFileSync(p('key.pem'), priv);
fs.writeFileSync(p('pub.pem'), pub);

const u16 = (s) => Buffer.from(s, 'utf16le');
fs.writeFileSync(p('base.apk'), writeZip([
  { name: 'AndroidManifest.xml', data: Buffer.concat([u16('dataExtractionRules'), u16('fullBackupContent')]) },
  { name: 'classes.dex', data: Buffer.from('Lvn/eduworld/platform/KeystoreSecureStorage;'), deflate: true },
  { name: 'res/xml/data_extraction_rules.xml', data: Buffer.from('x') },
  { name: 'res/xml/backup_rules.xml', data: Buffer.from('x') },
  { name: 'assets/scripts/platform/secure_storage_factory.gdc', data: Buffer.from('factory') },
  { name: 'META-INF/CERT.RSA', data: Buffer.from('old signature') },
  { name: 'META-INF/MANIFEST.MF', data: Buffer.from('old manifest') },
]));
fs.mkdirSync(p('bundles'));
const packs = {
  'zone.math_g3': { bands: ['g1_3'], data: Buffer.from('PUBLIC ZONE math g1_3 '.repeat(40)) },
  'zone.science_g4': { bands: ['g4_5'], data: Buffer.from('PUBLIC ZONE science g4_5 '.repeat(40)) },
};
const writeIndex = (version = '0.9.0', extra = {}) => {
  const all = { ...packs, ...extra };
  const bundles = Object.entries(all).map(([id, v]) => {
    const sha256 = crypto.createHash('sha256').update(v.data).digest('hex');
    fs.writeFileSync(p('bundles', `${sha256}.pck`), v.data);
    return { bundle_id: id, version: 1, url: `https://bundles.example.test/bundles/${sha256}.pck`, sha256, size_bytes: v.data.length, grade_bands: v.bands };
  });
  const bytes = Buffer.from(JSON.stringify({ schema_version: 1, client_version: version, generated_at: '2026-11-28T02:00:00Z', bundles }));
  fs.writeFileSync(p('index.json'), bytes);
  fs.writeFileSync(p('index.json.sig'), signBytes(bytes, priv));
  return bundles;
};

const bin = p('bin');
fs.mkdirSync(bin);
fs.writeFileSync(path.join(bin, 'zipalign'), '#!/usr/bin/env bash\ncp "${@: -2:1}" "${@: -1}"\n', { mode: 0o755 });
fs.writeFileSync(path.join(bin, 'apksigner'), '#!/usr/bin/env bash\necho "apksigner $1" >> "$FAKE_LOG"\nexit 0\n', { mode: 0o755 });
const env = { ...process.env, PATH: `${bin}${path.delimiter}${process.env.PATH}`, FAKE_LOG: p('signer.log'),
  EW_ANDROID_KEYSTORE_PATH: p('upload.keystore'), EW_ANDROID_KEYSTORE_PASS: 'test-only', EW_ANDROID_KEY_ALIAS: 'upload' };
const args = (out, extra = []) => [`--apk=${p('base.apk')}`, `--index=${p('index.json')}`, `--bundles-dir=${p('bundles')}`,
  '--bands=g1_3', '--client-version=0.9.0', '--url-base=https://play.example.test/releases/school-kit/0.9.0',
  `--public-key=${p('pub.pem')}`, `--release-key=${p('key.pem')}`, `--out=${p(out)}`, ...extra];
const run = (a) => spawnSync('bash', [script, ...a], { encoding: 'utf8', env });

test('unsigned local proof: only the packs of the kit bands, index + signature, old v1 signature removed', () => {
  const bundles = writeIndex();
  const r = run(args('dist-unsigned', ['--unsigned']));
  assert.equal(r.status, 0, r.stderr + r.stdout);
  const kit = readZip(fs.readFileSync(p('dist-unsigned', 'eduworld-school-g1_3-unsigned.apk')));
  const names = kit.map((e) => e.name);
  const math = bundles.find((b) => b.bundle_id === 'zone.math_g3');
  const sci = bundles.find((b) => b.bundle_id === 'zone.science_g4');
  assert.ok(names.includes(`assets/preinstalled/${math.sha256}.pck`));
  assert.ok(!names.includes(`assets/preinstalled/${sci.sha256}.pck`));
  assert.ok(names.includes('assets/preinstalled/index.json') && names.includes('assets/preinstalled/index.json.sig'));
  assert.ok(!names.some((n) => n.startsWith('META-INF/')));
  assert.ok(names.includes('classes.dex'));
  assert.ok(!fs.existsSync(p('dist-unsigned', 'kits.json')), 'an unsigned kit is never listed for publishing');
  assert.match(fs.readFileSync(p('dist-unsigned', 'eduworld-school-g1_3.size.md'), 'utf8'), /android/);
});

test('signed kit → kits.json → F134 publish-index.sh builds a valid signed school-kit index', () => {
  writeIndex();
  const r = run(args('dist'));
  assert.equal(r.status, 0, r.stderr + r.stdout);
  const kitFile = p('dist', 'eduworld-school-g1_3.apk');
  assert.ok(verifyBytes(fs.readFileSync(kitFile), fs.readFileSync(`${kitFile}.sig`, 'utf8'), pub));
  assert.match(fs.readFileSync(p('signer.log'), 'utf8'), /apksigner sign/);
  const kits = JSON.parse(fs.readFileSync(p('dist', 'kits.json'), 'utf8'));
  assert.equal(kits.length, 1);
  assert.deepEqual(kits[0].grade_bands, ['g1_3']);
  const pub2 = spawnSync('bash', [publish, '--channel=staging', `--kits=${p('dist', 'kits.json')}`, `--out=${p('cdn')}`,
    `--signing-key=${p('key.pem')}`, `--public-key=${p('pub.pem')}`], { encoding: 'utf8', env: { ...env, EW_RELEASED_AT: '2026-11-28T02:00:00Z' } });
  assert.equal(pub2.status, 0, pub2.stderr + pub2.stdout);
  const index = JSON.parse(fs.readFileSync(p('cdn', 'school-kit', 'staging.json'), 'utf8'));
  assert.equal(index.kits[0].url, 'https://play.example.test/releases/school-kit/0.9.0/eduworld-school-g1_3.apk');
  assert.equal(index.kits[0].sha256, crypto.createHash('sha256').update(fs.readFileSync(kitFile)).digest('hex'));
});

test('delta against the previous kit: unchanged packs are reported as unchanged', () => {
  writeIndex();
  const r = run(args('dist-delta', ['--unsigned', `--previous=${p('dist', 'eduworld-school-g1_3.packs.json')}`]));
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.stdout, /1 pack\(s\), 1 unchanged, 0 new/);
});

test('a pack with tenant data, a secret or answer keys is refused (F133-AC-17)', () => {
  for (const bad of ['{"tenant_id": "t1"}', '-----BEGIN PRIVATE KEY-----', '{"answer_key": {"a": 1}}']) {
    writeIndex('0.9.0', { 'zone.bad': { bands: ['g1_3'], data: Buffer.from(`PUBLIC ${bad}`) } });
    const r = run(args('dist-bad', ['--unsigned']));
    assert.equal(r.status, 1, bad);
    assert.match(r.stderr, /forbidden content/);
  }
});

test('a tampered index signature or an index of another client version is refused', () => {
  writeIndex();
  fs.appendFileSync(p('index.json'), ' ');
  assert.equal(run(args('dist-x', ['--unsigned'])).status, 1);
  writeIndex('0.9.1');
  const r = run(args('dist-x', ['--unsigned']));
  assert.equal(r.status, 1);
  assert.match(r.stderr, /index is for 0\.9\.1/);
});

test('signed mode without the keystore env or with a Windows band list stops', () => {
  writeIndex();
  const r = spawnSync('bash', [script, ...args('dist-y')], { encoding: 'utf8', env: { ...env, EW_ANDROID_KEYSTORE_PASS: '' } });
  assert.equal(r.status, 1);
  assert.match(r.stderr, /EW_ANDROID_KEYSTORE/);
  assert.equal(run(args('dist-y').map((a) => (a.startsWith('--bands=') ? '--bands=windows' : a))).status, 1);
});

test.after(() => fs.rmSync(w, { recursive: true, force: true }));
