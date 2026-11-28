// Signed-in browser contexts reused across the Playwright E2E suites (real stack, OQ F051-9). Every sign-in counts
// against the gateway login limits (EW_RATE_PORTAL_*, per user and device) and every boot of the portal reads
// /me/tenants (EW_RATE_SESSION_PER_MIN), so each account signs in at most once per run: the context state (HttpOnly
// refresh cookie, CSRF cookie, device id) is kept in a Playwright storage state under the git-ignored test-results
// folder and saved again when the suite closes the context (the refresh cookie rotates on each boot). No password is
// ever written: the state holds only the cookies the backend set (AGENTS R4).
import type { Browser, BrowserContext, Page } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { signIn } from './auth';

const HERE = path.dirname(fileURLToPath(import.meta.url));
/** Folder of the storage states (src/cms/frontend/test-results is git-ignored); EW_E2E_STATE_DIR overrides it. */
const STATE_DIR = process.env.EW_E2E_STATE_DIR ?? path.resolve(HERE, '../../../../../test-results/e2e-auth');

const statePath = (username: string) => path.join(STATE_DIR, `${username.replace(/[^a-z0-9._-]/gi, '_')}.json`);

export interface Session {
  username: string;
  context: BrowserContext;
  page: Page;
  /** Saves the (rotated) cookies for the next suite and closes the context. */
  close(): Promise<void>;
}

/** Heading of the tenant picker shown after sign-in to accounts with several tenants. */
export const TENANT_PICKER = 'Chọn đơn vị làm việc';

/** Waits until the portal booted: signed-in shell, tenant picker, or back on /login (state no longer valid). */
async function booted(page: Page): Promise<'login' | 'ready'> {
  await Promise.race([
    page.waitForURL((u) => u.pathname.startsWith('/login'), { timeout: 60_000 }),
    page.getByRole('navigation', { name: 'Menu điều hướng' }).first().waitFor({ timeout: 60_000 }),
    page.getByRole('heading', { name: TENANT_PICKER }).waitFor({ timeout: 60_000 }),
  ]).catch(() => undefined);
  return new URL(page.url()).pathname.startsWith('/login') ? 'login' : 'ready';
}

/**
 * Opens a browser context signed in as `username`: from the saved storage state when it still restores a session
 * (no new sign-in), otherwise through the login form once. `onContext` lets a suite attach request listeners before
 * the first page load.
 */
export async function openSession(browser: Browser, username: string, onContext?: (c: BrowserContext) => void | Promise<void>): Promise<Session> {
  const file = statePath(username);
  const reuse = fs.existsSync(file);
  const context = await browser.newContext(reuse ? { storageState: file } : {});
  // every suite starts from the default look (light, default density, Vietnamese): a visual suite may have left
  // dark/compact preferences in the saved state (`ew.portal.prefs`, app/preferences.ts)
  await context.addInitScript(() => {
    try {
      window.localStorage.setItem('ew.portal.prefs', JSON.stringify({ theme: 'light', density: 'default', locale: 'vi-VN' }));
    } catch {
      // storage unavailable: the portal falls back to its defaults
    }
  });
  await onContext?.(context);
  const page = await context.newPage();
  let signedIn = false;
  if (reuse) {
    await page.goto('/');
    signedIn = (await booted(page)) === 'ready';
  }
  if (!signedIn) await signIn(page, username);
  return {
    username,
    context,
    page,
    async close() {
      fs.mkdirSync(STATE_DIR, { recursive: true });
      await context.storageState({ path: file }).catch(() => undefined);
      await context.close();
    },
  };
}

/** Picks a tenant on the tenant picker when it is shown (accounts with several tenants). */
export async function enterTenant(page: Page, tenant: RegExp): Promise<void> {
  const picker = page.getByRole('heading', { name: TENANT_PICKER });
  await Promise.race([
    picker.waitFor({ timeout: 30_000 }),
    page.getByRole('navigation', { name: 'Menu điều hướng' }).waitFor({ timeout: 30_000 }),
  ]).catch(() => undefined);
  if (!(await picker.isVisible().catch(() => false))) return;
  await page.getByRole('radio', { name: tenant }).click();
  await page.getByRole('button', { name: 'Vào' }).click();
  await page.getByRole('navigation', { name: 'Menu điều hướng' }).waitFor({ timeout: 30_000 });
}
