// Playwright E2E of the portal against a REAL stack (make up && make seed && make up-app && pnpm dev), not the MOCK
// harness of the Vitest suites. Pending OQ F051-9 (no CI job yet): run by hand with
//   EW_E2E_PORTAL_URL=<portal url> EW_E2E_PASSWORD=<demo password> EW_E2E_SA_OTP=<TOTP of sa.admin> \
//   npx playwright test -c apps/portal/tests/e2e/playwright.config.ts --grep @F035
// Every URL and credential comes from the environment (AGENTS R3); nothing is hardcoded.
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: /.*\.spec\.ts$/,
  workers: 1,
  fullyParallel: false,
  reporter: [['list']],
  outputDir: '../../../../test-results/e2e',
  use: { ...devices['Desktop Chrome'], baseURL: process.env.EW_E2E_PORTAL_URL, locale: 'vi-VN' },
});
