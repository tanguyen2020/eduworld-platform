// F005-AC-07 (TC-UI-010): Godot outputs carry no Admin UI / admin token, web outputs carry no game token.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkSeparationOutputs, checkSeparationSources, isGodotOutput, isWebOutput } from './separation.mjs';
import { loadContexts } from '../lib/tokens.mjs';
import { generate } from '../lib/pipeline.mjs';
import { OUTPUTS } from '../style-dictionary.config.mjs';
import { copyDesign, editJson, rm, find } from './test-helpers.mjs';

test('generated outputs of the repository tokens are separated', async () => {
  const loaded = loadContexts();
  const { files } = await generate({ loaded });
  assert.deepEqual(checkSeparationOutputs(files, loaded.byFile), []);
  for (const [rel, content] of files) {
    if (Buffer.isBuffer(content)) continue;
    if (isGodotOutput(rel)) {
      assert.doesNotMatch(content, /antd|ConfigProvider/, rel);
    }
    if (isWebOutput(rel)) {
      assert.doesNotMatch(content, /game-tokens|g1_3|toon|UiTokens/, rel);
    }
  }
});

test('a game token leaking into tokens.css is reported', async () => {
  const loaded = loadContexts();
  const { files } = await generate({ loaded });
  files.set(OUTPUTS.tokensCss, `${files.get(OUTPUTS.tokensCss)}:root { --ew-density-g1-3-font-base: 24px; }\n`);
  assert.ok(find(checkSeparationOutputs(files, loaded.byFile), 'separation', 'density.g1_3.font.base'));
});

test('Ant Design or an admin token inside a Godot output is reported', async () => {
  const loaded = loadContexts();
  const { files } = await generate({ loaded });
  files.set(OUTPUTS.uiTokens, `${files.get(OUTPUTS.uiTokens)}\n# ConfigProvider\nconst ANTD_COLOR_PRIMARY: Color = Color(0, 0, 1, 1)\n`);
  const errors = checkSeparationOutputs(files, loaded.byFile);
  assert.ok(errors.some((e) => /ConfigProvider/.test(e.message)));
  assert.ok(find(errors, 'separation', 'antd.colorPrimary'));
});

test('the same token name in game-tokens and admin-tokens fails', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'admin-tokens/semantic.json', (j) => {
      j.motion = { press: { $type: 'duration', $value: '{ops.number.scale.100}', $description: 'clash' } };
    });
    assert.ok(find(checkSeparationSources(loadContexts(dir).byFile), 'separation', 'motion.press'));
  } finally {
    rm(dir);
  }
});
