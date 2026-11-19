// Format: CSS custom properties `--ew-*` for ops-ui (F005, contracts §5). Light values on :root,
// dark differences on [data-theme="dark"], compact differences on [data-density="compact"].
import { GENERATED_MARK } from '../../style-dictionary.config.mjs';
import { byPath, cssFontFamily, cssVarName } from '../../lib/names.mjs';

export function cssValue(value, type) {
  if (type === 'fontFamily') return cssFontFamily(value);
  if (type === 'gradient' || type === 'shadow') throw new Error(`type ${type} is not emitted to CSS`);
  return String(value);
}

function block(selector, entries) {
  if (entries.length === 0) return '';
  return `${selector} {\n${entries.map(([n, v]) => `  ${n}: ${v};`).join('\n')}\n}\n`;
}

/**
 * @param {{light: Map, dark: Map, compact: Map}} modes resolved maps path → {value, type}
 * @param {string} version tokens version
 */
export function formatCssVars(modes, version) {
  const paths = [...modes.light.keys()].sort(byPath);
  const val = (m, p) => cssValue(m.get(p).value, m.get(p).type);
  const root = paths.map((p) => [cssVarName(p), val(modes.light, p)]);
  const diff = (m) => paths.filter((p) => m.has(p) && val(m, p) !== val(modes.light, p)).map((p) => [cssVarName(p), val(m, p)]);
  return [
    `/* ${GENERATED_MARK}. Source: design/shared-tokens, design/admin-tokens (tokens@${version}). Run make design-tokens. */`,
    '',
    block(':root', root),
    block('[data-theme="dark"]', diff(modes.dark)),
    block('[data-density="compact"]', diff(modes.compact)),
  ].join('\n').replace(/\n{3,}/g, '\n\n').trimEnd() + '\n';
}
