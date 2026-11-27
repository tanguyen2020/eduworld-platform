// Tests of the daily rescan issues (F132-AC-19): only CRITICAL with a fix, one issue per CVE+package, due in 72 h.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { criticalWithFix, sync, body, title, FIX_HOURS } from './vuln-issues.mjs';

const trivy = (image, vulns) => ({ ArtifactName: image, Results: [{ Target: image, Vulnerabilities: vulns }] });
const v = (id, sev, fixed, pkg = 'openssl') => ({ VulnerabilityID: id, Severity: sev, PkgName: pkg, InstalledVersion: '1.0', FixedVersion: fixed });

test('only CRITICAL with a fixed version, merged across images', () => {
  const out = criticalWithFix([
    trivy('ghcr.io/x/eduworld/core-api@sha256:aa', [v('CVE-1', 'CRITICAL', '1.1'), v('CVE-2', 'CRITICAL', ''), v('CVE-3', 'HIGH', '2.0')]),
    trivy('ghcr.io/x/eduworld/realtime@sha256:bb', [v('CVE-1', 'CRITICAL', '1.1')]),
  ]);
  assert.equal(out.length, 1);
  assert.equal(out[0].id, 'CVE-1');
  assert.deepEqual(out[0].images, ['ghcr.io/x/eduworld/core-api@sha256:aa', 'ghcr.io/x/eduworld/realtime@sha256:bb']);
});

test('creates a missing issue with the 72 h due date and comments an open one', () => {
  const now = new Date('2026-11-27T00:00:00Z');
  const vulns = criticalWithFix([trivy('img@sha256:aa', [v('CVE-1', 'CRITICAL', '1.1'), v('CVE-9', 'CRITICAL', '3', 'zlib')])]);
  const calls = [];
  const gh = (args) => {
    calls.push(args);
    if (args[1] === 'list') return JSON.stringify([{ number: 7, title: title(vulns[1]) }]);
    return '';
  };
  const actions = sync(vulns, 'staging', now, gh);
  assert.deepEqual(actions.map((a) => a.action), ['created', 'updated']);
  const create = calls.find((c) => c[1] === 'create');
  assert.ok(create.includes('security,cve-critical'));
  assert.match(create[create.indexOf('--body') + 1], /2026-11-30T00:00:00.000Z/);
  assert.equal(FIX_HOURS, 72);
  const comment = calls.find((c) => c[1] === 'comment');
  assert.equal(comment[2], '7');
});

test('body names the environment and the fixed version', () => {
  const b = body({ id: 'CVE-1', pkg: 'openssl', installed: '1.0', fixed: '1.1', images: ['i@sha256:a'] }, 'production', new Date(0));
  assert.match(b, /\*\*production\*\*/);
  assert.match(b, /fixed in 1\.1/);
});
