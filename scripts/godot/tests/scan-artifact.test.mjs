// T008 (F133-AC-03, TC-AUTH-010; F133-AC-24/25 through check-secure-storage): scan-artifact.sh exits ≠ 0 on an
// artifact holding the eduworld-game-backend client, a Keycloak token URL, an OAuth intent filter / redirect, a
// private signing key, or question/answer-key content; it also unpacks APK/AAB/IPA and runs the secure storage
// check of the platform. Secret-looking values are generated at run time (nothing secret is committed).
// Run: node --test scripts/godot/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { writeZip } from '../lib/zip.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const script = path.join(here, '..', 'scan-artifact.sh');
const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'scan-'));
const clean = 'GDPC res://scenes/bootstrap/Boot.tscn res://scripts/platform/secure_storage_factory.gd ' +
  'res://scripts/platform/adapters/memory_only_secure_storage.gd {"env":"production","api_base_url":"https://api.eduworld.example/api/v1"}' +
  ' const ANSWER_KEYS = ["answer_key", "scoring_rule"]';
const privPem = crypto.generateKeyPairSync('rsa', { modulusLength: 2048 }).privateKey.export({ type: 'pkcs8', format: 'pem' });

const scan = (...args) => spawnSync('bash', [script, ...args], { encoding: 'utf8' });
const pck = (name, body) => { const f = path.join(dir, name); fs.writeFileSync(f, body); return f; };

test('a clean pack passes (the client only lists answer-key field names to strip them)', () => {
  const r = scan(pck('clean.pck', clean));
  assert.equal(r.status, 0, r.stderr);
});

const leaks = {
  'confidential client id': '{"client":"eduworld-game-backend"}',
  'Keycloak token URL': 'https://auth.example/realms/eduworld/protocol/openid-connect/token',
  'OAuth intent filter': '<data android:scheme="vn.eduworld.game" android:host="oauth2redirect"/>',
  'redirect URI': 'redirect_uri=https://play.example/auth/callback',
  'private signing key': privPem,
  'answer key content': '{"question_id":"q1","answer_key": {"choice": "b"}}',
  'correct option ids': '{"correct_option_ids" : ["a"]}',
  'scoring rule': '{"scoring_rule":{"partial":true}}',
  'accepted answers': '{"accepted_answers":["12"]}',
};
for (const [what, body] of Object.entries(leaks)) {
  test(`${what} in a pack is found (exit 1)`, () => {
    const r = scan(pck(`leak-${what.replace(/\W+/g, '_')}.pck`, `${clean}\n${body}`));
    assert.equal(r.status, 1, `${what}: ${r.stdout}${r.stderr}`);
    assert.match(r.stderr, /FORBIDDEN/);
    assert.doesNotMatch(r.stdout + r.stderr, /BEGIN PRIVATE KEY-----\n[A-Za-z0-9+/]{20}/, 'the secret itself is never printed');
  });
}

test('a leak inside an IPA (ZIP) is found; --platform=web also runs the secure storage check', () => {
  const ipa = path.join(dir, 'EduWorld.ipa');
  fs.writeFileSync(ipa, writeZip([{ name: 'Payload/EduWorld.app/EduWorld.pck', data: Buffer.from(`${clean} ${leaks['answer key content']}`), deflate: true }]));
  assert.equal(scan(ipa).status, 1);
  const web = path.join(dir, 'web');
  fs.mkdirSync(web);
  fs.writeFileSync(path.join(web, 'index.pck'), clean);
  fs.writeFileSync(path.join(web, 'index.html'), '<script>const EduWorldShell=1</script>');
  let r = scan('--platform=web', web);
  assert.equal(r.status, 0, r.stderr);
  fs.writeFileSync(path.join(web, 'index.html'), '<script>localStorage.setItem("s", t)</script>');
  r = scan('--platform=web', web);
  assert.equal(r.status, 1);
  assert.match(r.stderr, /localStorage/);
});

test('usage errors exit 2 (missing artifact, unknown platform, Windows)', () => {
  assert.equal(scan(path.join(dir, 'missing.pck')).status, 2);
  assert.equal(scan('--platform=windows', pck('w.pck', clean)).status, 2);
  assert.equal(scan().status, 2);
});

test.after(() => fs.rmSync(dir, { recursive: true, force: true }));
