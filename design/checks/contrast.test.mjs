// F005-AC-08 (TC-UI-002): WCAG 2.1 AA for every declared pair, report JSON, text token without pair fails.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkContrast } from './contrast.mjs';
import { contrastRatio } from '../lib/color.mjs';
import { loadContexts } from '../lib/tokens.mjs';
import { DENSITY_BANDS } from '../style-dictionary.config.mjs';
import { copyDesign, editJson, rm, find } from './test-helpers.mjs';

test('WCAG 2.1 contrast ratio matches reference values', () => {
  assert.equal(contrastRatio('#000000', '#FFFFFF'), 21);
  assert.equal(contrastRatio('#FFFFFF', '#FFFFFF'), 1);
  assert.equal(contrastRatio('#767676', '#FFFFFF'), 4.54);
  assert.equal(contrastRatio('#FFFFFF', '#2F6BFF'), 4.49); // the original brand proposal fails AA by a hair
});

test('repository pairs all pass and the report covers every mode and density', () => {
  const { errors, report } = checkContrast(loadContexts());
  assert.deepEqual(errors, []);
  assert.equal(report.summary.failed, 0);
  const contexts = new Set(report.pairs.map((p) => p.context));
  for (const c of ['web-light', 'web-dark', 'web-compact', 'game', 'game-high-contrast']) assert.ok(contexts.has(c), c);
  const densities = new Set(report.pairs.filter((p) => p.context === 'game-high-contrast').map((p) => p.density));
  assert.deepEqual([...densities].sort(), [...DENSITY_BANDS].sort());
});

test('a low-contrast admin text color fails and the report shows the pair', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'admin-tokens/seed.json', (j) => { j.ops.palette.slate['600'].$value = '#BFBFBF'; });
    const { errors, report } = checkContrast(loadContexts(dir));
    const e = find(errors, 'contrast', 'ops.color.text.secondary');
    assert.ok(e);
    assert.equal(e.context, 'web-light');
    assert.ok(report.pairs.some((p) => !p.pass && p.fg === 'ops.color.text.secondary' && p.context === 'web-light'));
  } finally {
    rm(dir);
  }
});

test('large/graphic level uses 3:1', () => {
  const dir = copyDesign();
  try {
    // #949494 on white ≈ 3.03:1: passes graphic, fails normal
    editJson(dir, 'admin-tokens/seed.json', (j) => { j.ops.palette.status.success.$value = '#949494'; });
    assert.equal(find(checkContrast(loadContexts(dir)).errors, 'contrast', 'status.success'), undefined);
    editJson(dir, 'admin-tokens/semantic.json', (j) => { j.status.success.$extensions.eduworld.contrastWith[0].level = 'normal'; });
    assert.ok(find(checkContrast(loadContexts(dir)).errors, 'contrast', 'status.success'));
  } finally {
    rm(dir);
  }
});

test('a high contrast override below AA fails in the game-high-contrast context', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'game-tokens/high-contrast.json', (j) => { j.color.text.primary.$value = '{palette.sky.200}'; });
    const e = find(checkContrast(loadContexts(dir)).errors, 'contrast', 'color.text.primary');
    assert.equal(e.context, 'game-high-contrast');
  } finally {
    rm(dir);
  }
});

test('a new text color token without contrastWith fails', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'game-tokens/semantic.json', (j) => { j.color.text.link = { $value: '{palette.blue.600}', $description: 'link' }; });
    assert.ok(find(checkContrast(loadContexts(dir)).errors, 'contrast-pair-missing', 'color.text.link'));
  } finally {
    rm(dir);
  }
});
