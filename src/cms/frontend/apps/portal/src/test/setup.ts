// Vitest setup (portal). Test doubles below are labelled MOCK (AGENTS R17): jsdom lacks matchMedia.
import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

afterEach(() => cleanup());

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
