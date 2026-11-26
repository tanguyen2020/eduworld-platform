// Test helpers of tests/ops (F130): temporary dirs, executable stubs of external CLIs (docker, pgbackrest, rclone, nats,
// curl, etcdctl…) and a runner for the bash scripts. TEST DOUBLES only (R17): the stubs never reach a real system.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

export const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');

export function tmpdir(prefix = 'ew-f130-') {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), prefix));
  return d;
}

/** Writes an executable bash stub `name` into dir (logs its argv to $STUB_LOG, one line per call). */
export function stub(dir, name, body) {
  const file = path.join(dir, name);
  fs.writeFileSync(file, `#!/usr/bin/env bash\nprintf '%s %s\\n' "${name}" "$*" >> "\${STUB_LOG:-/dev/null}"\n${body}\n`, { mode: 0o755 });
  return file;
}

/** Runs a bash script of the repo with PATH = stubs first. Returns {code, stdout, stderr, log}. */
export function run(script, args = [], { env = {}, stubs, cwd = ROOT, input } = {}) {
  const log = path.join(stubs ?? tmpdir(), 'calls.log');
  fs.writeFileSync(log, ''); // one log per run
  const sep = process.platform === 'win32' ? ';' : ':';
  const r = spawnSync('bash', [path.join(ROOT, script), ...args], {
    cwd, input, encoding: 'utf8',
    env: { ...process.env, PATH: `${stubs ? stubs + sep : ''}${process.env.PATH}`, STUB_LOG: log, ...env },
  });
  return { code: r.status, stdout: r.stdout ?? '', stderr: r.stderr ?? '', log: fs.readFileSync(log, 'utf8') };
}

/** Copies the deploy/ tree needed by the Swarm checks into a temp root; returns {root, env(name)}. */
export function copyDeploy() {
  const root = tmpdir('ew-f130-deploy-');
  for (const d of ['deploy/swarm', 'deploy/postgres', 'deploy/backup', 'deploy/ops-scheduler']) {
    fs.cpSync(path.join(ROOT, d), path.join(root, d), { recursive: true });
  }
  return { root, env: (e) => path.join(root, 'deploy', 'swarm', e) };
}

/** Replaces text in a file (throws when absent, so a test never passes on a no-op mutation). */
export function mutate(file, from, to) {
  const s = fs.readFileSync(file, 'utf8');
  if (!s.includes(from)) throw new Error(`mutation anchor not found in ${file}: ${from.slice(0, 60)}`);
  fs.writeFileSync(file, s.replace(from, () => to));
}

export const hasCmd = (cmd) => spawnSync('bash', ['-c', `command -v ${cmd}`], { encoding: 'utf8' }).status === 0;
