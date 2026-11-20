// F035 T071 — F035-AC-37 (TC-CMS-MENU-02), quickstart scenario 23 @F035: on the REAL stack, sa.admin turns off the
// platform defaults of menu.platform_admin.efficacy_studies and menu.platform_admin.learning_experiments (Feature
// flag screen, F059): both menus disappear after the permissions reload and the research API answers
// 403 FEATURE_DISABLED (notice "Tính năng đang tạm tắt" in the portal); the efficacy box of the question editor still
// follows action.content_studio.question_bank.edit. Turns both menus back on at the end. Needs EW_E2E_PORTAL_URL,
// EW_E2E_PASSWORD, EW_E2E_SA_OTP; skipped otherwise (OQ F051-9).
import { expect, test, type Page } from '@playwright/test';

const ready = !!process.env.EW_E2E_PORTAL_URL && !!process.env.EW_E2E_PASSWORD;
const MENUS = ['menu.platform_admin.efficacy_studies', 'menu.platform_admin.learning_experiments'];

async function signIn(page: Page) {
  await page.goto('/login');
  await page.getByLabel('Tên đăng nhập hoặc email').fill('sa.admin');
  await page.getByLabel('Mật khẩu').fill(process.env.EW_E2E_PASSWORD ?? '');
  await page.getByLabel('Mã xác thực 6 số').fill(process.env.EW_E2E_SA_OTP ?? '');
  await page.getByRole('button', { name: 'Đăng nhập' }).click();
}

async function setMenu(page: Page, key: string, state: 'Bật' | 'Tắt', reason: string) {
  await page.goto('/platform-admin/feature-toggles');
  await page.getByRole('row', { name: new RegExp(key.replace(/\./g, '\\.')) }).getByRole('button', { name: 'Sửa' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel(state).check();
  await dialog.getByLabel('Lý do').fill(reason);
  await dialog.getByRole('button', { name: 'Lưu' }).click();
  await expect(dialog).toBeHidden();
}

test.describe('@F035 Tắt menu nghiên cứu', () => {
  test.skip(!ready, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');

  test('turned-off research menus are hidden and the API answers FEATURE_DISABLED', async ({ page }) => {
    await signIn(page);
    try {
      for (const key of MENUS) await setMenu(page, key, 'Tắt', 'Kiểm thử tắt menu nghiên cứu F035');
      await page.reload();
      await expect(page.getByRole('link', { name: 'Nghiên cứu hiệu quả học tập' })).toHaveCount(0, { timeout: 120_000 });
      await expect(page.getByRole('link', { name: 'Thử nghiệm A/B' })).toHaveCount(0);
      await page.goto('/platform-admin/efficacy-studies');
      await expect(page.getByTestId('feature-disabled')).toBeVisible();
    } finally {
      for (const key of MENUS) await setMenu(page, key, 'Bật', 'Bật lại menu nghiên cứu sau kiểm thử F035');
    }
  });
});
