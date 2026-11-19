// F005-AC-12: OFL LICENSE present, glyph coverage for Vietnamese + Latin.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { checkFonts, requiredCodePoints } from './fonts.mjs';
import { copyDesign, rm, find } from './test-helpers.mjs';

test('the check set contains every Vietnamese letter with tone marks', () => {
  const set = new Set(requiredCodePoints());
  for (const ch of 'ỹỸđĐơƠưƯậẬễỄ') assert.ok(set.has(ch.codePointAt(0)), ch);
});

test('repository fonts have an OFL license and full coverage', () => {
  assert.deepEqual(checkFonts(), []);
});

test('a missing LICENSE fails', () => {
  const dir = copyDesign();
  try {
    fs.rmSync(path.join(dir, 'fonts/Baloo2/LICENSE'));
    assert.equal(find(checkFonts(dir), 'font-license').file, 'fonts/Baloo2/LICENSE');
  } finally {
    rm(dir);
  }
});

test('a non-OFL LICENSE fails', () => {
  const dir = copyDesign();
  try {
    fs.writeFileSync(path.join(dir, 'fonts/BeVietnamPro/LICENSE'), 'All rights reserved.\n');
    assert.ok(find(checkFonts(dir), 'font-license'));
  } finally {
    rm(dir);
  }
});

test('a glyph missing from the font is reported', () => {
  // U+4E00 (CJK) stands in for a missing Vietnamese glyph: none of the fonts have it.
  const errors = checkFonts(undefined, [...requiredCodePoints(), 0x4e00]);
  assert.ok(errors.length >= 1);
  assert.ok(errors.every((e) => e.rule === 'font-glyphs' && /一/.test(e.message)));
});
