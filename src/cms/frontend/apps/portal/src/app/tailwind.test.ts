// @vitest-environment node
// ADR-0097 (Tailwind as the layout/utility layer, complements ADR-0047, AGENTS R32): Preflight is off; colours,
// spacing, font sizes, font family and radii come only from the admin token variables `var(--ew-*)`; the CSS that
// Tailwind generates for the portal sources holds no colour literal and no length outside the breakpoint queries;
// every token variable the theme refers to exists in the generated tokens.css.
import fs from 'node:fs';
import path from 'node:path';
import postcss from 'postcss';
import tailwindcss from 'tailwindcss';
import { describe, expect, it } from 'vitest';
import config, { TAILWIND_SCREENS } from '../../tailwind.config';

const TOKENS_CSS = path.resolve(__dirname, '../../../../packages/ops-ui/src/theme/tokens.css');
const TOKEN = /^var\(--ew-[a-z0-9-]+\)$/;
const KEYWORDS = new Set(['0', 'transparent', 'currentColor', 'inherit']);

function leaves(obj: unknown, prefix = ''): [string, string][] {
  if (typeof obj === 'string') return [[prefix, obj]];
  return Object.entries(obj as Record<string, unknown>).flatMap(([k, v]) => leaves(v, prefix ? `${prefix}.${k}` : k));
}

describe('tailwind.config (ADR-0097)', () => {
  it('Preflight is disabled (Ant Design keeps its own reset) and every class carries the tw- prefix', () => {
    expect(config.corePlugins).toMatchObject({ preflight: false });
    expect(config.prefix).toBe('tw-');
  });

  it.each(['colors', 'spacing', 'fontSize', 'fontFamily', 'borderRadius', 'borderWidth'] as const)(
    'theme.%s replaces the defaults and maps only to admin token variables',
    (key) => {
      const values = leaves(config.theme[key]);
      expect(values.length).toBeGreaterThan(0);
      for (const [name, value] of values) expect(KEYWORDS.has(value) || TOKEN.test(value), `${key}.${name} = ${value}`).toBe(true);
    },
  );

  it('every referenced token variable exists in the generated tokens.css', () => {
    const css = fs.readFileSync(TOKENS_CSS, 'utf8');
    const refs = (['colors', 'spacing', 'fontSize', 'fontFamily', 'borderRadius', 'borderWidth'] as const)
      .flatMap((k) => leaves(config.theme[k]))
      .map(([, v]) => /^var\((--ew-[a-z0-9-]+)\)$/.exec(v)?.[1])
      .filter((v): v is string => !!v);
    for (const ref of refs) expect(css, ref).toContain(`${ref}:`);
  });

  it('the generated utilities hold no colour literal and no length outside the breakpoint media queries', async () => {
    const src = '@tailwind base;\n@tailwind components;\n@tailwind utilities;\n';
    const { css } = await postcss([tailwindcss(config)]).process(src, { from: undefined });
    // the shell classes are in the output (content scan works)
    for (const cls of ['tw-h-dvh', 'tw-overflow-hidden', 'tw-overflow-y-auto', 'tw-min-h-0']) expect(css).toContain(`.${cls}`);
    // no unprefixed utility leaks from words of the sources (hidden, table, fixed, transition…)
    expect(css.match(/(?<![\w-])\.(?!tw-)[a-z][a-z0-9-]*\s*\{/g) ?? []).toEqual([]);
    expect(css).not.toMatch(/#[0-9a-fA-F]{3,8}\b/);
    expect(css).not.toMatch(/\b(?:rgba?|hsla?|hwb|oklch|oklab|lab|lch)\(/);
    const withoutMedia = css.replace(/@media[^{]*\{/g, '@media{');
    expect(withoutMedia).not.toMatch(/\d+(?:\.\d+)?px/);
    // breakpoints = shell breakpoints (ADMIN_PORTAL §1.5)
    expect(Object.values(TAILWIND_SCREENS)).toEqual(['768px', '1280px']);
  });
});
