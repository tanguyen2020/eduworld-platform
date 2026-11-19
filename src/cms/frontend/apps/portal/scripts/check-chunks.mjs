// F050-AC-02: after `vite build`, each of the 5 workspaces is its own dynamic chunk and the entry chunk does
// not import any of them statically (they load only when their route is opened). Reads dist/.vite/manifest.json.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const WORKSPACES = ['game-ops', 'content-studio', 'school-lms', 'platform-admin', 'live-ops'];

/** Returns problems (empty = OK) for a Vite manifest object. */
export function checkManifest(manifest) {
  const problems = [];
  const entries = Object.entries(manifest);
  const entry = entries.find(([, c]) => c.isEntry);
  if (!entry) return ['no entry chunk in the manifest'];
  const staticClosure = new Set();
  const visit = (key) => {
    if (staticClosure.has(key)) return;
    staticClosure.add(key);
    for (const imp of manifest[key]?.imports ?? []) visit(imp);
  };
  visit(entry[0]);
  for (const ws of WORKSPACES) {
    const key = `src/workspaces/${ws}/index.tsx`;
    const chunk = manifest[key];
    if (!chunk) problems.push(`${ws}: no chunk for ${key}`);
    else if (!chunk.isDynamicEntry) problems.push(`${ws}: chunk is not a dynamic entry`);
    else if (!/workspace-/.test(chunk.file)) problems.push(`${ws}: chunk file ${chunk.file} is not named per workspace`);
    if (staticClosure.has(key)) problems.push(`${ws}: statically imported by the entry chunk (loaded on /)`);
  }
  const files = WORKSPACES.map((w) => manifest[`src/workspaces/${w}/index.tsx`]?.file).filter(Boolean);
  if (new Set(files).size !== files.length) problems.push('two workspaces share a chunk');
  return problems;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const file = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'dist', '.vite', 'manifest.json');
  const manifest = JSON.parse(fs.readFileSync(file, 'utf8'));
  const problems = checkManifest(manifest);
  if (problems.length) {
    console.error(`check-chunks: ${problems.length} problem(s)\n  ${problems.join('\n  ')}`);
    process.exit(1);
  }
  for (const w of WORKSPACES) console.log(`  ${w} → ${manifest[`src/workspaces/${w}/index.tsx`].file}`);
  console.log('check-chunks: 5 workspace chunks, none loaded by the entry');
}
