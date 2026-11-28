// Sign-in helper of the Playwright E2E suites (real stack, OQ F051-9). The portal form has only username + password
// for every account, platform staff included (owner 2026-11-21, ADR-0094: no 6-digit code). Every credential comes
// from the environment (AGENTS R3, R4).
import type { Page } from '@playwright/test';

export const e2eReady = !!process.env.EW_E2E_PORTAL_URL && !!process.env.EW_E2E_PASSWORD;

/** Opens /login, signs in with the demo password of the environment and waits for the outcome. */
export async function signIn(page: Page, username: string): Promise<void> {
  // the /login page is always English (owner 2026-11-23, ADMIN_PORTAL §1.3b — `loginLocale.tsx`)
  await page.goto('/login');
  await page.getByLabel('Username or email', { exact: true }).fill(username);
  await page.getByLabel('Password', { exact: true }).fill(process.env.EW_E2E_PASSWORD ?? '');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  // the sign-in is finished when the portal leaves /login or shows its answer (error, student message)
  await Promise.race([
    page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 30_000 }),
    page.getByRole('alert').first().waitFor({ timeout: 30_000 }),
  ]);
}
