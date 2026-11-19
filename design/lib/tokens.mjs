// Token loading, merging and alias resolution (F005). Pure functions over W3C Design Tokens JSON:
// a token is an object with `$value`; groups may carry `$type` (inherited), `$description`, `$extensions`.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { CONTEXTS, PRIMITIVE_FILES } from '../style-dictionary.config.mjs';

export const DESIGN_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export const REPO_ROOT = path.resolve(DESIGN_DIR, '..');

export const W3C_TYPES = new Set([
  'color', 'dimension', 'fontFamily', 'fontWeight', 'duration', 'number', 'shadow', 'gradient', 'cubicBezier',
]);

const ALIAS = /^\{([^{}]+)\}$/;
const NAME = /^[A-Za-z0-9_-]+$/;

export function aliasTarget(value) {
  const m = typeof value === 'string' ? ALIAS.exec(value) : null;
  return m ? m[1] : null;
}

/** Reads a token file relative to `designDir`; returns the parsed JSON or throws with the file name. */
export function readTokenFile(rel, designDir = DESIGN_DIR) {
  const abs = path.join(designDir, rel);
  try {
    return JSON.parse(fs.readFileSync(abs, 'utf8'));
  } catch (e) {
    throw new Error(`${rel}: ${e.message}`);
  }
}

/**
 * Flattens one parsed file into tokens. Returns {tokens, errors}; tokens are
 * {path, value, type, description, deprecated, extensions, file}.
 */
export function flattenFile(json, file) {
  const tokens = [];
  const errors = [];
  const walk = (node, trail, inheritedType) => {
    const groupType = node.$type ?? inheritedType;
    for (const [key, child] of Object.entries(node)) {
      if (key.startsWith('$')) continue;
      const p = [...trail, key];
      const tokenPath = p.join('.');
      if (!NAME.test(key)) errors.push({ rule: 'structure', file, path: tokenPath, message: `invalid token name segment "${key}"` });
      if (child === null || typeof child !== 'object' || Array.isArray(child)) {
        errors.push({ rule: 'structure', file, path: tokenPath, message: 'expected a token ({"$value": …}) or a group object' });
        continue;
      }
      if ('$value' in child) {
        const type = child.$type ?? groupType;
        if (!type) errors.push({ rule: 'structure', file, path: tokenPath, message: 'missing $type (own or inherited from a group)' });
        else if (!W3C_TYPES.has(type)) errors.push({ rule: 'structure', file, path: tokenPath, message: `unknown $type "${type}"` });
        if (typeof child.$description !== 'string' || child.$description.trim() === '') {
          errors.push({ rule: 'structure', file, path: tokenPath, message: 'missing $description' });
        }
        tokens.push({
          path: tokenPath,
          value: child.$value,
          type,
          description: child.$description,
          deprecated: child.$deprecated,
          extensions: child.$extensions?.eduworld ?? {},
          file,
        });
      } else {
        walk(child, p, groupType);
      }
    }
  };
  walk(json, [], undefined);
  return { tokens, errors };
}

/** Loads files (relative to designDir) → {byFile: Map(file → tokens[]), errors}. */
export function loadFiles(files, designDir = DESIGN_DIR) {
  const byFile = new Map();
  const errors = [];
  for (const file of [...new Set(files)]) {
    let json;
    try {
      json = readTokenFile(file, designDir);
    } catch (e) {
      errors.push({ rule: 'structure', file, path: '', message: e.message });
      continue;
    }
    const r = flattenFile(json, file);
    byFile.set(file, r.tokens);
    errors.push(...r.errors);
  }
  return { byFile, errors };
}

/**
 * Merges files in order. A later token with the same path overrides `$value` (and `$type`/`$description`
 * when given); eduworld extensions are merged so overrides keep e.g. `contrastWith`.
 */
export function mergeTokens(fileList, byFile) {
  const map = new Map();
  for (const file of fileList) {
    for (const t of byFile.get(file) ?? []) {
      const prev = map.get(t.path);
      map.set(t.path, prev
        ? { ...prev, ...t, extensions: { ...prev.extensions, ...t.extensions }, overrides: [...(prev.overrides ?? [prev.file]), t.file] }
        : { ...t });
    }
  }
  return map;
}

/** Replaces `$value` of business colors with the requested variant (light | dark | game). */
export function applyVariant(map, variant) {
  const out = new Map();
  for (const [p, t] of map) {
    const v = t.extensions?.variants?.[variant];
    out.set(p, v !== undefined ? { ...t, value: v } : t);
  }
  return out;
}

function mapComposite(value, fn) {
  if (Array.isArray(value)) return value.map((v) => mapComposite(v, fn));
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, mapComposite(v, fn)]));
  }
  return fn(value);
}

/**
 * Resolves every alias. Returns {resolved: Map(path → value), errors}. Errors name the token path:
 * `alias-missing` (target not found) and `alias-cycle` (with the cycle chain).
 */
export function resolveAll(map) {
  const resolved = new Map();
  const errors = [];
  const reported = new Set();
  const resolveToken = (p, stack) => {
    if (resolved.has(p)) return { ok: true, value: resolved.get(p) };
    const t = map.get(p);
    const idx = stack.indexOf(p);
    if (idx >= 0) {
      const chain = [...stack.slice(idx), p];
      const key = [...new Set(chain)].sort().join('|');
      if (!reported.has(key)) {
        reported.add(key);
        errors.push({ rule: 'alias-cycle', file: t?.file, path: stack[stack.length - 1], message: `circular alias: ${chain.join(' → ')}` });
      }
      return { ok: false };
    }
    let failed = false;
    const value = mapComposite(t.value, (v) => {
      const target = aliasTarget(v);
      if (target === null) return v;
      if (!map.has(target)) {
        errors.push({ rule: 'alias-missing', file: t.file, path: p, message: `alias {${target}} points to a token that does not exist` });
        failed = true;
        return undefined;
      }
      const r = resolveToken(target, [...stack, p]);
      if (!r.ok) failed = true;
      return r.value;
    });
    if (failed) return { ok: false };
    resolved.set(p, value);
    return { ok: true, value };
  };
  for (const p of map.keys()) resolveToken(p, []);
  return { resolved, errors };
}

function containsLiteral(value, type) {
  if (aliasTarget(value) !== null) return false;
  if (type === 'gradient' && Array.isArray(value)) {
    return value.some((stop) => !stop || typeof stop !== 'object' || aliasTarget(stop.color) === null || typeof stop.position !== 'number');
  }
  if (type === 'shadow' && value && typeof value === 'object' && !Array.isArray(value)) {
    return Object.entries(value).some(([k, v]) => (k === 'color' ? aliasTarget(v) === null : typeof v === 'string' && aliasTarget(v) === null));
  }
  return true;
}

/** Semantic/component/mode/density/art tokens must alias a primitive (F005-AC-04). */
export function checkLiterals(byFile, primitiveFiles = PRIMITIVE_FILES) {
  const errors = [];
  for (const [file, tokens] of byFile) {
    if (primitiveFiles.includes(file)) continue;
    for (const t of tokens) {
      if (containsLiteral(t.value, t.type)) {
        errors.push({ rule: 'literal-in-semantic', file, path: t.path, message: `literal value ${JSON.stringify(t.value)} — semantic/component tokens must alias a primitive ({path.to.token})` });
      }
    }
  }
  return errors;
}

/**
 * Loads every context of the config. Returns {byFile, contexts: {name → {target, variant, tokens, resolved}}, errors}.
 */
export function loadContexts(designDir = DESIGN_DIR, contexts = CONTEXTS) {
  const allFiles = [...new Set(Object.values(contexts).flatMap((c) => c.files))];
  const { byFile, errors } = loadFiles(allFiles, designDir);
  const out = {};
  for (const [name, c] of Object.entries(contexts)) {
    const tokens = applyVariant(mergeTokens(c.files, byFile), c.variant);
    const r = resolveAll(tokens);
    for (const e of r.errors) {
      if (!errors.some((x) => x.rule === e.rule && x.path === e.path && x.message === e.message)) errors.push({ ...e, context: name });
    }
    out[name] = { ...c, name, tokens, resolved: r.resolved };
  }
  return { byFile, contexts: out, errors };
}

/** Formats findings as `rule file path: message` lines. */
export function formatErrors(errors) {
  return errors.map((e) => `[${e.rule}] ${e.file ?? ''}${e.path ? ` ${e.path}` : ''}${e.context ? ` (${e.context})` : ''}: ${e.message}`).join('\n');
}
