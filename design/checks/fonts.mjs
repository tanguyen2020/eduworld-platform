// F005-AC-12: every font family directory in design/fonts/ ships an OFL LICENSE and every font file covers
// Vietnamese with diacritics + Latin Extended (DESIGN_SYSTEM §5.5, ADR-0056).
import fs from 'node:fs';
import path from 'node:path';
import * as fontkit from 'fontkit';
import { DESIGN_DIR } from '../lib/tokens.mjs';

const VI_LOWER = 'aàáảãạăằắẳẵặâầấẩẫậbcdđeèéẻẽẹêềếểễệghiìíỉĩịklmnoòóỏõọôồốổỗộơờớởỡợpqrstuùúủũụưừứửữựvxyỳýỷỹỵ';

/**
 * Check set [Proposed, Clarification 2026-11-19]: Vietnamese letters (both cases, precomposed), Vietnamese
 * combining tone marks, Latin Extended Additional U+1EA0–U+1EF9, printable ASCII, Latin-1 Supplement letters
 * (English loanwords: café, naïve). Full Latin Extended-A is not required (no target language needs it).
 */
export function requiredCodePoints() {
  const set = new Set();
  for (const ch of VI_LOWER + VI_LOWER.toUpperCase()) set.add(ch.codePointAt(0));
  for (let c = 0x21; c <= 0x7e; c++) set.add(c);
  for (let c = 0xc0; c <= 0xff; c++) if (c !== 0xd7 && c !== 0xf7) set.add(c);
  for (const c of [0x300, 0x301, 0x303, 0x309, 0x323]) set.add(c);
  for (let c = 0x1ea0; c <= 0x1ef9; c++) set.add(c);
  return [...set].sort((a, b) => a - b);
}

const FONT_EXT = /\.(ttf|otf|woff2?)$/i;

export function checkFonts(designDir = DESIGN_DIR, codePoints = requiredCodePoints()) {
  const errors = [];
  const root = path.join(designDir, 'fonts');
  if (!fs.existsSync(root)) return [{ rule: 'font', file: 'fonts/', path: '', message: 'missing fonts directory' }];
  const families = fs.readdirSync(root, { withFileTypes: true }).filter((d) => d.isDirectory()).map((d) => d.name);
  if (families.length === 0) errors.push({ rule: 'font', file: 'fonts/', path: '', message: 'no font family directory' });
  for (const fam of families) {
    const dir = path.join(root, fam);
    const rel = `fonts/${fam}`;
    const license = path.join(dir, 'LICENSE');
    if (!fs.existsSync(license)) errors.push({ rule: 'font-license', file: `${rel}/LICENSE`, path: '', message: 'missing LICENSE (SIL OFL 1.1 required)' });
    else if (!/SIL OPEN FONT LICENSE Version 1\.1/i.test(fs.readFileSync(license, 'utf8'))) errors.push({ rule: 'font-license', file: `${rel}/LICENSE`, path: '', message: 'LICENSE is not the SIL Open Font License 1.1' });
    const fonts = fs.readdirSync(dir).filter((f) => FONT_EXT.test(f));
    if (fonts.length === 0) errors.push({ rule: 'font', file: rel, path: '', message: 'no font file' });
    for (const f of fonts) {
      let font;
      try {
        font = fontkit.openSync(path.join(dir, f));
      } catch (e) {
        errors.push({ rule: 'font', file: `${rel}/${f}`, path: '', message: `cannot read font: ${e.message}` });
        continue;
      }
      const missing = codePoints.filter((cp) => !font.hasGlyphForCodePoint(cp));
      if (missing.length) {
        errors.push({ rule: 'font-glyphs', file: `${rel}/${f}`, path: '', message: `missing ${missing.length} glyph(s): ${missing.slice(0, 20).map((cp) => String.fromCodePoint(cp)).join(' ')}${missing.length > 20 ? ' …' : ''}` });
      }
    }
  }
  return errors;
}
