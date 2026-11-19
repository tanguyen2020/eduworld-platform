// Local ESLint plugin `ew` (F050): design-system and configuration guards for src/cms/frontend.
import noHardcodedStyle from './no-hardcoded-style.js';
import noGameImports from './no-game-imports.js';
import noEnvUrlLiteral from './no-env-url-literal.js';

export default {
  meta: { name: 'eslint-plugin-ew', version: '0.1.0' },
  rules: {
    'no-hardcoded-style': noHardcodedStyle,
    'no-game-imports': noGameImports,
    'no-env-url-literal': noEnvUrlLiteral,
  },
};
