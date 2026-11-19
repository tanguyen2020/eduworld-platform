// F050-AC-07 (TC-UI-010): imports of src/game, design/game-tokens or Godot resources fail lint and build.
import { RuleTester } from 'eslint';
import tseslint from 'typescript-eslint';
import { describe, it, expect } from 'vitest';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import rule from './no-game-imports.js';
import { forbidGameImports, isGameImport } from '../tooling/forbid-game-imports.mjs';

RuleTester.describe = describe;
RuleTester.it = it;
RuleTester.itOnly = it.only;

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const FILE = path.join(ROOT, 'apps/portal/src/app/App.tsx');
const tester = new RuleTester({ languageOptions: { parser: tseslint.parser } });

tester.run('ew/no-game-imports', rule, {
  valid: [
    { code: 'import { lightTheme } from "@eduworld/ops-ui";', filename: FILE },
    { code: 'import "../../../../packages/ops-ui/src/theme/tokens.css";', filename: FILE },
    { code: 'import font from "../../../../../../../design/fonts/BeVietnamPro/BeVietnamPro-Regular.ttf";', filename: FILE },
  ],
  invalid: [
    { code: 'import x from "../../../../../../game/scripts/core/ui_tokens.gd";', filename: FILE, errors: [{ messageId: 'gameImport' }] },
    { code: 'import t from "../../../../../../../design/game-tokens/semantic.json";', filename: FILE, errors: [{ messageId: 'gameImport' }] },
    { code: 'const m = import("../../../../../../game/resources/ui/theme/eduworld_theme.tres");', filename: FILE, errors: [{ messageId: 'gameImport' }] },
    { code: 'export * from "src/game/scenes/ui";', filename: FILE, errors: [{ messageId: 'gameImport' }] },
  ],
});

describe('Vite plugin forbidGameImports', () => {
  it('fails resolution of a game path (build fails)', () => {
    const plugin = forbidGameImports();
    const ctx = { error: (msg) => { throw new Error(msg); } };
    expect(() => plugin.resolveId.call(ctx, '../../../../../../game/project.godot', FILE)).toThrow(/forbidden/);
    expect(plugin.resolveId.call(ctx, 'antd', FILE)).toBeNull();
  });

  it('resolves relative sources against the importer', () => {
    expect(isGameImport('../../../../../../game/x.ts', FILE)).toBe(true);
    expect(isGameImport('../../../../../../cms/frontend/x.ts', FILE)).toBe(false);
  });
});
