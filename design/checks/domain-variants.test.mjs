// F005-AC-05, F005-AC-06 (TC-UI-008): business colors only in shared-tokens, three variants each, and the
// web and Godot outputs carry the same names with the matching variant value.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkDomainSources, checkDomainOutputs, sharedBusinessColors, parseWebDomainColors, parseGodotDomainColors } from './domain-variants.mjs';
import { loadContexts } from '../lib/tokens.mjs';
import { generate } from '../lib/pipeline.mjs';
import { godotColor } from '../lib/color.mjs';
import { OUTPUTS } from '../style-dictionary.config.mjs';
import { copyDesign, editJson, rm, find } from './test-helpers.mjs';

test('repository business colors are valid', () => {
  assert.deepEqual(checkDomainSources(loadContexts().byFile), []);
});

test('a business color declared in admin-tokens or game-tokens fails', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'admin-tokens/semantic.json', (j) => {
      j.color = { subject: { math: { $type: 'color', $value: '{ops.palette.white}', $description: 'redeclared' } } };
    });
    editJson(dir, 'game-tokens/semantic.json', (j) => {
      j.color.rarity = { epic: { $value: '{palette.white}', $description: 'redeclared' } };
    });
    const errors = checkDomainSources(loadContexts(dir).byFile);
    assert.equal(find(errors, 'business-color-outside-shared', 'color.subject.math').file, 'admin-tokens/semantic.json');
    assert.equal(find(errors, 'business-color-outside-shared', 'color.rarity.epic').file, 'game-tokens/semantic.json');
  } finally {
    rm(dir);
  }
});

test('a business color without its dark variant fails', () => {
  const dir = copyDesign();
  try {
    editJson(dir, 'shared-tokens/domain.json', (j) => { delete j.color.rarity.epic.$extensions.eduworld.variants.dark; });
    const e = find(checkDomainSources(loadContexts(dir).byFile), 'business-variant-missing', 'color.rarity.epic');
    assert.match(e.message, /"dark"/);
  } finally {
    rm(dir);
  }
});

test('web and Godot outputs have the same names and the variant values (TC-UI-008)', async () => {
  const loaded = loadContexts();
  const shared = sharedBusinessColors(loaded.byFile);
  const { files } = await generate({ loaded });
  assert.deepEqual(checkDomainOutputs(files, shared), []);
  const web = parseWebDomainColors(files.get(OUTPUTS.antdTheme));
  const godot = parseGodotDomainColors(files.get(OUTPUTS.uiTokens));
  assert.deepEqual([...web.keys()].sort(), [...shared.keys()].sort());
  assert.deepEqual([...godot.keys()].sort(), [...shared.keys()].sort());
  const math = shared.get('color.subject.math');
  assert.deepEqual(web.get('color.subject.math'), { light: math.light, dark: math.dark });
  assert.equal(godot.get('color.subject.math'), godotColor(math.game));
});

test('a tampered domainColors value is reported', async () => {
  const loaded = loadContexts();
  const { files } = await generate({ loaded });
  const shared = sharedBusinessColors(loaded.byFile);
  const ts = files.get(OUTPUTS.antdTheme).replace(/("color\.subject\.math": \{\s*light: )"#[0-9A-F]+"/, '$1"#000001"');
  assert.notEqual(ts, files.get(OUTPUTS.antdTheme));
  files.set(OUTPUTS.antdTheme, ts);
  assert.ok(find(checkDomainOutputs(files, shared), 'business-parity', 'color.subject.math'));
});
