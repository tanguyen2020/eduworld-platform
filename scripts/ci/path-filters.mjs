// Evaluate GitHub Actions path filters locally (F001 T031/T036): which workflows a pull request
// triggers and which modules the go.yml matrix covers. Same semantics as GitHub `on.<event>.paths`
// (last matching pattern wins, `!` negates) and dorny/paths-filter (any pattern of a filter matches).
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const yaml = require('../node_modules/js-yaml');
const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');

export const MODULES = ['identity', 'learning', 'game', 'competition', 'chat'];

/** GitHub glob → RegExp: `**` any depth, `*` within a segment, `?` one char. */
export function globToRegExp(glob) {
  let re = '';
  for (let i = 0; i < glob.length; i++) {
    const c = glob[i];
    if (c === '*' && glob[i + 1] === '*') {
      if (glob[i + 2] === '/') { re += '(?:.*/)?'; i += 2; } else { re += '.*'; i += 1; }
    } else if (c === '*') re += '[^/]*';
    else if (c === '?') re += '[^/]';
    else re += c.replace(/[.+^${}()|[\]\\]/g, '\\$&');
  }
  return new RegExp(`^${re}$`);
}

function matchesPaths(file, patterns) {
  let matched = false;
  for (const p of patterns) {
    const neg = p.startsWith('!');
    if (globToRegExp(neg ? p.slice(1) : p).test(file)) matched = !neg;
  }
  return matched;
}

export function loadWorkflows(dir = path.join(ROOT, '.github', 'workflows')) {
  if (!fs.existsSync(dir)) return [];
  return fs.readdirSync(dir).filter((f) => /\.ya?ml$/.test(f)).map((f) => ({
    id: f.replace(/\.ya?ml$/, ''),
    doc: yaml.load(fs.readFileSync(path.join(dir, f), 'utf8')) ?? {},
  }));
}

/** Workflows (by file id) started by a pull_request touching `files`, sorted. */
export function workflowsTriggered(files, workflows = loadWorkflows()) {
  const out = [];
  for (const { id, doc } of workflows) {
    const on = doc.on ?? doc[true]; // js-yaml may parse a bare `on` key as boolean true
    if (!on || typeof on !== 'object' || !('pull_request' in on)) continue;
    const pr = on.pull_request ?? {};
    const paths = pr.paths;
    const ignore = pr['paths-ignore'];
    let run = true;
    if (paths) run = files.some((f) => matchesPaths(f, paths));
    else if (ignore) run = files.some((f) => !matchesPaths(f, ignore));
    if (run) out.push(id);
  }
  return out.sort();
}

export function loadModuleFilters(file = path.join(ROOT, '.github', 'module-filters.yml')) {
  return yaml.load(fs.readFileSync(file, 'utf8'));
}

/** Filter keys of .github/module-filters.yml matched by `files` (what dorny/paths-filter outputs as `changes`). */
export function changedFilters(files, filters = loadModuleFilters()) {
  return Object.entries(filters)
    .filter(([, patterns]) => files.some((f) => patterns.some((p) => globToRegExp(p).test(f))))
    .map(([k]) => k);
}

/** Modules of the go.yml matrix: `shared` (pkg/, cmd/, go.mod…) enables every module. Sorted. */
export function moduleMatrixFromChanges(changes) {
  if (changes.includes('shared')) return [...MODULES].sort();
  return MODULES.filter((m) => changes.includes(m)).sort();
}

export function moduleMatrix(files, filters) {
  return moduleMatrixFromChanges(changedFilters(files, filters));
}
