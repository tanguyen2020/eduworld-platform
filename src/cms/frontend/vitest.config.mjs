// Vitest projects for src/cms/frontend (F050). Run with one worker: `pnpm test` (--maxWorkers=1).
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    projects: [
      'apps/portal',
      'packages/ops-ui',
      {
        test: {
          name: 'eslint-rules',
          testTimeout: 60_000,
          environment: 'node',
          include: ['eslint-rules/**/*.test.js', 'tooling/**/*.test.mjs'],
        },
      },
    ],
  },
});
