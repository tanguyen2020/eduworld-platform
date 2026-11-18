#!/usr/bin/env node
// Writes (or checks) atlas.sum of an Atlas migration directory without the Atlas CLI, with the algorithm
// of ariga.io/atlas migrate.HashFile: per file a running SHA-256 over name + content (base64), and the
// header h1 = SHA-256 over every (name, file hash) pair. Verified identical to `atlas migrate hash`
// on the existing directories; Atlas itself still validates the sum when it applies migrations.
//   node scripts/db/atlas-sum.mjs <migrations dir> [--check]
import { createHash } from 'node:crypto';
import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

export function atlasSum(dir) {
  const files = readdirSync(dir).filter((f) => f.endsWith('.sql')).sort();
  const running = createHash('sha256');
  const pairs = files.map((f) => {
    running.update(f);
    const body = readFileSync(join(dir, f));
    if (/^--\s*atlas:sum\s+ignore\s*$/m.test(body.toString('utf8'))) throw new Error(`${f}: atlas:sum ignore is not supported`);
    running.update(body);
    return [f, running.copy().digest('base64')];
  });
  const total = createHash('sha256');
  for (const [n, h] of pairs) { total.update(n); total.update(h); }
  return `h1:${total.digest('base64')}\n${pairs.map(([n, h]) => `${n} h1:${h}\n`).join('')}`;
}

if (process.argv[1]?.endsWith('atlas-sum.mjs')) {
  const dir = process.argv[2];
  if (!dir) { console.error('usage: atlas-sum.mjs <dir> [--check]'); process.exit(2); }
  const want = atlasSum(dir);
  const file = join(dir, 'atlas.sum');
  if (process.argv.includes('--check')) {
    let cur = '';
    try { cur = readFileSync(file, 'utf8').replace(/\r\n/g, '\n'); } catch { /* missing */ }
    if (cur !== want) { console.error(`stale: ${file}`); process.exit(1); }
    console.log(`${file} ok`);
  } else {
    writeFileSync(file, want);
    console.log(`wrote ${file}`);
  }
}
