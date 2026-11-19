// Naming and value conversion shared by formats and checks (F005), so a check can tell which token a
// generated name came from.

const camelToKebab = (s) => s.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase();

/** `antd.colorPrimary` → `--ew-antd-color-primary`; `text.expansion_allowance` → `--ew-text-expansion-allowance`. */
export function cssVarName(tokenPath) {
  return `--ew-${tokenPath.split('.').map((s) => camelToKebab(s).replace(/_/g, '-')).join('-')}`;
}

/** `color.subject.math` → `COLOR_SUBJECT_MATH`; `palette.zone.sky-day` → `PALETTE_ZONE_SKY_DAY`. */
export function gdConstName(tokenPath) {
  return tokenPath.split('.').map((s) => camelToKebab(s).replace(/-/g, '_')).join('_').toUpperCase();
}

export function px(value) {
  const m = /^(-?\d+(?:\.\d+)?)px$/.exec(String(value));
  if (!m) throw new Error(`expected a px dimension, got ${JSON.stringify(value)}`);
  return Number(m[1]);
}

export function seconds(value) {
  const m = /^(\d+(?:\.\d+)?)(ms|s)$/.exec(String(value));
  if (!m) throw new Error(`expected a duration, got ${JSON.stringify(value)}`);
  return m[2] === 'ms' ? Number(m[1]) / 1000 : Number(m[1]);
}

/** GDScript float literal (always has a decimal point). */
export function gdFloat(n) {
  const s = String(Math.round(n * 10000) / 10000);
  return s.includes('.') ? s : `${s}.0`;
}

export function cssFontFamily(list) {
  return [].concat(list).map((f) => (/^[a-z-]+$/.test(f) ? f : `"${f}"`)).join(', ');
}

/** Stable sort of token paths. */
export function byPath(a, b) {
  return a < b ? -1 : a > b ? 1 : 0;
}
