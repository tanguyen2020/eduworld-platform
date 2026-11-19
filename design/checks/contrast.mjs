// F005-AC-08 (TC-UI-002): every declared text/background pair reaches WCAG 2.1 AA in every admin mode
// (light, dark, compact) and every game density, normal and high contrast. A text color token of a
// semantic/component file without `$extensions.eduworld.contrastWith` fails the build.
import { contrastRatio, isHex, WCAG_AA } from '../lib/color.mjs';
import { DENSITY_BANDS } from '../style-dictionary.config.mjs';

const PAIR_FILES = /(^|\/)(semantic|component)\.json$/;

export function isTextColorToken(t) {
  return t.type === 'color' && PAIR_FILES.test(t.file) && t.path.split('.').includes('text');
}

/**
 * @param loaded result of loadContexts
 * @returns {{errors: object[], report: {generatedFor: string, pairs: object[], summary: object}}}
 */
export function checkContrast(loaded) {
  const errors = [];
  const pairs = [];
  for (const tokens of loaded.byFile.values()) {
    for (const t of tokens) {
      if (isTextColorToken(t) && !(t.extensions?.contrastWith?.length > 0)) {
        errors.push({ rule: 'contrast-pair-missing', file: t.file, path: t.path, message: 'text color token must declare $extensions.eduworld.contrastWith' });
      }
    }
  }
  for (const [ctxName, ctx] of Object.entries(loaded.contexts)) {
    const scopes = ctx.target === 'game' ? DENSITY_BANDS : [null];
    for (const [p, t] of ctx.tokens) {
      for (const pair of t.extensions?.contrastWith ?? []) {
        const level = pair.level ?? 'normal';
        const min = WCAG_AA[level];
        const fg = ctx.resolved.get(p);
        const bg = ctx.resolved.get(pair.token);
        if (min === undefined) {
          errors.push({ rule: 'contrast-pair-invalid', file: t.file, path: p, context: ctxName, message: `unknown level "${level}" (normal | large | graphic)` });
          continue;
        }
        if (!ctx.tokens.has(pair.token)) {
          errors.push({ rule: 'contrast-pair-invalid', file: t.file, path: p, context: ctxName, message: `contrastWith token ${pair.token} does not exist in this context` });
          continue;
        }
        if (!isHex(fg) || !isHex(bg)) continue; // unresolved aliases are reported by the alias check
        const ratio = contrastRatio(fg, bg);
        const pass = ratio >= min;
        for (const density of scopes) {
          pairs.push({ context: ctxName, ...(density ? { density } : {}), fg: p, fgValue: fg, bg: pair.token, bgValue: bg, level, ratio, min, pass });
        }
        if (!pass) {
          errors.push({ rule: 'contrast', file: t.file, path: p, context: ctxName, message: `${fg} on ${pair.token} ${bg} = ${ratio}:1 < ${min}:1 (WCAG 2.1 AA, ${level})` });
        }
      }
    }
  }
  const failed = pairs.filter((x) => !x.pass).length;
  return {
    errors,
    report: { standard: 'WCAG 2.1 AA', thresholds: WCAG_AA, summary: { pairs: pairs.length, failed, passed: pairs.length - failed }, pairs },
  };
}
