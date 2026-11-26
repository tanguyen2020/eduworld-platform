// Shared loader of deploy/swarm/policy.yaml and the stack files of one environment (F130 check scripts).
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
export const yaml = require('../node_modules/js-yaml');

export const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');

export function loadPolicy(file = path.join(REPO_ROOT, 'deploy', 'swarm', 'policy.yaml')) {
  return yaml.load(fs.readFileSync(file, 'utf8'));
}

/** Glob with `*` only (service name patterns of the policy). */
export function globMatch(pattern, name) {
  const re = new RegExp('^' + pattern.split('*').map((s) => s.replace(/[.+?^${}()|[\]\\]/g, '\\$&')).join('.*') + '$');
  return re.test(name);
}
export const anyGlob = (patterns, name) => (patterns ?? []).some((p) => globMatch(p, name));

/** Loads every stack file of an environment directory: [{file, text, doc}]. Missing files are reported by the caller. */
export function loadStacks(envDir, policy) {
  return (policy.stack_files ?? []).map((f) => {
    const file = path.join(envDir, f);
    if (!fs.existsSync(file)) return { file, missing: true };
    const text = fs.readFileSync(file, 'utf8');
    return { file, text, doc: yaml.load(text) ?? {} };
  });
}

/** 1-based line of the first line matching re at or after line `from` (1-based), else `fallback`. */
export function lineOf(text, re, from = 1, fallback = 1) {
  const lines = text.split(/\r?\n/);
  for (let i = Math.max(0, from - 1); i < lines.length; i++) if (re.test(lines[i])) return i + 1;
  return fallback;
}

/** Line of the `  <service>:` key of a stack file. */
export function serviceLine(text, svc) {
  return lineOf(text, new RegExp(`^  ${svc.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}:\\s*$`));
}

/** Reads a dotenv file as [{key, value, line}], ignoring comments/blank lines. */
export function readEnvFile(file) {
  if (!fs.existsSync(file)) return null;
  return fs.readFileSync(file, 'utf8').split(/\r?\n/).flatMap((l, i) => {
    const m = l.match(/^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$/);
    return m ? [{ key: m[1], value: m[2].replace(/^["']|["']$/g, ''), line: i + 1 }] : [];
  });
}

/** Environment of a service as [{key, value}] from `environment` (map or list). */
export function serviceEnv(svc) {
  const env = svc?.environment;
  if (!env) return [];
  if (Array.isArray(env)) return env.map((e) => { const [k, ...v] = String(e).split('='); return { key: k, value: v.join('=') }; });
  return Object.entries(env).map(([key, value]) => ({ key, value: value == null ? '' : String(value) }));
}

/** Secret references of a service: [{source, target}]. */
export function serviceSecrets(svc) {
  return (svc?.secrets ?? []).map((s) => (typeof s === 'string' ? { source: s, target: s } : { source: s.source, target: s.target ?? s.source }));
}

export const asList = (v) => (v == null ? [] : Array.isArray(v) ? v : Object.keys(v));
