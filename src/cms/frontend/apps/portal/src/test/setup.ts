// Vitest setup (portal). Test doubles below are labelled MOCK (AGENTS R17): jsdom lacks matchMedia.
import '@testing-library/jest-dom/vitest';
import { cleanup, configure } from '@testing-library/react';
import { afterEach, vi } from 'vitest';

afterEach(() => cleanup());

// whole-portal tests render the router + Ant Design on a shared, throttled CI/dev CPU: allow 10 s for async queries
configure({ asyncUtilTimeout: 10_000 });

if (typeof window !== 'undefined' && !window.matchMedia) {
  // MOCK (test only): Ant Design responsive utilities call matchMedia, which jsdom does not implement.
  window.matchMedia = (query: string) =>
    ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }) as MediaQueryList;
}

// MOCK (test only, AGENTS R17): MathLive's <math-field> needs a real browser (layout, fonts); jsdom gets a stand-in
// module so MathInput mounts its element and the LaTeX source/preview paths are tested. The real component is
// exercised in Storybook / Playwright.
vi.mock('mathlive', () => ({ MathfieldElement: { fontsDirectory: null, soundsDirectory: null } }));
vi.mock('mathlive/fonts.css', () => ({}));

// MOCK (test only, AGENTS R17): jsdom has no 2D canvas; the chart library (ECharts SVG renderer, F056 DashboardChart)
// measures text through it and falls back to an estimate when no context is returned. Real charts: Storybook/Playwright.
if (typeof HTMLCanvasElement !== 'undefined') HTMLCanvasElement.prototype.getContext = (() => null) as typeof HTMLCanvasElement.prototype.getContext;
