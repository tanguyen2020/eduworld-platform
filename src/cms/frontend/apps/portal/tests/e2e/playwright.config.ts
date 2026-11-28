// Playwright E2E of the portal against a REAL stack (make up && make seed && make up-app && pnpm dev), not the MOCK
// harness of the Vitest suites. Pending OQ F051-9 (no CI job yet): run by hand with
//   EW_E2E_PORTAL_URL=<portal url> EW_E2E_PASSWORD=<demo password> \
//   npx playwright test -c apps/portal/tests/e2e/playwright.config.ts --grep @F035
// Every account signs in with username + password only (ADR-0094, support/auth.ts). Screenshots of every portal screen:
//   EW_E2E_SCREENSHOT_DIR=<folder outside the repo> ... --grep @screenshots
// Visual regression of real screens (@visual, F055 T043, F056 T046): baselines per platform under
// __screenshots__/<platform>/, data regions masked; after a reviewed change of the screen or of the seed:
//   ... --grep @visual --update-snapshots
// Every URL and credential comes from the environment (AGENTS R3); nothing is hardcoded. No trace, video or
// screenshot on failure is recorded (the login form holds the password — AGENTS R4).
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: /.*\.spec\.ts$/,
  workers: 1,
  fullyParallel: false,
  reporter: [['list']],
  outputDir: '../../../../test-results/e2e',
  snapshotPathTemplate: '{testDir}/__screenshots__/{platform}/{arg}{ext}',
  expect: { toHaveScreenshot: { maxDiffPixelRatio: 0.01, animations: 'disabled', caret: 'hide' } },
  use: { ...devices['Desktop Chrome'], baseURL: process.env.EW_E2E_PORTAL_URL, locale: 'vi-VN', trace: 'off', video: 'off', screenshot: 'off' },
});
