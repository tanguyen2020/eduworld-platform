// Theme and density of the signed-in portal for the visual suites (TC-UI-004): light, dark and compact are switched
// with the header button and the user-menu switch, which apply without a reload (ADMIN_PORTAL §1.3, F051-AC-17) —
// a reload would read /me/tenants again (EW_RATE_SESSION_PER_MIN). Every context of `openSession` starts from the
// default look (light, default density), see `session.ts`.
import type { Page } from '@playwright/test';

export type Appearance = 'light' | 'dark' | 'compact';
export const APPEARANCES: readonly Appearance[] = ['light', 'dark', 'compact'];

const TO_DARK = 'Chuyển sang giao diện tối';
const TO_LIGHT = 'Chuyển sang giao diện sáng';
/** User-menu button (accessible name starts with "Tài khoản", then the display name). */
const USER_MENU = /^Tài khoản/;

async function setTheme(page: Page, theme: 'light' | 'dark'): Promise<void> {
  const button = page.getByRole('button', { name: theme === 'dark' ? TO_DARK : TO_LIGHT });
  if (await button.isVisible().catch(() => false)) await button.click();
  await page.getByRole('button', { name: theme === 'dark' ? TO_LIGHT : TO_DARK }).waitFor({ timeout: 10_000 });
}

async function setCompact(page: Page, compact: boolean): Promise<void> {
  const menu = page.getByRole('button', { name: USER_MENU });
  const popover = page.locator('.ant-popover:not(.ant-popover-hidden)');
  await menu.click();
  const sw = popover.getByRole('switch', { name: 'Mật độ gọn' });
  await sw.waitFor({ state: 'visible', timeout: 10_000 });
  if ((await sw.getAttribute('aria-checked')) !== String(compact)) {
    await sw.click();
    await page.waitForFunction((want) => document.querySelector('[aria-labelledby="ew-compact-label"]')?.getAttribute('aria-checked') === want, String(compact));
  }
  // the popover opens and closes on its trigger (trigger="click")
  await menu.click();
  await popover.waitFor({ state: 'detached', timeout: 10_000 }).catch(() => undefined);
  await page.locator('.ant-popover:not(.ant-popover-hidden)').waitFor({ state: 'hidden', timeout: 10_000 }).catch(() => undefined);
}

/** Applies one appearance (light, dark = dark theme + default density, compact = light theme + compact density). */
export async function applyAppearance(page: Page, a: Appearance): Promise<void> {
  await setTheme(page, a === 'dark' ? 'dark' : 'light');
  await setCompact(page, a === 'compact');
  await page.mouse.move(0, 0);
}
