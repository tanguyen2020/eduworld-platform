// F005-AC-09, F005-AC-10: minimum sizes. Thresholds are read from the owning document (AGENTS R33):
//   DESIGN_SYSTEM §5.2 table (base font, minimum button height per grade band; g1_3 = NFR-A11Y-001),
//   DESIGN_SYSTEM §4.5 ("Cỡ chữ tối thiểu N px ở compact").
import fs from 'node:fs';
import path from 'node:path';
import { REPO_ROOT } from '../lib/tokens.mjs';
import { DENSITY_BANDS } from '../style-dictionary.config.mjs';
import { px } from '../lib/names.mjs';

export const DESIGN_SYSTEM_DOC = 'docs/05-ux/DESIGN_SYSTEM.md';

/** Parses thresholds from DESIGN_SYSTEM.md text. Throws when a band or the compact minimum is missing. */
export function parseThresholds(markdown) {
  const bands = {};
  for (const m of markdown.matchAll(/^\|\s*`(g\d_\d)`\s*\|\s*(\d+)\s*px\s*\|\s*(\d+)\s*px\s*\|/gm)) {
    bands[m[1]] = { fontBase: Number(m[2]), buttonMinHeight: Number(m[3]) };
  }
  for (const b of DENSITY_BANDS) if (!bands[b]) throw new Error(`${DESIGN_SYSTEM_DOC} §5.2: no row for ${b}`);
  const c = /Cỡ chữ tối thiểu (\d+) px ở compact/.exec(markdown);
  if (!c) throw new Error(`${DESIGN_SYSTEM_DOC} §4.5: compact minimum font size not found`);
  return { bands, compactMinFont: Number(c[1]) };
}

export function loadThresholds(root = REPO_ROOT) {
  return parseThresholds(fs.readFileSync(path.join(root, DESIGN_SYSTEM_DOC), 'utf8'));
}

const dim = (v) => (typeof v === 'string' && /px$/.test(v) ? px(v) : null);

export function checkMinSizes(loaded, thresholds) {
  const errors = [];
  const game = loaded.contexts.game;
  const at = (p) => ({ value: dim(game.resolved.get(p)), file: game.tokens.get(p)?.file });
  for (const band of DENSITY_BANDS) {
    const th = thresholds.bands[band];
    const base = at(`density.${band}.font.base`);
    const btn = at(`density.${band}.button.min_height`);
    if (base.value === null) errors.push({ rule: 'min-size', file: base.file, path: `density.${band}.font.base`, message: 'missing or not a px dimension' });
    else if (base.value < th.fontBase) errors.push({ rule: 'min-size', file: base.file, path: `density.${band}.font.base`, message: `${base.value}px < ${th.fontBase}px (DESIGN_SYSTEM §5.2)` });
    if (btn.value === null) errors.push({ rule: 'min-size', file: btn.file, path: `density.${band}.button.min_height`, message: 'missing or not a px dimension' });
    else if (btn.value < th.buttonMinHeight) errors.push({ rule: 'min-size', file: btn.file, path: `density.${band}.button.min_height`, message: `${btn.value}px < ${th.buttonMinHeight}px (DESIGN_SYSTEM §5.2)` });
  }
  // NFR-A11Y-001: no font of the youngest band below its base minimum.
  const youngest = DENSITY_BANDS[0];
  for (const [p, t] of game.tokens) {
    if (!p.startsWith(`density.${youngest}.font.`)) continue;
    const v = dim(game.resolved.get(p));
    if (v !== null && v < thresholds.bands[youngest].fontBase) {
      errors.push({ rule: 'min-size', file: t.file, path: p, message: `${v}px < ${thresholds.bands[youngest].fontBase}px (NFR-A11Y-001, ${youngest})` });
    }
  }
  // DESIGN_SYSTEM §4.5: compact admin mode, every emitted font size (primitive scale ops.size.* excluded).
  const compact = loaded.contexts['web-compact'];
  for (const [p, t] of compact.tokens) {
    if (t.type !== 'dimension' || !/font/i.test(p) || p.startsWith('ops.size.')) continue;
    const v = dim(compact.resolved.get(p));
    if (v !== null && v < thresholds.compactMinFont) {
      errors.push({ rule: 'min-size', file: t.file, path: p, context: 'web-compact', message: `${v}px < ${thresholds.compactMinFont}px (DESIGN_SYSTEM §4.5 compact minimum)` });
    }
  }
  return errors;
}
