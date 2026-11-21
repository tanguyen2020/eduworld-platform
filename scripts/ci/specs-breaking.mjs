#!/usr/bin/env node
// F001 T038 / ADR-0095 mục 7 — `make specs-breaking`: oasdiff breaking-change check of every public and internal OpenAPI
// file against a base ref (default origin/main). New files are skipped (nothing to break). Specs are normalized for the
// OpenAPI 3.0 loader of oasdiff (numeric exclusive bounds, see breaking-allowlist.mjs) before the diff. A breaking change
// passes only when specs/breaking-allowlist.yaml lists it exactly with its ADR; the check itself is never disabled.
//   node scripts/ci/specs-breaking.mjs [base-ref]          (needs docker and `npm ci --prefix scripts`)
//   OASDIFF="oasdiff" node scripts/ci/specs-breaking.mjs   (a local oasdiff binary instead of the container)
//   node scripts/ci/specs-breaking.mjs <base> --suggest    (also prints the rejected changes as allowlist entries to review:
//                                                          adr/reason are left empty, the check refuses them until filled)
import { execFileSync, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { classify, describe, normalizeFor30, validateAllowlist } from './breaking-allowlist.mjs';

const require = createRequire(import.meta.url);
const yaml = require('js-yaml');
const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const cli = process.argv.slice(2);
const suggest = cli.includes('--suggest');
const base = cli.find((a) => !a.startsWith('--')) || 'origin/main';
const suggestions = new Map();
const IMG = 'tufin/oasdiff:v1.11.7';
const ALLOWLIST = 'specs/breaking-allowlist.yaml';

const adrExists = (adr) => fs.readdirSync(path.join(repo, 'docs/adr')).some((f) => f.startsWith(`${adr}-`));
const allow = yaml.load(fs.readFileSync(path.join(repo, ALLOWLIST), 'utf8')) ?? {};
const entries = allow.exceptions ?? [];
const problems = validateAllowlist(entries, adrExists);
if (problems.length) {
  problems.forEach((p) => console.error(`${ALLOWLIST}: ${p}`));
  process.exit(1);
}

const git = (...args) => execFileSync('git', args, { cwd: repo, encoding: 'utf8', maxBuffer: 256 * 1024 * 1024, stdio: ['ignore', 'pipe', 'ignore'] });
const gitShow = (ref, file) => {
  try {
    return git('show', `${ref}:${file}`);
  } catch {
    return null;
  }
};

// Both trees are written normalized under <work>/base and <work>/rev with their repository layout, so the relative
// $refs of the internal specs (common/internal-common.v1.yaml) resolve against the same version.
const work = fs.mkdtempSync(path.join(os.tmpdir(), 'specs-breaking-'));
const put = (tree, file, text) => {
  const out = path.join(work, tree, file);
  fs.mkdirSync(path.dirname(out), { recursive: true });
  fs.writeFileSync(out, yaml.dump(normalizeFor30(yaml.load(text)), { noRefs: true, lineWidth: -1 }));
};
const listRev = (dir) => fs.readdirSync(path.join(repo, dir), { withFileTypes: true }).flatMap((d) =>
  d.isDirectory() ? listRev(`${dir}/${d.name}`) : d.name.endsWith('.yaml') ? [`${dir}/${d.name}`] : []);
let baseFiles = [];
try {
  baseFiles = git('ls-tree', '-r', '--name-only', base, '--', 'specs/openapi').split('\n').filter((f) => f.endsWith('.yaml'));
} catch {
  console.error(`unknown base ref ${base}`);
  process.exit(1);
}
const files = ['specs/openapi/eduworld-api.v1.yaml', ...fs.readdirSync(path.join(repo, 'specs/openapi/modules'))
  .filter((f) => f.endsWith('.yaml')).sort().map((f) => `specs/openapi/modules/${f}`)];

function oasdiff(baseFile, revFile) {
  const local = process.env.OASDIFF;
  const args = ['breaking', baseFile, revFile, '--format', 'json'];
  const r = local
    ? spawnSync(local, args.map((a) => a.replace(/^\/w\//, `${work}/`)), { encoding: 'utf8', maxBuffer: 256 * 1024 * 1024 })
    : spawnSync('docker', ['run', '--rm', '-v', `${work.split(path.sep).join('/')}:/w:ro`, IMG, ...args],
      { encoding: 'utf8', maxBuffer: 256 * 1024 * 1024, env: { ...process.env, MSYS_NO_PATHCONV: '1' } });
  if (r.status !== 0) throw new Error((r.stderr || r.error?.message || `exit ${r.status}`).trim().split('\n')[0]);
  return JSON.parse(r.stdout.trim() || '[]');
}

let status = 0;
let acceptedTotal = 0;
try {
  for (const f of listRev('specs/openapi')) put('rev', f, fs.readFileSync(path.join(repo, f), 'utf8'));
  for (const f of baseFiles) put('base', f, gitShow(base, f));
  for (const f of files) {
    let baseFile = f;
    if (!baseFiles.includes(f)) {
      const old = gitShow(base, `specs/openapi/services/${path.basename(f)}`); // pre-ADR-0074 path
      if (old === null) {
        console.log(`skip (new): ${f}`);
        continue;
      }
      baseFile = `specs/openapi/modules/${path.basename(f)}`;
      put('base', baseFile, old);
    }
    let changes;
    try {
      changes = oasdiff(`/w/base/${baseFile}`, `/w/rev/${f}`);
    } catch (e) {
      console.error(`ERROR: ${f}: oasdiff could not compare: ${e.message}`);
      status = 1;
      continue;
    }
    const { accepted, rejected } = classify(entries, f, changes);
    acceptedTotal += accepted.length;
    accepted.forEach(({ change, entry }) => console.log(`allowed (${entry.adr}): ${describe(f, change)}`));
    if (rejected.length) {
      rejected.forEach((c) => console.error(`BREAKING: ${describe(f, c)}`));
      for (const c of rejected) {
        const value = /enum value '([^']+)'/.exec(c.text ?? '')?.[1];
        const key = [c.id, c.operation, c.path, value ?? c.text].join(' ');
        const s = suggestions.get(key) ?? { adr: '', reason: '', file: [], id: c.id, operation: c.operation, path: c.path,
          text: value ? `enum value '${value}'` : c.text };
        if (!s.file.includes(f)) s.file.push(f);
        suggestions.set(key, s);
      }
      status = 1;
    } else {
      console.log(`ok: ${f}${accepted.length ? ` (${accepted.length} breaking change(s) allowed by ${ALLOWLIST})` : ''}`);
    }
  }
} finally {
  fs.rmSync(work, { recursive: true, force: true });
}
if (suggest && suggestions.size) console.log(yaml.dump({ exceptions: [...suggestions.values()] }, { lineWidth: -1 }));
console.log(`base ${base}: ${acceptedTotal} breaking change(s) allowed by ADR exceptions${status ? ', others rejected' : ''}`);
process.exit(status);
