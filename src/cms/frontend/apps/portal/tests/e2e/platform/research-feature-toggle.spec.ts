// F035 T071 — F035-AC-37 (TC-CMS-MENU-02), quickstart scenario 23 @F035: on the REAL stack, sa.admin turns off the
// platform defaults of menu.platform_admin.efficacy_studies and menu.platform_admin.learning_experiments (Feature
// flag screen, F059): both menus disappear after the permissions reload and the research API answers
// 403 FEATURE_DISABLED (notice "Tính năng đang tạm tắt" in the portal); the efficacy box of the question editor still
// follows action.content_studio.question_bank.edit. Turns both menus back on at the end. Needs EW_E2E_PORTAL_URL,
// EW_E2E_PASSWORD (password only for every account, ADR-0094); skipped otherwise (OQ F051-9).
import { expect, test, type Page } from '@playwright/test';
import { e2eReady as ready, signIn as signInAs } from '../support/auth';
import { openOptions, openSelect, spaGoto } from '../support/nav';

const MENUS = ['menu.platform_admin.efficacy_studies', 'menu.platform_admin.learning_experiments'];

const signIn = (page: Page) => signInAs(page, 'sa.admin');

async function setMenu(page: Page, key: string, state: 'Bật' | 'Tắt', reason: string) {
  await spaGoto(page, '/platform-admin/feature-toggles');
  // the list is long (every action, menu and feature flag, cursor pages): show the menu flags only, then load pages
  // until the row of the key is there
  await openSelect(page, 'Loại');
  await openOptions(page).filter({ hasText: /^Menu$/ }).first().click();
  const row = page.locator('tr').filter({ has: page.getByText(key, { exact: true }) });
  const more = page.getByRole('button', { name: 'Tải thêm' });
  await expect(row.or(more).first()).toBeVisible({ timeout: 30_000 });
  while (!(await row.isVisible()) && (await more.isVisible())) {
    await more.click();
    await expect(row.or(more).first()).toBeVisible({ timeout: 30_000 });
  }
  await row.getByRole('button', { name: 'Sửa' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel(state).check();
  await dialog.getByLabel('Lý do').fill(reason);
  await dialog.getByRole('button', { name: 'Lưu' }).click();
  await expect(dialog).toBeHidden();
}

test.describe('@F035 Tắt menu nghiên cứu', () => {
  test.skip(!ready, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');

  test('turned-off research menus are hidden and the API answers FEATURE_DISABLED', async ({ page }) => {
    test.setTimeout(6 * 60_000); // four flag edits and up to 120 s for the permissions to reload
    await signIn(page);
    try {
      for (const key of MENUS) await setMenu(page, key, 'Tắt', 'Kiểm thử tắt menu nghiên cứu F035');
      await page.reload();
      // wait for the session restore (refreshPortalToken) of the reload: leaving the page while that refresh is in
      // flight loses the rotated refresh cookie and the next page lands on /login
      await expect(page.getByRole('navigation', { name: 'Menu điều hướng' })).toBeVisible({ timeout: 30_000 });
      await expect(page.getByRole('link', { name: 'Nghiên cứu hiệu quả học tập' })).toHaveCount(0, { timeout: 120_000 });
      await expect(page.getByRole('link', { name: 'Thử nghiệm A/B' })).toHaveCount(0);
      await spaGoto(page, '/platform-admin/efficacy-studies');
      await expect(page.getByTestId('feature-disabled')).toBeVisible();
    } finally {
      // the platform defaults are always turned back on, from a fresh sign-in when the session was lost
      if (new URL(page.url()).pathname.startsWith('/login')) await signIn(page);
      for (const key of MENUS) await setMenu(page, key, 'Bật', 'Bật lại menu nghiên cứu sau kiểm thử F035');
    }
  });
});
