// F005-AC-05, F005-AC-06: business colors (brand, subject/zone, rarity, content status) are declared only in
// shared-tokens, each has light/dark/game variants, and the generated web and Godot outputs carry the same
// names with the value of the matching variant.
import { BUSINESS_COLOR_PREFIXES, BUSINESS_VARIANTS, SETS, OUTPUTS } from '../style-dictionary.config.mjs';
import { godotColor, isHex } from '../lib/color.mjs';
import { cssVarName } from '../lib/names.mjs';

const isBusiness = (p) => BUSINESS_COLOR_PREFIXES.some((pre) => p.startsWith(pre));

export function checkDomainSources(byFile) {
  const errors = [];
  for (const [file, tokens] of byFile) {
    const shared = SETS.shared.includes(file);
    for (const t of tokens) {
      if (!isBusiness(t.path)) continue;
      if (!shared) {
        errors.push({ rule: 'business-color-outside-shared', file, path: t.path, message: 'business colors may only be declared in design/shared-tokens/ (AGENTS R32)' });
        continue;
      }
      const v = t.extensions?.variants ?? {};
      for (const name of BUSINESS_VARIANTS) {
        if (!isHex(v[name])) errors.push({ rule: 'business-variant-missing', file, path: t.path, message: `missing or invalid "${name}" variant (#RRGGBB) in $extensions.eduworld.variants` });
      }
    }
  }
  return errors;
}

/** Business colors of the shared set: path → variants. */
export function sharedBusinessColors(byFile) {
  const out = new Map();
  for (const f of SETS.shared) for (const t of byFile.get(f) ?? []) if (isBusiness(t.path)) out.set(t.path, t.extensions?.variants ?? {});
  return out;
}

export function parseWebDomainColors(ts) {
  const block = /export const domainColors[^=]*=\s*(\{[\s\S]*?\n\});/.exec(ts);
  const out = new Map();
  if (!block) return out;
  for (const m of block[1].matchAll(/"([\w.]+)":\s*\{\s*light:\s*"(#[0-9A-Fa-f]+)",\s*dark:\s*"(#[0-9A-Fa-f]+)"\s*\}/g)) out.set(m[1], { light: m[2], dark: m[3] });
  return out;
}

export function parseGodotDomainColors(gd) {
  const block = /const DOMAIN_COLORS: Dictionary = \{([\s\S]*?)\n\}/.exec(gd);
  const out = new Map();
  if (!block) return out;
  for (const m of block[1].matchAll(/"([\w.]+)":\s*(Color\([^)]*\))/g)) out.set(m[1], m[2]);
  return out;
}

function cssBlock(css, selector) {
  const i = css.indexOf(`${selector} {`);
  if (i < 0) return '';
  return css.slice(i, css.indexOf('}', i));
}

/** Compares generated files (Map rel → content) with the shared source variants (TC-UI-008). */
export function checkDomainOutputs(files, shared) {
  const errors = [];
  const text = (rel) => String(files.get(rel) ?? '');
  const web = parseWebDomainColors(text(OUTPUTS.antdTheme));
  const godot = parseGodotDomainColors(text(OUTPUTS.uiTokens));
  const css = text(OUTPUTS.tokensCss);
  const root = cssBlock(css, ':root');
  const dark = cssBlock(css, '[data-theme="dark"]');
  for (const [p, v] of shared) {
    if (!web.has(p)) errors.push({ rule: 'business-parity', file: OUTPUTS.antdTheme, path: p, message: 'missing in domainColors' });
    else if (web.get(p).light !== v.light || web.get(p).dark !== v.dark) errors.push({ rule: 'business-parity', file: OUTPUTS.antdTheme, path: p, message: `domainColors ${JSON.stringify(web.get(p))} ≠ source light ${v.light} / dark ${v.dark}` });
    if (!godot.has(p)) errors.push({ rule: 'business-parity', file: OUTPUTS.uiTokens, path: p, message: 'missing in UiTokens.DOMAIN_COLORS' });
    else if (godot.get(p) !== godotColor(v.game)) errors.push({ rule: 'business-parity', file: OUTPUTS.uiTokens, path: p, message: `DOMAIN_COLORS ${godot.get(p)} ≠ source game ${v.game}` });
    const name = cssVarName(p);
    if (!root.includes(`${name}: ${String(v.light).toUpperCase()};`)) errors.push({ rule: 'business-parity', file: OUTPUTS.tokensCss, path: p, message: `${name} on :root ≠ light ${v.light}` });
    if (v.dark !== v.light && !dark.includes(`${name}: ${String(v.dark).toUpperCase()};`)) errors.push({ rule: 'business-parity', file: OUTPUTS.tokensCss, path: p, message: `${name} on [data-theme="dark"] ≠ dark ${v.dark}` });
  }
  for (const p of web.keys()) if (!shared.has(p)) errors.push({ rule: 'business-parity', file: OUTPUTS.antdTheme, path: p, message: 'domainColors has a name that is not a shared business color' });
  for (const p of godot.keys()) if (!shared.has(p)) errors.push({ rule: 'business-parity', file: OUTPUTS.uiTokens, path: p, message: 'DOMAIN_COLORS has a name that is not a shared business color' });
  return errors;
}
