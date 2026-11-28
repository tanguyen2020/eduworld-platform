// @F058 T047 — US4 (F058-AC-22…AC-25, TC-LRN-015) on the REAL stack with the golden dataset of `make seed-attempts`
// (LOCAL_DEVELOPMENT §5.2): the platform question "[DEV] Chu vi hình vuông cạnh 4 cm là?" (assigned to th-demo-a) has a
// version 1 with a faulty key and a corrected version 2 (published); 3 graded attempts pinned v1.
// - `qlcm.truong.a` (TH Demo A, learning_content.re_evaluate) opens the question → "Phiên bản · Phát hành …" →
//   version 1 "Xem" → "Chấm lại hệ thống…": `use_new_key` needs the corrected version (v2), a reason under 10
//   characters is refused, the server announces 3 affected attempts before confirming → one createRegradeRequest
//   (Idempotency-Key) → followed to "Hoàn tất" with 3/3 re-evaluated, and the request is listed on the version;
// - `ad.truong.a` (school_admin, no learning_content.re_evaluate) sees neither the tab nor the button.
// ORDER: run AFTER dashboard/overview.spec.ts (F056 T021): this regrade changes the MATH.G3.GEO.PERIMETER row of the
// golden heatmap. A second run finds the request of the first one in the history and stops at the impact (no second
// re-evaluation of the same version is asked). Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise
// (OQ F051-9). No secret is written anywhere (AGENTS R4).
import { expect, test, type Page } from '@playwright/test';
import { e2eReady } from '../support/auth';
import { openOptions, spaGoto } from '../support/nav';
import { enterTenant, openSession } from '../support/session';

const QUESTION = 'Chu vi hình vuông cạnh 4 cm';
const RELEASE_SECTION = 'Phiên bản · Phát hành · Sử dụng · Xem trước';

/** Opens the seeded regrade question from the question bank list (search by stem) and its version 1 drawer. */
async function openVersionOne(page: Page): Promise<void> {
  await spaGoto(page, '/content-studio/question-bank');
  const search = page.getByLabel('Tìm theo từ khóa (không cần dấu)').first();
  await expect(search).toBeVisible({ timeout: 60_000 });
  await search.fill(QUESTION);
  await search.press('Enter');
  // the search runs on the stem; the list shows the question by type + short id (one match: the seeded question)
  const rows = page.locator('.ant-table-tbody tr.ant-table-row');
  await expect(rows, 'missing seed data: regrade question of make seed-attempts').toHaveCount(1, { timeout: 60_000 });
  await rows.first().getByRole('button', { name: 'Mở' }).click();
  await page.getByRole('button', { name: RELEASE_SECTION }).click();
  const v1 = page.locator('tr').filter({ hasText: /^v1\b|v1 / }).filter({ has: page.getByRole('button', { name: 'Xem' }) }).first();
  await expect(v1).toBeVisible({ timeout: 60_000 });
  await v1.getByRole('button', { name: 'Xem' }).click();
  await expect(page.getByTestId('version-view')).toBeVisible({ timeout: 60_000 });
}

test.describe('@F058 Chấm lại hệ thống — version câu hỏi lỗi → đáp án mới (F058-AC-22…AC-25)', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
  test.describe.configure({ mode: 'serial' });

  test('qlcm.truong.a: v1 → use_new_key v2 → 3 affected attempts → completed, 3 re-evaluated', async ({ browser }) => {
    test.setTimeout(8 * 60_000);
    const s = await openSession(browser, 'qlcm.truong.a');
    try {
      const { page } = s;
      await enterTenant(page, /TH Demo A/);
      await openVersionOne(page);
      const drawer = page.locator('.ant-drawer').filter({ has: page.getByTestId('version-view') });
      const entry = drawer.getByTestId('regrade-entry');
      await expect(entry).toBeVisible();
      const history = entry.getByTestId('regrade-history');
      const done = history.getByText('Hoàn tất');
      if (await done.count()) {
        // a previous run already re-evaluated this version: the history shows it with its counts
        await expect(history).toContainText('3/3 bài');
        test.info().annotations.push({ type: 'rerun', description: 'regrade already completed by an earlier run' });
        return;
      }
      await entry.getByRole('button', { name: 'Chấm lại hệ thống…' }).click();
      const dialog = page.getByRole('dialog', { name: 'Chấm lại hệ thống' });
      const box = dialog.getByTestId('regrade-dialog');
      await expect(box.getByRole('radio', { name: 'Dùng đáp án mới' })).toBeChecked();
      const confirm = dialog.getByRole('button', { name: 'Xác nhận chấm lại' });
      // a reason shorter than 10 characters is refused; no corrected version yet → nothing to confirm
      await box.getByLabel('Lý do (10–1.000 ký tự)').fill('ngắn');
      await expect(box.getByText('Lý do cần ít nhất 10 ký tự.')).toBeVisible();
      await expect(confirm).toBeDisabled();
      await box.getByLabel('Lý do (10–1.000 ký tự)').fill('Đáp án version 1 sai (16 cm); chấm lại theo version 2 đã sửa — kiểm thử E2E F058 T047');
      await expect(confirm).toBeDisabled(); // use_new_key needs the corrected version
      await box.getByRole('combobox', { name: 'Phiên bản đã sửa (đã xuất bản)' }).locator('xpath=ancestor::div[contains(@class,"ant-select")][1]').click();
      await openOptions(page).filter({ hasText: /^v2$/ }).first().click();
      // the server announces the impact before confirming
      await expect(box.getByTestId('regrade-impact')).toHaveText(/3 bài làm sẽ được chấm lại\./, { timeout: 60_000 });
      const creates: string[] = [];
      page.on('request', (r) => {
        if (r.method() === 'POST' && /\/regrade-requests$/.test(new URL(r.url()).pathname)) creates.push(r.headers()['idempotency-key'] ?? '');
      });
      await expect(confirm).toBeEnabled();
      await confirm.click();
      const status = box.getByTestId('regrade-status');
      await expect(status).toBeVisible({ timeout: 60_000 });
      await expect(status).toContainText('Hoàn tất', { timeout: 5 * 60_000 });
      await expect(status.locator('tr, .ant-descriptions-row').filter({ hasText: 'Bài bị ảnh hưởng' })).toContainText('3');
      await expect(status.locator('tr, .ant-descriptions-row').filter({ hasText: 'Đã chấm lại' })).toContainText('3');
      expect(creates.length, 'one createRegradeRequest').toBe(1);
      expect(creates[0]).toMatch(/.{8,}/);
      await dialog.getByRole('button', { name: 'Đóng' }).click();
      await expect(history).toContainText('Hoàn tất', { timeout: 60_000 });
      await expect(history).toContainText('3/3 bài');
    } finally {
      await s.close();
    }
  });

  test('ad.truong.a: no system re-evaluation anywhere (no learning_content.re_evaluate)', async ({ browser }) => {
    test.setTimeout(4 * 60_000);
    const s = await openSession(browser, 'ad.truong.a');
    try {
      const { page } = s;
      await openVersionOne(page);
      await expect(page.getByTestId('version-view')).toBeVisible();
      await expect(page.getByTestId('regrade-entry')).toHaveCount(0);
      await expect(page.getByRole('button', { name: 'Chấm lại hệ thống…' })).toHaveCount(0);
      await expect(page.getByRole('tab', { name: 'Chấm lại hệ thống' })).toHaveCount(0);
    } finally {
      await s.close();
    }
  });
});
