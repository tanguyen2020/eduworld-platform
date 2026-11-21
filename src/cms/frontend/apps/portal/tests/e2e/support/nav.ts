// In-app navigation helpers of the Playwright E2E suites (real stack, OQ F051-9). A `page.goto` reloads the portal,
// and every boot reads /me/tenants, whose gateway limit is per user and minute (APISIX `ew-api-rate`, profile
// `session`): a suite that reloads on each screen ends in 429 RATE_LIMITED. After sign-in the suites move with the
// router (history + popstate) as the sidebar does.
import type { Page } from '@playwright/test';

/** Moves the signed-in portal to `path` without reloading it (React Router listens to popstate). */
export async function spaGoto(page: Page, path: string): Promise<void> {
  await page.evaluate((p) => {
    window.history.pushState({}, '', p);
    window.dispatchEvent(new PopStateEvent('popstate'));
  }, path);
  await page.waitForURL((u) => `${u.pathname}${u.search}`.startsWith(path), { timeout: 15_000 }).catch(() => undefined);
}

/** Opens an antd select by its accessible name (the selected item covers the input, hence `force`). */
export async function openSelect(page: Page, label: string): Promise<void> {
  await page.getByRole('combobox', { name: label, exact: true }).click({ force: true, timeout: 10_000 });
}

/** Options of the open antd dropdown. */
export function openOptions(page: Page) {
  return page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').last().locator('.ant-select-item-option');
}
