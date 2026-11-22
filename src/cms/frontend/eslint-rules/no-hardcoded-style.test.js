// F050-AC-06 (TC-UI-003): ESLint ew/no-hardcoded-style and stylelint reject color/font/size/spacing literals;
// generated token files are ignored.
import { RuleTester, ESLint } from 'eslint';
import stylelint from 'stylelint';
import tseslint from 'typescript-eslint';
import { describe, it, expect } from 'vitest';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import rule from './no-hardcoded-style.js';
import stylelintConfig from '../stylelint.config.mjs';

RuleTester.describe = describe;
RuleTester.it = it;
RuleTester.itOnly = it.only;

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const tester = new RuleTester({
  languageOptions: { parser: tseslint.parser, parserOptions: { ecmaFeatures: { jsx: true } } },
});

tester.run('ew/no-hardcoded-style', rule, {
  valid: [
    { code: 'const s = { color: token.colorText, padding: token.padding };' },
    { code: 'const s = { margin: 0, color: "inherit", background: "transparent" };' },
    { code: 'const s = { color: "var(--ew-ops-color-text-primary)" };' },
    { code: 'const el = <span style={{ backgroundColor: domainColors[key][mode] }} />;' },
    { code: 'const id = "#main";' },
    { code: 'const x = { color: "#ABCDEF" };', options: [{ allow: ['#ABCDEF'] }] },
    // Tailwind (ADR-0098): theme classes, arbitrary variants and token-variable arbitrary values
    { code: 'const el = <div className="h-dvh overflow-hidden md:p-4 bg-sider text-sider-fg" />;' },
    { code: 'const el = <div className="[&>a]:p-2 bg-[var(--ew-ops-color-sider-bg)] text-[color:var(--ew-x)]" />;' },
    { code: 'const sel = document.querySelector("[data-theme=dark] input[type=text]");' },
  ],
  invalid: [
    { code: 'const el = <div style={{ color: "#fff" }} />;', errors: [{ messageId: 'color' }] },
    { code: 'const s = { background: "rgb(0, 0, 0)" };', errors: [{ messageId: 'color' }] },
    { code: 'const s = { color: "red" };', errors: [{ messageId: 'style' }] },
    { code: 'const s = { fontSize: 14 };', errors: [{ messageId: 'style' }] },
    { code: 'const s = { fontFamily: "Arial" };', errors: [{ messageId: 'style' }] },
    { code: 'const s = { padding: "8px 16px" };', errors: [{ messageId: 'style' }] },
    { code: 'const s = { marginTop: -4 };', errors: [{ messageId: 'style' }] },
    { code: 'const theme = { token: { colorPrimary: "#2F6BFF" } };', errors: [{ messageId: 'color' }] },
    { code: 'const css = `border: 1px solid #ccc`;', errors: [{ messageId: 'color' }] },
    { code: 'const el = <div className="bg-[#fff]" />;', errors: [{ messageId: 'color' }] },
    { code: 'const el = <div className="flex p-[13px]" />;', errors: [{ messageId: 'tailwind' }] },
    { code: 'const el = <div className="md:hover:text-[14px]" />;', errors: [{ messageId: 'tailwind' }] },
    { code: 'const el = <div className="[color:red]" />;', errors: [{ messageId: 'tailwind' }] },
    { code: 'const cls = `${a} !-mt-[2px]`;', errors: [{ messageId: 'tailwind' }] },
    { code: 'const el = <div className="[&>a]:w-[37px]" />;', errors: [{ messageId: 'tailwind' }] },
  ],
});

describe('project ESLint config', () => {
  const eslint = new ESLint({ cwd: ROOT });

  it('reports a hardcoded color in a portal TSX file', async () => {
    const [res] = await eslint.lintText('export const A = () => <div style={{ color: "#fff" }} />;\n', {
      filePath: path.join(ROOT, 'apps/portal/src/app/Probe.tsx'),
    });
    expect(res.messages.map((m) => m.ruleId)).toContain('ew/no-hardcoded-style');
  });

  it('ignores the generated antd-theme.ts', async () => {
    expect(await eslint.isPathIgnored(path.join(ROOT, 'packages/ops-ui/src/theme/antd-theme.ts'))).toBe(true);
  });
});

describe('stylelint', () => {
  const lint = (code, file = 'packages/ops-ui/src/components/x.css') =>
    stylelint.lint({ code, config: stylelintConfig, codeFilename: path.join(ROOT, file) });

  it('rejects hex, named and rgb colors, literal font size, font family and spacing', async () => {
    const { results } = await lint('.a { color: #fff; background: red; border-color: rgb(0 0 0); font-size: 14px; font-family: Arial; margin: 8px; }\n');
    const rules = results[0].warnings.map((w) => w.rule);
    expect(rules).toEqual(expect.arrayContaining(['color-no-hex', 'color-named', 'function-disallowed-list', 'declaration-property-value-allowed-list']));
    expect(results[0].warnings.filter((w) => w.rule === 'declaration-property-value-allowed-list')).toHaveLength(3);
  });

  it('accepts token variables', async () => {
    const { results } = await lint('.a {\n  margin: var(--ew-space-sm) 0;\n  color: var(--ew-ops-color-text-primary);\n  font-size: var(--ew-antd-font-size);\n}\n');
    expect(results[0].warnings).toEqual([]);
  });

  it('Tailwind (ADR-0098): @tailwind directives pass, @apply and theme() in CSS are rejected', async () => {
    const ok = await lint('@tailwind base;\n@tailwind utilities;\n', 'apps/portal/src/app/tailwind.css');
    expect(ok.results[0].warnings).toEqual([]);
    const { results } = await lint('.a {\n  @apply p-2;\n  color: theme("colors.primary");\n}\n', 'apps/portal/src/app/x.css');
    expect(results[0].warnings.map((w) => w.rule)).toEqual(expect.arrayContaining(['at-rule-disallowed-list', 'function-disallowed-list']));
  });

  it('ignores the generated tokens.css', async () => {
    const { results } = await lint(':root { --ew-x: #fff; }\n', 'packages/ops-ui/src/theme/tokens.css');
    expect(results[0]?.warnings ?? []).toEqual([]);
  });
});
