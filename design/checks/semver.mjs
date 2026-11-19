// F005-AC-13, F005-AC-14: token versioning (DESIGN_SYSTEM §3, §8). Compared with a base git ref (the PR base,
// default origin/main): removing or renaming a token needs a MAJOR bump of design/package.json, a removed
// token must already carry `$deprecated` in the base, the version never goes down, and design/CHANGELOG.md
// has an entry `## tokens@<version>`.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { DESIGN_DIR, REPO_ROOT, flattenFile } from '../lib/tokens.mjs';

const TOKEN_DIRS = /^design\/(shared-tokens|game-tokens|admin-tokens)\/.+\.json$/;

export function parseSemver(v) {
  const m = /^(\d+)\.(\d+)\.(\d+)$/.exec(String(v));
  if (!m) throw new Error(`invalid semver "${v}"`);
  return m.slice(1).map(Number);
}

function cmp(a, b) {
  for (let i = 0; i < 3; i++) if (a[i] !== b[i]) return a[i] - b[i];
  return 0;
}

/** snapshot: {version, tokens: Map(path → {deprecated})}. */
export function compareSnapshots(base, head) {
  const errors = [];
  const bv = parseSemver(base.version);
  const hv = parseSemver(head.version);
  if (cmp(hv, bv) < 0) errors.push({ rule: 'semver', file: 'package.json', path: '', message: `version ${head.version} is lower than base ${base.version}` });
  const removed = [...base.tokens.keys()].filter((p) => !head.tokens.has(p)).sort();
  if (removed.length && hv[0] <= bv[0]) {
    errors.push({ rule: 'semver-major', file: 'package.json', path: '', message: `token(s) removed or renamed without a MAJOR bump (${base.version} → ${head.version}): ${removed.join(', ')}` });
  }
  for (const p of removed) {
    if (!base.tokens.get(p).deprecated) errors.push({ rule: 'deprecated-before-removal', file: '', path: p, message: 'token removed but it was not marked $deprecated in the base version' });
  }
  return errors;
}

export function snapshotFromFiles(version, files) {
  const tokens = new Map();
  for (const [file, text] of files) {
    for (const t of flattenFile(JSON.parse(text), file).tokens) tokens.set(t.path, { deprecated: t.deprecated !== undefined && t.deprecated !== false });
  }
  return { version, tokens };
}

const git = (args, cwd) => execFileSync('git', args, { cwd, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });

/** Snapshot of design/ at a git ref, or null when the ref (or design/) does not exist there. */
export function snapshotAtRef(ref, root = REPO_ROOT) {
  let list;
  try {
    list = git(['ls-tree', '-r', '--name-only', ref, '--', 'design/'], root).split('\n').filter(Boolean);
  } catch {
    return null;
  }
  if (!list.includes('design/package.json')) return null;
  const version = JSON.parse(git(['show', `${ref}:design/package.json`], root)).version;
  const files = list.filter((f) => TOKEN_DIRS.test(f)).map((f) => [f, git(['show', `${ref}:${f}`], root)]);
  return snapshotFromFiles(version, files);
}

export function snapshotWorkingTree(designDir = DESIGN_DIR) {
  const version = JSON.parse(fs.readFileSync(path.join(designDir, 'package.json'), 'utf8')).version;
  const files = [];
  const walk = (dir) => {
    for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
      const abs = path.join(dir, e.name);
      if (e.isDirectory()) walk(abs);
      else if (e.name.endsWith('.json')) files.push([`design/${path.relative(designDir, abs).split(path.sep).join('/')}`, fs.readFileSync(abs, 'utf8')]);
    }
  };
  for (const d of ['shared-tokens', 'game-tokens', 'admin-tokens']) if (fs.existsSync(path.join(designDir, d))) walk(path.join(designDir, d));
  return snapshotFromFiles(version, files);
}

export function checkChangelog(version, designDir = DESIGN_DIR) {
  const file = path.join(designDir, 'CHANGELOG.md');
  const text = fs.existsSync(file) ? fs.readFileSync(file, 'utf8') : '';
  return text.includes(`## tokens@${version}`) ? [] : [{ rule: 'semver', file: 'CHANGELOG.md', path: '', message: `no "## tokens@${version}" entry` }];
}

/** Full check against `baseRef`; returns {errors, skipped} (skipped when the base has no design/). */
export function checkSemver(baseRef, designDir = DESIGN_DIR, root = REPO_ROOT) {
  const head = snapshotWorkingTree(designDir);
  const errors = checkChangelog(head.version, designDir);
  const base = baseRef ? snapshotAtRef(baseRef, root) : null;
  if (!base) return { errors, skipped: true };
  return { errors: [...errors, ...compareSnapshots(base, head)], skipped: false };
}
