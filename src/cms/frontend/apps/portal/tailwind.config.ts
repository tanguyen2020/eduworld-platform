// Tailwind CSS v3 as the layout/utility layer of the Admin Portal shell (ADR-0098, complements ADR-0047, AGENTS R32).
// Ant Design stays the component library and the source of every component style. Rules:
//   - Every class carries the `tw-` prefix: without it, words of the sources (`hidden`, `table`, `fixed`, `transition`)
//     would become global utilities that hit third-party DOM (MathLive, MathJax) sharing those class names.
//   - Preflight is off (it fights the Ant Design reset); base only emits the --tw-* defaults of the utilities in use.
//   - colors, spacing, fontSize, fontFamily, borderRadius REPLACE the Tailwind defaults and map to the generated
//     admin token variables `var(--ew-*)` (packages/ops-ui/src/theme/tokens.css, built from design/admin-tokens):
//     no raw hex/px palette is reachable from a class. Arbitrary values (`bg-[#…]`, `p-[13px]`) are rejected by
//     ESLint ew/no-hardcoded-style; `tailwind.test.ts` checks this config and the generated CSS.
//   - Plugins whose defaults carry raw colours or sizes (shadow, ring, blur, …) are disabled.
//   - Screens mirror the shell breakpoints (ADMIN_PORTAL §1.5): md = Ant Design screenMD, xl = DESKTOP_MIN_WIDTH_PX.
//     Media queries cannot read CSS variables, so these two are the only lengths written here.
import { fileURLToPath } from 'node:url';
import type { Config } from 'tailwindcss';

/** Content globs resolved against this file, so the scan works whatever the working directory is. */
const HERE = fileURLToPath(new URL('.', import.meta.url)).split('\\').join('/');

const v = (name: string) => `var(--ew-${name})`;

export const TAILWIND_SCREENS = { md: '768px', xl: '1280px' } as const;

export default {
  prefix: 'tw-',
  content: [`${HERE}index.html`, `${HERE}src/**/*.{ts,tsx}`],
  corePlugins: {
    preflight: false,
    container: false,
    boxShadow: false,
    boxShadowColor: false,
    dropShadow: false,
    ringWidth: false,
    ringColor: false,
    ringOpacity: false,
    ringOffsetWidth: false,
    ringOffsetColor: false,
    blur: false,
    backdropBlur: false,
    backdropFilter: false,
    filter: false,
  },
  theme: {
    screens: TAILWIND_SCREENS,
    colors: {
      transparent: 'transparent',
      current: 'currentColor',
      inherit: 'inherit',
      primary: v('antd-color-primary'),
      layout: v('ops-color-bg-layout'),
      container: v('ops-color-bg-container'),
      fg: {
        DEFAULT: v('ops-color-text-primary'),
        secondary: v('ops-color-text-secondary'),
        'on-primary': v('ops-color-text-on-primary'),
      },
      line: {
        DEFAULT: v('ops-color-border'),
        subtle: v('ops-color-border-subtle'),
      },
      sider: {
        DEFAULT: v('ops-color-sider-bg'),
        hover: v('ops-color-sider-hover-bg'),
        selected: v('ops-color-sider-selected-bg'),
        fg: v('ops-color-sider-text-default'),
        'fg-selected': v('ops-color-sider-text-selected'),
      },
    },
    spacing: {
      0: '0',
      1: v('ops-size-space-4'),
      2: v('ops-size-space-8'),
      3: v('ops-size-space-12'),
      4: v('ops-size-space-16'),
      6: v('ops-size-space-24'),
      8: v('ops-size-space-32'),
      12: v('ops-size-space-48'),
    },
    fontSize: {
      sm: v('antd-font-size-sm'),
      base: v('antd-font-size'),
      lg: v('ops-size-font-16'),
      xl: v('ops-size-font-18'),
      '2xl': v('ops-size-font-20'),
      '3xl': v('ops-size-font-24'),
      '4xl': v('ops-size-font-28'),
    },
    fontFamily: {
      sans: v('antd-font-family'),
    },
    borderRadius: {
      none: '0',
      DEFAULT: v('antd-border-radius'),
      lg: v('antd-border-radius-lg'),
    },
    borderWidth: {
      0: '0',
      accent: v('dashboard-accent-width'),
    },
  },
  plugins: [],
} satisfies Config;
