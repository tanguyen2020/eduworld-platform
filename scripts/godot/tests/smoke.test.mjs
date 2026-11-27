// T014 (F133-AC-06, TC-AUTH-001): smoke.sh blocks publishing when getClientConfig fails or refuses this version, and
// when the headless login → session → Hub run fails; without the QA student secrets it stops as "pending" (exit 3,
// which fails a CI job). A local HTTP server stands in for the API and a fake Godot for the headless client.
// Run: node --test scripts/godot/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const script = path.join(here, '..', 'smoke.sh');
const posix = (p) => p.replaceAll('\\', '/');
let reply = { status: 200, body: { min_client_version: '0.1.0', update_required: false, maintenance: { active: false } } };
const seen = [];
const server = http.createServer((req, res) => {
  seen.push({ url: req.url, auth: req.headers.authorization ?? '' });
  res.writeHead(reply.status, { 'content-type': 'application/json' });
  res.end(JSON.stringify(reply.body));
});
await new Promise((r) => server.listen(0, '127.0.0.1', r));
const api = `http://127.0.0.1:${server.address().port}/api/v1`;
const w = fs.mkdtempSync(path.join(os.tmpdir(), 'smoke-'));
const fakeGodot = (ok) => {
  const f = path.join(w, ok ? 'godot-ok' : 'godot-bad');
  fs.writeFileSync(f, `#!/usr/bin/env bash
[ -n "$EDUWORLD_IT_STUDENT_PASSWORD" ] && [ -n "$EDUWORLD_SMOKE_USERNAME" ] || { echo "no creds"; exit 9; }
echo "Statistics: 1 test cases | 0 errors | ${ok ? 0 : 1} failures | 0 flaky | 0 skipped | 0 orphans | ${ok ? 'PASSED' : 'FAILED'}"
exit ${ok ? 0 : 100}
`, { mode: 0o755 });
  return posix(f);
};
const run = (args, env = {}) => new Promise((resolve) => {
  const p = spawn('bash', [script, ...args], { env: { ...process.env, EW_SMOKE_USERNAME: '', EW_SMOKE_PASSWORD: '', ...env } });
  let out = '';
  p.stdout.on('data', (d) => { out += d; });
  p.stderr.on('data', (d) => { out += d; });
  p.on('close', (status) => resolve({ status, out }));
});

test('client-config OK but no QA student secrets → pending (exit 3), public call without token', async () => {
  reply = { status: 200, body: { min_client_version: '0.1.0', update_required: false, maintenance: { active: false } } };
  const r = await run([`--api=${api}`, '--client-version=1.0.0']);
  assert.equal(r.status, 3, r.out);
  assert.match(r.out, /getClientConfig OK/);
  assert.match(r.out, /PENDING/);
  const last = seen.at(-1);
  assert.equal(last.url, '/api/v1/game/client-config?platform=web&version=1.0.0');
  assert.equal(last.auth, '');
});

test('client-config error, a non-ClientConfig body or update_required → red (exit 1)', async () => {
  reply = { status: 503, body: { code: 'UNAVAILABLE' } };
  assert.equal((await run([`--api=${api}`, '--client-version=1.0.0'])).status, 1);
  reply = { status: 200, body: { hello: 'world' } };
  assert.equal((await run([`--api=${api}`, '--client-version=1.0.0'])).status, 1);
  reply = { status: 200, body: { min_client_version: '2.0.0', update_required: true, maintenance: { active: false } } };
  const r = await run([`--api=${api}`, '--client-version=1.0.0']);
  assert.equal(r.status, 1);
  assert.match(r.out, /requires an update/);
});

test('with the QA secrets the headless run decides: passed → 0, failed → 1; the password is never printed', async () => {
  reply = { status: 200, body: { min_client_version: '0.1.0', update_required: false, maintenance: { active: false } } };
  const env = { EW_SMOKE_USERNAME: 'qa-user', EW_SMOKE_PASSWORD: 'pw-only-for-this-test' };
  const ok = await run([`--api=${api}`, '--client-version=1.0.0', `--godot=${fakeGodot(true)}`], env);
  assert.equal(ok.status, 0, ok.out);
  assert.match(ok.out, /Hub OK/);
  const bad = await run([`--api=${api}`, '--client-version=1.0.0', `--godot=${fakeGodot(false)}`], env);
  assert.equal(bad.status, 1);
  assert.doesNotMatch(ok.out + bad.out, /pw-only-for-this-test/);
});

test('usage errors exit 2 (no api, bad version, Windows platform)', async () => {
  assert.equal((await run([])).status, 2);
  assert.equal((await run([`--api=${api}`, '--client-version=1.0'])).status, 2);
  assert.equal((await run([`--api=${api}`, '--platform=windows'])).status, 2);
});

test.after(() => { server.close(); fs.rmSync(w, { recursive: true, force: true }); });
