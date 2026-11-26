// F130 T052: scripts/deploy/apisix-sync.sh logic with stubbed adc/curl (TEST DOUBLES, R17): lint + diff of both
// gateways against their own Admin API, sync only with the admin key, the key only in the environment (never argv),
// pinned ADC version, sync refused while a canary traffic-split is active. Real gateways:
// deploy/apisix/scripts/validate-adc.sh.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { ROOT, tmpdir, stub, run, hasCmd } from './helpers.mjs';

const skip = hasCmd('node') && hasCmd('sha256sum') ? false : 'bash tooling missing';
const PIN = /^ADC_VERSION=(.+)$/m.exec(fs.readFileSync(path.join(ROOT, 'deploy', 'apisix', 'adc.env'), 'utf8'))[1].trim();

function rig({ version = PIN, split = false } = {}) {
  const stubs = tmpdir('ew-t052-stubs-');
  const dir = tmpdir('ew-t052-');
  const key = path.join(dir, 'admin.key'); fs.writeFileSync(key, 'admin-key-test-value');
  const viewer = path.join(dir, 'viewer.key'); fs.writeFileSync(viewer, 'viewer-key-test-value');
  // adc stub: logs argv and whether ADC_TOKEN reached its environment.
  const adc = stub(stubs, 'adc', `if [ "$1" = --version ]; then echo "${version}"; exit 0; fi\necho "token=\${ADC_TOKEN:+set}" >> "$STUB_LOG"\necho "adc-stub $1 ok"`);
  const routes = split
    ? '{"list":[{"value":{"id":"r1","labels":{"ew-upstream":"core-api"},"plugins":{"traffic-split":{"rules":[]}}}}]}'
    : '{"list":[{"value":{"id":"r1","labels":{"ew-upstream":"core-api"},"plugins":{}}}]}';
  stub(stubs, 'curl', `cat > /dev/null\necho '${routes}'`);
  const env = {
    ADC_BIN: adc, EW_APISIX_ROUTES_ENV_FILE: path.join(ROOT, 'deploy', 'apisix', 'routes', 'env', 'staging.env.example'),
    EW_APISIX_ADMIN_URL: 'http://apisix-admin:9180', EW_APISIX_INTERNAL_ADMIN_URL: 'http://apisix-internal-admin:9180',
    EW_APISIX_ADMIN_KEY_FILE: key, EW_APISIX_VIEWER_KEY_FILE: viewer, EW_AUDIT_FILE: '',
  };
  return { stubs, env, lines: (r) => r.log.split('\n').filter(Boolean) };
}

test('F130 T052: diff lints and diffs both gateways, each against its own Admin API, key only in the environment', { skip }, () => {
  const { stubs, env, lines } = rig();
  const r = run('scripts/deploy/apisix-sync.sh', ['--env=staging', '--mode=diff'], { env, stubs });
  assert.equal(r.code, 0, r.stderr);
  const l = lines(r);
  assert.ok(l.some((x) => /^adc lint -f .*public\.adc\.yaml$/.test(x)));
  assert.ok(l.some((x) => /^adc diff --backend apisix --server http:\/\/apisix-admin:9180 .*public\.adc\.yaml$/.test(x)));
  assert.ok(l.some((x) => /^adc diff --backend apisix --server http:\/\/apisix-internal-admin:9180 .*internal\.adc\.yaml$/.test(x)));
  assert.ok(!l.some((x) => /^adc sync/.test(x)), 'diff never syncs');
  assert.ok(l.filter((x) => x === 'token=set').length >= 2, 'ADC_TOKEN in the environment of adc');
  assert.ok(!r.log.includes('viewer-key-test-value') && !r.log.includes('admin-key-test-value'), 'no key in argv');
  assert.ok(!r.stdout.includes('key-test-value') && !r.stderr.includes('key-test-value'), 'no key in the output');
});

test('F130 T052: sync applies both gateways and writes one audit line per gateway', { skip }, () => {
  const { stubs, env, lines } = rig();
  const r = run('scripts/deploy/apisix-sync.sh', ['--env=production', '--mode=sync'], { env, stubs });
  assert.equal(r.code, 0, r.stderr);
  const l = lines(r);
  assert.ok(l.some((x) => /^adc sync --backend apisix --server http:\/\/apisix-admin:9180 /.test(x)));
  assert.ok(l.some((x) => /^adc sync --backend apisix --server http:\/\/apisix-internal-admin:9180 /.test(x)));
  assert.equal((r.stdout.match(/"action":"apisix_synced"/g) ?? []).length, 2);
});

test('F130 T052: sync refuses while a canary traffic-split is on the public gateway', { skip }, () => {
  const { stubs, env, lines } = rig({ split: true });
  const r = run('scripts/deploy/apisix-sync.sh', ['--env=production', '--mode=sync'], { env, stubs });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /canary is in progress/);
  assert.ok(!lines(r).some((x) => /^adc sync/.test(x)));
  // diff still works during a canary (read only)
  assert.equal(run('scripts/deploy/apisix-sync.sh', ['--env=production', '--mode=diff'], { env, stubs }).code, 0);
});

test('F130 T052: sync needs the admin key; a wrong adc version or a missing routes env is refused', { skip }, () => {
  const a = rig();
  const noKey = run('scripts/deploy/apisix-sync.sh', ['--env=staging', '--mode=sync'], { env: { ...a.env, EW_APISIX_ADMIN_KEY_FILE: '' }, stubs: a.stubs });
  assert.equal(noKey.code, 1); assert.match(noKey.stderr, /EW_APISIX_ADMIN_KEY_FILE/);
  const b = rig({ version: '0.0.1' });
  const ver = run('scripts/deploy/apisix-sync.sh', ['--env=staging', '--mode=diff'], { env: b.env, stubs: b.stubs });
  assert.equal(ver.code, 1); assert.match(ver.stderr, new RegExp(`pins ${PIN.replace(/\./g, '\\.')}`));
  const c = rig();
  const noEnv = run('scripts/deploy/apisix-sync.sh', ['--env=staging', '--mode=diff'], { env: { ...c.env, EW_APISIX_ROUTES_ENV_FILE: '/nonexistent' }, stubs: c.stubs });
  assert.equal(noEnv.code, 1); assert.match(noEnv.stderr, /APISIX_ROUTES_ENV/);
  const badMode = run('scripts/deploy/apisix-sync.sh', ['--env=staging', '--mode=apply'], { env: c.env, stubs: c.stubs });
  assert.equal(badMode.code, 1);
});
