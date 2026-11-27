// running-images.sh against a stubbed docker CLI (F132-AC-19): unique digests of the running services, no service → error.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const script = path.join(path.dirname(fileURLToPath(import.meta.url)), 'running-images.sh');

function stub(lsOut, inspectOut) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ri-'));
  const f = path.join(dir, 'docker');
  fs.writeFileSync(f, `#!/usr/bin/env bash\ncase "$1 $2" in\n  "service ls") printf '%b' '${lsOut}' ;;\n  "service inspect") printf '%b' '${inspectOut}' ;;\nesac\n`);
  fs.chmodSync(f, 0o755);
  return f.replace(/\\/g, '/');
}

test('unique image digests of the running services', () => {
  const docker = stub('a\\nb\\nc\\n', 'ghcr.io/o/eduworld/core-api:v1@sha256:aa\\nghcr.io/o/eduworld/core-api:v1@sha256:aa\\npostgres:17@sha256:bb\\n');
  const r = spawnSync('bash', [script, '--env=staging'], { env: { ...process.env, DOCKER: `bash ${docker}` }, encoding: 'utf8' });
  assert.equal(r.status, 0, r.stderr);
  assert.deepEqual(r.stdout.trim().split('\n'), ['ghcr.io/o/eduworld/core-api:v1@sha256:aa', 'postgres:17@sha256:bb']);
});

test('no service or a bad environment fails', () => {
  const docker = stub('', '');
  assert.equal(spawnSync('bash', [script, '--env=staging'], { env: { ...process.env, DOCKER: `bash ${docker}` } }).status, 1);
  assert.equal(spawnSync('bash', [script, '--env=prod'], { env: { ...process.env, DOCKER: `bash ${docker}` } }).status, 2);
});
