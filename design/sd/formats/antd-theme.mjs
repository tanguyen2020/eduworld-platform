// Format: Ant Design 5 ThemeConfig for OpsThemeProvider (F005 contracts §5):
//   export const lightTheme, darkTheme, compactTheme: ThemeConfig;
//   export const compactOverrides (token diff of compact vs light, to combine compact with dark);
//   export const domainColors: Record<string, {light: string; dark: string}>.
import { GENERATED_MARK, BUSINESS_COLOR_PREFIXES } from '../../style-dictionary.config.mjs';
import { byPath, cssFontFamily, px } from '../../lib/names.mjs';

/** Seed value for ConfigProvider: px dimensions as numbers, font stacks as one string. */
export function antdValue(value, type) {
  if (type === 'dimension') return px(value);
  if (type === 'fontFamily') return cssFontFamily(value).replace(/"/g, "'");
  return value;
}

function seedTokens(map) {
  const out = {};
  for (const p of [...map.keys()].sort(byPath)) {
    if (!p.startsWith('antd.')) continue;
    const { value, type } = map.get(p);
    out[p.slice('antd.'.length)] = antdValue(value, type);
  }
  return out;
}

const ts = (obj, indent = 2) => JSON.stringify(obj, null, indent).replace(/"([A-Za-z_][A-Za-z0-9_]*)":/g, '$1:');

/**
 * @param {{light: Map, dark: Map, compact: Map}} modes resolved maps (variant applied per mode)
 * @param {Map} variants path → {light, dark} for business colors
 */
export function formatAntdTheme(modes, variants, version) {
  const light = seedTokens(modes.light);
  const dark = seedTokens(modes.dark);
  const compact = seedTokens(modes.compact);
  const compactOverrides = Object.fromEntries(Object.entries(compact).filter(([k, v]) => light[k] !== v));
  const domain = {};
  for (const p of [...variants.keys()].sort(byPath)) {
    if (!BUSINESS_COLOR_PREFIXES.some((pre) => p.startsWith(pre))) continue;
    const v = variants.get(p);
    domain[p] = { light: v.light, dark: v.dark };
  }
  return `// ${GENERATED_MARK}. Source: design/shared-tokens, design/admin-tokens (tokens@${version}). Run make design-tokens.
import { theme } from 'antd';
import type { ThemeConfig } from 'antd';

export const TOKENS_VERSION = '${version}';

export const lightTheme: ThemeConfig = {
  algorithm: theme.defaultAlgorithm,
  token: ${ts(light).replace(/\n/g, '\n  ')},
};

export const darkTheme: ThemeConfig = {
  algorithm: theme.darkAlgorithm,
  token: ${ts(dark).replace(/\n/g, '\n  ')},
};

/** Compact density on the light mode (Ant Design compactAlgorithm is not used: 13px minimum, DESIGN_SYSTEM §4.5). */
export const compactTheme: ThemeConfig = {
  algorithm: theme.defaultAlgorithm,
  token: ${ts(compact).replace(/\n/g, '\n  ')},
};

/** Seed tokens that compact changes; merge into darkTheme.token for dark + compact. */
export const compactOverrides: NonNullable<ThemeConfig['token']> = ${ts(compactOverrides)};

/** Business colors (shared-tokens) per admin mode; the game client uses the same names (F005-AC-05). */
export const domainColors: Record<string, { light: string; dark: string }> = ${ts(domain)};
`;
}
