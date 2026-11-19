// F005-AC-01: design/ layout (DESIGN_SYSTEM §2) and W3C token properties.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { checkStructure } from './structure.mjs';
import { copyDesign, editJson, rm, find } from './test-helpers.mjs';

test('the repository design/ passes the structure check', () => {
  assert.deepEqual(checkStructure(), []);
});

test('a missing density file or icons directory fails', () => {
  const dir = copyDesign();
  try {
    fs.rmSync(path.join(dir, 'game-tokens/density/g4_5.json'));
    fs.rmSync(path.join(dir, 'icons/admin'), { recursive: true });
    const errors = checkStructure(dir);
    assert.ok(errors.some((e) => e.file === 'game-tokens/density/g4_5.json'));
    assert.ok(errors.some((e) => e.file === 'icons/admin/'));
  } finally {
    rm(dir);
  }
});

test('a token without $description or $type fails with its path', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'admin-tokens/semantic.json', (j) => {
      delete j.math['font-scale'].$description;
      delete j.math['font-scale'].$type;
    });
    const errors = checkStructure(dir);
    assert.match(find(errors, 'structure', 'math.font-scale').message, /\$description|\$type/);
    assert.equal(errors.filter((e) => e.path === 'math.font-scale').length, 2);
  } finally {
    rm(dir);
  }
});

test('an unknown $type fails', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'game-tokens/primitive.json', (j) => { j.size.font['16'].$type = 'pixels'; });
    assert.match(find(checkStructure(dir), 'structure', 'size.font.16').message, /unknown \$type/);
  } finally {
    rm(dir);
  }
});
