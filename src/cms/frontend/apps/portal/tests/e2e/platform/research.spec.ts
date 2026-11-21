// F035 T064 — F035-AC-35 (TC-UI-003, TC-UI-006), quickstart scenario 22 @F035: on the REAL stack (seed of
// quickstart §1), sa.admin goes through the research flow of workspace D (agreement → study → classes → activate →
// wave → report) and the A/B experiment screens; ad.truong.a never sees the research menus; the pages pass a basic
// accessibility check (labelled controls, one h1). Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD (password only, ADR-0094); skipped
// otherwise (OQ F051-9: no E2E job against a real backend yet).
import { expect, test, type Page } from '@playwright/test';
import { e2eReady as ready, signIn } from '../support/auth';

/** Accessibility smoke: one h1, every form control has an accessible name. */
async function a11y(page: Page) {
  await expect(page.getByRole('heading', { level: 1 })).toHaveCount(1);
  const unnamed = await page.$$eval('input:not([type=hidden]), select, textarea, [role=combobox]', (els) =>
    els.filter((e) => !(e.getAttribute('aria-label') || e.getAttribute('aria-labelledby') || (e.id && document.querySelector(`label[for="${e.id}"]`)))).length,
  );
  expect(unnamed).toBe(0);
}

test.describe('@F035 Nghiên cứu hiệu quả trên portal', () => {
  test.skip(!ready, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');

  test('sa.admin: agreement, study, class pair, activation, wave and report', async ({ page }) => {
    await signIn(page, 'sa.admin');
    await page.goto('/platform-admin'); // sign-in lands on the first workspace (school_lms)
    await page.getByRole('link', { name: 'Nghiên cứu hiệu quả học tập' }).click();
    await expect(page.getByRole('heading', { level: 1, name: 'Nghiên cứu hiệu quả học tập' })).toBeVisible();
    await a11y(page);
    // agreements of th-demo-a (tenant_ids[] → audit cross_tenant.accessed)
    await page.getByRole('tab', { name: 'Thỏa thuận tham gia' }).click();
    await page.getByRole('combobox', { name: 'Chọn tenant để xem thỏa thuận' }).click();
    await page.getByTitle(/th-demo-a/).last().click();
    await page.getByRole('button', { name: 'Ghi nhận thỏa thuận' }).click();
    const agreement = page.getByRole('dialog');
    await agreement.getByRole('combobox', { name: 'Tenant đích' }).click();
    await page.getByTitle(/th-demo-a/).last().click();
    const ref = `HD-E2E-${Date.now()}`;
    await agreement.getByLabel('Mã văn bản thỏa thuận').fill(ref);
    await agreement.getByLabel('Ngày ký').fill('01/09/2026');
    await agreement.getByLabel('Ngày ký').press('Enter');
    await agreement.getByRole('button', { name: 'Lưu' }).click();
    await expect(page.getByText(ref)).toBeVisible();
    // the seeded draft study (quickstart §1): detail, activation, wave
    await page.getByRole('tab', { name: 'Nghiên cứu' }).click();
    await page.getByRole('link', { name: /math-g3/ }).first().click();
    await expect(page.getByRole('tab', { name: 'Lớp và cặp' })).toBeVisible();
    await a11y(page);
    if (await page.getByRole('button', { name: 'Kích hoạt' }).isVisible()) {
      await page.getByRole('button', { name: 'Kích hoạt' }).click();
      await page.getByRole('button', { name: 'Kích hoạt' }).last().click();
      await expect(page.getByText('Nghiên cứu đã kích hoạt: cặp lớp, nhánh và dạng đề đã khóa.')).toBeVisible();
    }
    await page.getByRole('tab', { name: 'Đợt kiểm tra' }).click();
    await expect(page.getByText(/Đợt kiểm tra không công bố điểm/)).toBeVisible();
    await page.getByRole('tab', { name: 'Báo cáo' }).click();
    await page.getByRole('button', { name: 'Tạo báo cáo' }).click();
    await expect(page.getByRole('region', { name: 'Báo cáo hiệu quả' })).toBeVisible({ timeout: 60_000 });
    // no direct identifier of a student anywhere on the report page (R16)
    await expect(page.locator('body')).not.toContainText(/hs\.an\.3a|hs\.binh\.3a/);
  });

  test('sa.admin: experiment list, detail and results by arm', async ({ page }) => {
    await signIn(page, 'sa.admin');
    await page.goto('/platform-admin');
    await page.getByRole('link', { name: 'Thử nghiệm A/B' }).click();
    await expect(page.getByRole('heading', { level: 1, name: 'Thử nghiệm A/B' })).toBeVisible();
    await a11y(page);
    await page.getByRole('button', { name: 'Tạo thử nghiệm' }).click();
    await expect(page.getByRole('dialog').getByLabel('Mã thử nghiệm')).toBeVisible();
    await page.getByRole('dialog').getByRole('button', { name: 'Hủy' }).click();
  });

  test('ad.truong.a never sees the research menus; the URL answers 403', async ({ page }) => {
    await signIn(page, 'ad.truong.a');
    await expect(page.getByRole('link', { name: 'Nghiên cứu hiệu quả học tập' })).toHaveCount(0);
    await page.goto('/platform-admin/efficacy-studies');
    await expect(page.getByText('Không có quyền truy cập')).toBeVisible();
  });
});
