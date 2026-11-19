// Visual regression of ops-ui stories (F050-AC-20, TC-UI-004): static Storybook served locally, one worker.
// Baselines live per platform under __screenshots__/<platform>/; a diff beyond the threshold fails and the new
// image must be reviewed and committed (`pnpm test:visual --update-snapshots`).
import { defineConfig, devices } from '@playwright/test';

const PORT = 6008;

export default defineConfig({
  testDir: '.',
  testMatch: /.*\.spec\.ts$/,
  workers: 1,
  fullyParallel: false,
  reporter: [['list'], ['html', { open: 'never', outputFolder: '../../playwright-report' }]],
  outputDir: '../../test-results',
  snapshotPathTemplate: '{testDir}/__screenshots__/{platform}/{arg}{ext}',
  expect: { toHaveScreenshot: { maxDiffPixelRatio: 0.01, animations: 'disabled', caret: 'hide' } },
  use: { ...devices['Desktop Chrome'], baseURL: `http://127.0.0.1:${PORT}` },
  webServer: {
    command: `npx http-server packages/ops-ui/storybook-static -p ${PORT} -a 127.0.0.1 -s`,
    cwd: '../..',
    url: `http://127.0.0.1:${PORT}/index.json`,
    reuseExistingServer: !process.env.CI,
  },
});
