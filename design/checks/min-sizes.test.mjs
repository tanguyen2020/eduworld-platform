// F005-AC-09, F005-AC-10: density minimums (DESIGN_SYSTEM §5.2, NFR-A11Y-001) and compact minimum (§4.5),
// thresholds read from the owning document.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkMinSizes, loadThresholds, parseThresholds } from './min-sizes.mjs';
import { loadContexts } from '../lib/tokens.mjs';
import { copyDesign, editJson, rm, find } from './test-helpers.mjs';

test('thresholds come from DESIGN_SYSTEM §5.2 and §4.5', () => {
  const t = loadThresholds();
  assert.deepEqual(t.bands.g1_3, { fontBase: 24, buttonMinHeight: 64 });
  assert.deepEqual(t.bands.g8_9, { fontBase: 18, buttonMinHeight: 44 });
  assert.equal(t.compactMinFont, 13);
  assert.throws(() => parseThresholds('no table'), /g1_3/);
});

test('repository tokens meet every minimum', () => {
  assert.deepEqual(checkMinSizes(loadContexts(), loadThresholds()), []);
});

test('g1_3 base font below 24px fails', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'game-tokens/density/g1_3.json', (j) => { j.density.g1_3.font.base.$value = '{size.font.22}'; });
    const e = find(checkMinSizes(loadContexts(dir), loadThresholds()), 'min-size', 'density.g1_3.font.base');
    assert.match(e.message, /22px < 24px/);
  } finally {
    rm(dir);
  }
});

test('any g1_3 font below the NFR minimum fails, other band button height below §5.2 fails', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'game-tokens/density/g1_3.json', (j) => { j.density.g1_3.font.small.$value = '{size.font.20}'; });
    editJson(dir, 'game-tokens/density/g8_9.json', (j) => { j.density.g8_9.button.min_height.$value = '{size.space.32}'; });
    const errors = checkMinSizes(loadContexts(dir), loadThresholds());
    assert.ok(find(errors, 'min-size', 'density.g1_3.font.small'));
    assert.match(find(errors, 'min-size', 'density.g8_9.button.min_height').message, /32px < 44px/);
  } finally {
    rm(dir);
  }
});

test('compact font size below 13px fails', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'admin-tokens/seed.json', (j) => { j.ops.size.font['12'] = { $value: '12px', $description: 'too small' }; });
    editJson(dir, 'admin-tokens/modes/compact.json', (j) => { j.antd.fontSize.$value = '{ops.size.font.12}'; });
    const e = find(checkMinSizes(loadContexts(dir), loadThresholds()), 'min-size', 'antd.fontSize');
    assert.equal(e.context, 'web-compact');
  } finally {
    rm(dir);
  }
});
