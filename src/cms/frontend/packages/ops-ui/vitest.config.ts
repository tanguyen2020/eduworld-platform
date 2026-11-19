/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { forbidGameImports } from '../../tooling/forbid-game-imports.mjs';

export default defineConfig({
  plugins: [forbidGameImports() as never, react()],
  test: {
    name: 'ops-ui',
    testTimeout: 60_000,
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}', '.storybook/**/*.test.mjs'],
    css: false,
  },
});
