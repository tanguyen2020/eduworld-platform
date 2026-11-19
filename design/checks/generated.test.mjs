// F005-AC-02, F005-AC-03 (TC-UI-001): fresh outputs match; a hand edit, a missing or a stale file is listed.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { checkMarkers, compareWithDisk } from './generated.mjs';
import { generate, writeFiles } from '../lib/pipeline.mjs';
import { REPO_ROOT } from '../lib/tokens.mjs';
import { OUTPUTS } from '../style-dictionary.config.mjs';
import { find } from './test-helpers.mjs';

async function freshTree() {
  const { files } = await generate();
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'ew-gen-'));
  writeFiles(files, root);
  return { files, root };
}

test('committed generated files are up to date', async () => {
  const { files } = await generate();
  assert.deepEqual(compareWithDisk(files, REPO_ROOT), []);
});

test('every text output starts with the GENERATED marker', async () => {
  const { files } = await generate();
  assert.deepEqual(checkMarkers(files), []);
  assert.ok(checkMarkers(new Map([['x.ts', 'export const a = 1;\n']])).length === 1);
});

test('a hand edit in tokens.css is listed as drift', async () => {
  const { files, root } = await freshTree();
  try {
    assert.deepEqual(compareWithDisk(files, root), []);
    const css = path.join(root, OUTPUTS.tokensCss);
    fs.writeFileSync(css, fs.readFileSync(css, 'utf8').replace('--ew-antd-color-primary: #2A62F0;', '--ew-antd-color-primary: #FF0000;'));
    const errors = compareWithDisk(files, root);
    assert.equal(errors.length, 1);
    assert.equal(find(errors, 'generated-drift').file, OUTPUTS.tokensCss);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

test('a missing output and a stale file in an owned directory are listed; Godot sidecars are ignored', async () => {
  const { files, root } = await freshTree();
  try {
    fs.rmSync(path.join(root, OUTPUTS.uiTokens));
    fs.writeFileSync(path.join(root, OUTPUTS.artDir, 'env_old.tres'), 'stale');
    fs.writeFileSync(path.join(root, OUTPUTS.godotFontsDir, 'Baloo2-Bold.ttf.import'), 'sidecar');
    fs.writeFileSync(path.join(root, OUTPUTS.artDir, 'art3d_tokens.gd.uid'), 'uid://x');
    const errors = compareWithDisk(files, root);
    assert.equal(find(errors, 'generated-missing').file, OUTPUTS.uiTokens);
    assert.equal(find(errors, 'generated-stale').file, `${OUTPUTS.artDir}/env_old.tres`);
    assert.equal(errors.length, 2);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});
