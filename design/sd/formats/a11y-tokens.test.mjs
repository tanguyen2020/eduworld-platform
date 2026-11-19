// F005-AC-11: text.expansion_allowance (text_expansion_allowance = 1.35, CONFIG_DEFAULTS) in UiTokens and
// math.font-scale in the CSS variables.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { generate } from '../../lib/pipeline.mjs';
import { OUTPUTS } from '../../style-dictionary.config.mjs';

test('UiTokens.TEXT_EXPANSION_ALLOWANCE and --ew-math-font-scale are generated from the sources', async () => {
  const { files, resolved } = await generate();
  assert.equal(resolved.game.get('text.expansion_allowance').value, 1.35);
  assert.match(files.get(OUTPUTS.uiTokens), /^const TEXT_EXPANSION_ALLOWANCE: float = 1\.35$/m);
  assert.match(files.get(OUTPUTS.uiTokens), /^const TEXT_SECONDARY_LANGUAGE_SCALE: float = 0\.85$/m);
  const css = files.get(OUTPUTS.tokensCss);
  assert.match(css, /--ew-math-font-scale: 1;/);
  assert.match(css, /--ew-math-font-scale-block: 1\.15;/);
});
