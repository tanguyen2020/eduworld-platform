// Style Dictionary v4 integration (F005): custom transforms and resolution of one context.
// Style Dictionary resolves aliases and applies transforms; formats in sd/formats/ turn the result into files.
import StyleDictionary from 'style-dictionary';
import { cssVarName, gdConstName } from '../lib/names.mjs';

let registered = false;

/** Registers EduWorld transforms once: CSS variable name, GDScript constant name, hex normalization. */
export function registerTransforms() {
  if (registered) return;
  registered = true;
  StyleDictionary.registerTransform({
    name: 'ew/name/css',
    type: 'name',
    transform: (token) => cssVarName(token.path.join('.')),
  });
  StyleDictionary.registerTransform({
    name: 'ew/name/gd',
    type: 'name',
    transform: (token) => gdConstName(token.path.join('.')),
  });
  StyleDictionary.registerTransform({
    name: 'ew/color/hex-upper',
    type: 'value',
    filter: (token) => token.$type === 'color' && typeof token.$value === 'string',
    transform: (token) => token.$value.toUpperCase(),
  });
}

/** Rebuilds a nested W3C tree from a flat token map (variant already applied). */
export function toTree(map) {
  const root = {};
  for (const [p, t] of map) {
    const parts = p.split('.');
    let node = root;
    for (const k of parts.slice(0, -1)) node = node[k] ??= {};
    node[parts[parts.length - 1]] = { $value: t.value, $type: t.type, $description: t.description };
  }
  return root;
}

/**
 * Resolves a context's token map with Style Dictionary. Returns Map(path → {value, type, name}) where
 * `name` follows `nameTransform` (`ew/name/css` or `ew/name/gd`).
 */
export async function resolveWithStyleDictionary(map, nameTransform = 'ew/name/css') {
  registerTransforms();
  const sd = new StyleDictionary({
    tokens: toTree(map),
    usesDtcg: true,
    log: { verbosity: 'silent', warnings: 'disabled', errors: { brokenReferences: 'throw' } },
    platforms: { p: { transforms: [nameTransform, 'ew/color/hex-upper'] } },
  });
  await sd.hasInitialized;
  const { allTokens } = await sd.getPlatformTokens('p');
  const out = new Map();
  for (const t of allTokens) out.set(t.path.join('.'), { value: t.$value, type: t.$type, name: t.name });
  return out;
}
