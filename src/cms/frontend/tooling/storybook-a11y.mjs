// F050-AC-19: serve the static Storybook on 127.0.0.1:6007 (or STORYBOOK_A11Y_PORT), run the test-runner (axe on every
// story, one worker), stop the server, exit with the runner's code. Used by `make web-storybook-check` and web.yml.
// F051: refuses to run when the port already answers — a server left over from an earlier run would be tested
// instead of the fresh build (stories missing → misleading failures).
import { spawn } from 'node:child_process';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const PORT = Number(process.env.STORYBOOK_A11Y_PORT ?? 6007);
const shell = process.platform === 'win32';

async function answers() {
  try {
    return (await fetch(`http://127.0.0.1:${PORT}/index.json`)).ok;
  } catch {
    return false;
  }
}

if (await answers()) {
  console.error(`storybook-a11y: port ${PORT} is already in use (stale server?) — stop it or set STORYBOOK_A11Y_PORT`);
  process.exit(1);
}

// started with node directly (no shell) so that kill() stops the server itself on every OS
const httpServerBin = createRequire(path.join(ROOT, 'package.json')).resolve('http-server/bin/http-server');
const server = spawn(process.execPath, [httpServerBin, 'packages/ops-ui/storybook-static', '-p', String(PORT), '-a', '127.0.0.1', '-s'], { cwd: ROOT, stdio: 'ignore' });

async function waitForServer() {
  for (let i = 0; i < 60; i++) {
    if (await answers()) return;
    await new Promise((r) => setTimeout(r, 1000));
  }
  throw new Error(`Storybook static server did not start on ${PORT}`);
}

let code = 1;
try {
  await waitForServer();
  code = await new Promise((resolve) => {
    const args = ['--filter', '@eduworld/ops-ui', 'exec', 'test-storybook', '--url', `http://127.0.0.1:${PORT}`, '--maxWorkers=1'];
    const runner = spawn('pnpm', args, { cwd: ROOT, stdio: 'inherit', shell });
    runner.on('exit', (c) => resolve(c ?? 1));
  });
} catch (e) {
  console.error(e.message);
} finally {
  server.kill();
}
process.exit(code);
