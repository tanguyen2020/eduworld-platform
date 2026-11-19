// F005-AC-04: broken alias, alias cycle and literal in a semantic token fail with the token path.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkAliases } from './aliases.mjs';
import { loadContexts, resolveAll, mergeTokens, flattenFile } from '../lib/tokens.mjs';
import { copyDesign, editJson, rm, find } from './test-helpers.mjs';

test('the repository tokens have no alias problem', () => {
  assert.deepEqual(checkAliases(loadContexts()), []);
});

test('alias to a missing token fails and names the token', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'game-tokens/semantic.json', (j) => { j.color.focus.$value = '{color.khong.ton.tai}'; });
    const e = find(checkAliases(loadContexts(dir)), 'alias-missing', 'color.focus');
    assert.ok(e);
    assert.match(e.message, /color\.khong\.ton\.tai/);
    assert.equal(e.file, 'game-tokens/semantic.json');
  } finally {
    rm(dir);
  }
});

test('alias cycle fails with the chain', () => {
  const { tokens } = flattenFile({
    a: { $type: 'color', $value: '{b}', $description: 'a' },
    b: { $type: 'color', $value: '{c}', $description: 'b' },
    c: { $type: 'color', $value: '{a}', $description: 'c' },
  }, 'x.json');
  const { errors } = resolveAll(mergeTokens(['x.json'], new Map([['x.json', tokens]])));
  assert.equal(errors.length, 1);
  assert.equal(errors[0].rule, 'alias-cycle');
  assert.match(errors[0].message, /a → b → c → a|b → c → a → b|c → a → b → c/);
});

test('cycle inside the repository layout is reported by checkAliases', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'game-tokens/semantic.json', (j) => {
      j.color.accent.primary.$value = '{color.accent.primary_pressed}';
      j.color.accent.primary_pressed.$value = '{color.accent.primary}';
    });
    assert.ok(find(checkAliases(loadContexts(dir)), 'alias-cycle'));
  } finally {
    rm(dir);
  }
});

test('a literal in a semantic, component or mode token fails', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'game-tokens/semantic.json', (j) => { j.color.surface.panel.$value = '#FFFFFF'; });
    editJson(dir, 'admin-tokens/modes/compact.json', (j) => { j.antd.fontSize.$value = '13px'; });
    editJson(dir, 'game-tokens/art3d.json', (j) => { j.vfx.correct.gradient.$value[0].color = '#00FF00'; });
    const errors = checkAliases(loadContexts(dir));
    assert.ok(find(errors, 'literal-in-semantic', 'color.surface.panel'));
    assert.ok(find(errors, 'literal-in-semantic', 'antd.fontSize'));
    assert.ok(find(errors, 'literal-in-semantic', 'vfx.correct.gradient'));
  } finally {
    rm(dir);
  }
});

test('literals are allowed in primitive files', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'game-tokens/primitive.json', (j) => { j.palette.white.$value = '#FEFEFE'; });
    assert.equal(find(checkAliases(loadContexts(dir)), 'literal-in-semantic'), undefined);
  } finally {
    rm(dir);
  }
});

test('light and dark admin modes must declare the same tokens', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'admin-tokens/modes/dark.json', (j) => { delete j.ops.mode.border; });
    assert.ok(find(checkAliases(loadContexts(dir)), 'mode-parity', 'ops.mode.border'));
  } finally {
    rm(dir);
  }
});
