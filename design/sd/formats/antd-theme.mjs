// Format: Ant Design 5 ThemeConfig for OpsThemeProvider (F005 contracts §5):
//   export const lightTheme, darkTheme, compactTheme: ThemeConfig (seed/map tokens `antd.*`, component tokens
//   `antd.components.<Component>.<token>` → ThemeConfig.components);
//   export const compactOverrides (token diff of compact vs light, to combine compact with dark);
//   export const authTheme (`auth.antd.*`: sizes of the sign-in pages, merged by a nested ConfigProvider);
//   export const domainColors: Record<string, {light: string; dark: string}>.
import { GENERATED_MARK, BUSINESS_COLOR_PREFIXES } from '../../style-dictionary.config.mjs';
import { byPath, cssFontFamily, px } from '../../lib/names.mjs';

/** Seed value for ConfigProvider: px dimensions as numbers, font stacks as one string. */
export function antdValue(value, type) {
  if (type === 'dimension') return px(value);
  if (type === 'fontFamily') return cssFontFamily(value).replace(/"/g, "'");
  return value;
}

/** `prefix.*` tokens → { token, components } of a ThemeConfig (components under `prefix.components.<C>.<t>`). */
function themeParts(map, prefix) {
  const token = {};
  const components = {};
  const comp = `${prefix}components.`;
  for (const p of [...map.keys()].sort(byPath)) {
    if (!p.startsWith(prefix)) continue;
    const { value, type } = map.get(p);
    if (p.startsWith(comp)) {
      const [name, ...rest] = p.slice(comp.length).split('.');
      (components[name] ??= {})[rest.join('.')] = antdValue(value, type);
    } else token[p.slice(prefix.length)] = antdValue(value, type);
  }
  return { token, components };
}

const ts = (obj, indent = 2) => JSON.stringify(obj, null, indent).replace(/"([A-Za-z_][A-Za-z0-9_]*)":/g, '$1:');
const block = (obj) => ts(obj).replace(/\n/g, '\n  ');
const componentsLine = (c) => (Object.keys(c).length ? `\n  components: ${block(c)},` : '');

/**
 * @param {{light: Map, dark: Map, compact: Map}} modes resolved maps (variant applied per mode)
 * @param {Map} variants path → {light, dark} for business colors
 */
export function formatAntdTheme(modes, variants, version) {
  const light = themeParts(modes.light, 'antd.');
  const dark = themeParts(modes.dark, 'antd.');
  const compact = themeParts(modes.compact, 'antd.');
  const auth = themeParts(modes.light, 'auth.antd.');
  const compactOverrides = Object.fromEntries(Object.entries(compact.token).filter(([k, v]) => light.token[k] !== v));
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
  token: ${block(light.token)},${componentsLine(light.components)}
};

export const darkTheme: ThemeConfig = {
  algorithm: theme.darkAlgorithm,
  token: ${block(dark.token)},${componentsLine(dark.components)}
};

/** Compact density on the light mode (Ant Design compactAlgorithm is not used: 13px minimum, DESIGN_SYSTEM §4.5). */
export const compactTheme: ThemeConfig = {
  algorithm: theme.defaultAlgorithm,
  token: ${block(compact.token)},${componentsLine(compact.components)}
};

/** Seed tokens that compact changes; merge into darkTheme.token for dark + compact. */
export const compactOverrides: NonNullable<ThemeConfig['token']> = ${ts(compactOverrides)};

/** Sign-in pages (AuthLayout): sizes merged over the active theme by a nested ConfigProvider (DESIGN_SYSTEM §4.1). */
export const authTheme: ThemeConfig = {
  token: ${block(auth.token)},${componentsLine(auth.components)}
};

/** Business colors (shared-tokens) per admin mode; the game client uses the same names (F005-AC-05). */
export const domainColors: Record<string, { light: string; dark: string }> = ${ts(domain)};
`;
}
