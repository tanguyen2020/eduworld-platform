// Tests of sign-manifest.mjs (F133 T013, F133-AC-05, research R2: RSA-3072, PKCS#1 v1.5 SHA-256, detached base64
// .sig). Throwaway keys are generated in memory (TEST KEY, never a release key). Run: node --test scripts/godot/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { signBytes, verifyBytes, makeReleaseManifest, validateDoc } from '../sign-manifest.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const { privateKey, publicKey } = crypto.generateKeyPairSync('rsa', { modulusLength: 3072 });
const priv = privateKey.export({ type: 'pkcs8', format: 'pem' });
const pub = publicKey.export({ type: 'spki', format: 'pem' });

test('signature is base64 RSA-SHA256 PKCS#1 v1.5 over the exact bytes and openssl-compatible', () => {
  const bytes = Buffer.from('{"schema_version":1}');
  const sig = signBytes(bytes, priv);
  assert.match(sig, /^[A-Za-z0-9+/]+=*$/);
  assert.equal(Buffer.from(sig, 'base64').length, 384);
  assert.ok(verifyBytes(bytes, sig, pub));
  assert.ok(crypto.verify('sha256', bytes, { key: pub, padding: crypto.constants.RSA_PKCS1_PADDING }, Buffer.from(sig, 'base64')));
  assert.ok(!verifyBytes(Buffer.from('{"schema_version":2}'), sig, pub), 'one changed byte must not verify');
});

test('a key shorter than 3072 bits or a non-RSA key is refused', () => {
  const small = crypto.generateKeyPairSync('rsa', { modulusLength: 2048 }).privateKey.export({ type: 'pkcs8', format: 'pem' });
  assert.throws(() => signBytes(Buffer.from('x'), small), /3072/);
  const ec = crypto.generateKeyPairSync('ec', { namedCurve: 'P-256' }).privateKey.export({ type: 'pkcs8', format: 'pem' });
  assert.throws(() => signBytes(Buffer.from('x'), ec), /RSA/);
});

test('release manifest follows client-release-manifest.schema.json and has no Windows platform (ADR-0101)', () => {
  const art = Buffer.from('apk bytes');
  const m = makeReleaseManifest({ platform: 'android', channel: 'staging', clientVersion: '0.9.0-rc1', build: 412,
    url: 'https://play.example.test/releases/android/0.9.0-rc1/eduworld.apk', artifact: art, minOs: 'Android 9',
    releasedAt: '2026-11-28T02:00:00Z' });
  assert.equal(m.sha256, crypto.createHash('sha256').update(art).digest('hex'));
  assert.equal(m.size_bytes, art.length);
  assert.deepEqual(validateDoc('client-release-manifest', m), []);
  assert.throws(() => makeReleaseManifest({ ...m, platform: 'windows', artifact: art, clientVersion: '1.0.0', build: 1,
    minOs: 'x', releasedAt: m.released_at }), /platform/);
  assert.notDeepEqual(validateDoc('client-release-manifest', { ...m, url: 'http://insecure.example/x' }), []);
});

test('CLI: manifest → sign → verify; a tampered file fails verify; no key material in the output', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'signm-'));
  fs.writeFileSync(path.join(dir, 'k.pem'), priv);
  fs.writeFileSync(path.join(dir, 'p.pem'), pub);
  fs.writeFileSync(path.join(dir, 'a.apk'), 'apk');
  const cli = (...a) => spawnSync(process.execPath, [path.join(here, '..', 'sign-manifest.mjs'), ...a], { encoding: 'utf8',
    env: { ...process.env, GITHUB_RUN_NUMBER: '9', EW_RELEASED_AT: '2026-11-28T02:00:00Z' } });
  const out = path.join(dir, 'staging.json');
  let r = cli('manifest', '--platform=android', '--channel=staging', '--client-version=1.0.0', `--artifact=${path.join(dir, 'a.apk')}`,
    '--url=https://play.example.test/releases/android/1.0.0/eduworld.apk', '--min-os=Android 9', `--out=${out}`);
  assert.equal(r.status, 0, r.stderr);
  r = cli('sign', `--in=${out}`, `--key=${path.join(dir, 'k.pem')}`, `--pub=${path.join(dir, 'p.pem')}`);
  assert.equal(r.status, 0, r.stderr);
  assert.ok(fs.existsSync(`${out}.sig`));
  assert.doesNotMatch(r.stdout + r.stderr, /PRIVATE|BEGIN/);
  assert.equal(cli('verify', `--in=${out}`, `--pub=${path.join(dir, 'p.pem')}`).status, 0);
  fs.appendFileSync(out, ' ');
  assert.notEqual(cli('verify', `--in=${out}`, `--pub=${path.join(dir, 'p.pem')}`).status, 0);
  fs.rmSync(dir, { recursive: true, force: true });
});
