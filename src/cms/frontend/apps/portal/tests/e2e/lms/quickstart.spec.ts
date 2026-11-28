// @F055 T046 — quickstart of F055 (specs/055-portal-lms-assignments/quickstart.md §2) on the REAL stack, the scenarios
// not covered by lms/assign.spec.ts (T015: 1–2, 5, 8 and student visibility), lms/authz.spec.ts (19),
// lms/tenant-isolation.spec.ts (20) and school-lms/school-grade.spec.ts (21 menus):
//   1 content tabs, only server-graded published types · 2 schools of the scope only · 3 graded → "follow_latest"
//   disabled · 4 title bounded by maxLength, due before open refused at the field (no next step) · 9 extend
//   one student, excuse another with a reason · 10 remind twice in a day → "Bài này đã được nhắc hôm nay." ·
//   11 cancel of an assignment without submission (with a reason) · 18 `ad.donvi` (tenant_admin, read only) has no
//   "Giao bài".
// Scenarios 13–16 and 22 (course, learning path, calendar, study groups) are gone with ADR-0095; 6 (RESOURCE_NOT_ASSIGNED),
// 7 (System Admin target tenant) and 12 (two tabs) are Vitest (wizard.test.tsx, detail.test.tsx); 17 (accommodations)
// needs a graded assessment blueprint, not in the local seed. The assignment created here is cancelled at the end (the
// golden dashboard is unchanged). No student signs in. Rows are found by position: the submissions table shows no
// student name on this stack (AssignmentSubmission.display_name is never filled by module learning — reported
// 2026-11-28, F055-AC-08). Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise
// (OQ F051-9). No secret is written anywhere (AGENTS R4).
import { expect, test, type Page } from '@playwright/test';
import { e2eReady } from '../support/auth';
import { openOptions, selectedItem, spaGoto } from '../support/nav';
import { openSession, type Session } from '../support/session';

const ACTIVITY = '[DEV] Quiz bảng nhân 6–9';

const pad = (n: number) => String(n).padStart(2, '0');
const pickerValue = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;

async function pick(page: Page, label: string, option: RegExp): Promise<void> {
  await page.getByRole('combobox', { name: label, exact: true }).click({ force: true });
  await openOptions(page).filter({ hasText: option }).first().click();
}

async function setDate(page: Page, label: string, at: Date): Promise<void> {
  const input = page.getByLabel(label, { exact: true }).first();
  await input.click();
  await input.fill(pickerValue(at));
  await input.press('Enter');
}

test.describe('@F055 quickstart — giao bài và theo dõi nộp bài trên stack thật', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
  test.describe.configure({ mode: 'serial' });

  let s: Session;
  let assignmentId: string | undefined;
  let cancelled = false;
  const title = `[E2E] Quickstart F055 ${Date.now().toString(36)}`;

  test.beforeAll(async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    s = await openSession(browser, 'ad.truong.a');
  });
  test.afterAll(async () => {
    // a failed run still cancels the assignment it created (no submission), so the dashboard of the grade is unchanged
    if (s && assignmentId && !cancelled) {
      await spaGoto(s.page, `/school-lms/assignments/${assignmentId}`).catch(() => undefined);
      await s.page.getByRole('button', { name: 'Hủy bài', exact: true }).click({ timeout: 30_000 }).catch(() => undefined);
      await s.page.getByRole('dialog').getByLabel('Lý do').fill('Dọn dữ liệu sau quickstart E2E F055').catch(() => undefined);
      await s.page.getByRole('dialog').getByRole('button', { name: 'Hủy bài', exact: true }).click({ timeout: 10_000 }).catch(() => undefined);
      await s.page.waitForTimeout(2000);
    }
    await s?.close();
  });

  test('1–4: content tabs, schools of the scope, graded disables follow_latest, field limits → assigned', async () => {
    test.setTimeout(6 * 60_000);
    const { page } = s;
    await spaGoto(page, '/school-lms/assignments/new');
    const picker = page.getByTestId('content-picker');
    await expect(picker).toBeVisible({ timeout: 60_000 });
    // 1: the five tabs; the activity tab lists published server-graded types only (no written_response…)
    expect((await picker.getByRole('tab').allTextContents()).map((x) => x.trim())).toEqual(['Activity', 'Bộ câu hỏi', 'Đề kiểm tra', 'Bài học', 'Hoạt động game']);
    await picker.getByRole('tab', { name: 'Activity' }).click();
    const item = picker.locator('.ant-list-item').filter({ hasText: ACTIVITY }).first();
    await expect(item).toBeVisible({ timeout: 60_000 });
    expect(await picker.locator('.ant-list-item').filter({ hasText: /Tự luận|written_response|speaking|project/ }).count()).toBe(0);
    await item.getByRole('button', { name: 'Chọn', exact: true }).click();
    await page.getByRole('button', { name: 'Tiếp' }).click();
    // 2: the school select offers the schools of the scope only (TH Demo A)
    await expect(selectedItem(page, 'Trường')).toHaveText(/Demo A$/, { timeout: 120_000 });
    await page.getByRole('combobox', { name: 'Trường', exact: true }).click({ force: true });
    const schools = (await openOptions(page).allTextContents()).map((x) => x.trim());
    expect(schools.length).toBeGreaterThan(0);
    expect(schools.filter((x) => !/Demo A$/.test(x))).toEqual([]);
    await page.keyboard.press('Escape');
    await pick(page, 'Môn học', /^Toán/);
    await pick(page, 'Khối được giao', /^Khối 3$/);
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: 'Tiếp' }).click();
    // 4: title bounded by maxLength; due before open → error at the field, no next step
    await expect(page.locator('.ant-steps-item-active')).toContainText('Thời gian');
    const titleBox = page.locator('#asg-title');
    const max = Number(await titleBox.getAttribute('maxlength'));
    await titleBox.fill('x'.repeat(max + 20));
    expect((await titleBox.inputValue()).length).toBe(max);
    await titleBox.fill(title);
    const now = new Date();
    await setDate(page, 'Mở bài lúc', new Date(now.getTime() + 2 * 86_400_000));
    await setDate(page, 'Hạn nộp', new Date(now.getTime() + 86_400_000));
    await page.getByRole('button', { name: 'Tiếp' }).click();
    await expect(page.getByText('Hạn nộp phải sau thời điểm mở bài.')).toBeVisible();
    await expect(page.locator('.ant-steps-item-active')).toContainText('Thời gian');
    await setDate(page, 'Mở bài lúc', now);
    await setDate(page, 'Hạn nộp', new Date(now.getTime() + 7 * 86_400_000));
    await page.getByRole('button', { name: 'Tiếp' }).click();
    // 3: graded → "follow_latest" disabled; back to practice
    await expect(page.locator('.ant-steps-item-active')).toContainText('Chính sách');
    await page.getByRole('radio', { name: 'Kiểm tra (tính điểm)' }).check({ force: true });
    await expect(page.getByRole('checkbox', { name: /Luôn dùng phiên bản mới nhất/ })).toBeDisabled();
    await page.getByRole('radio', { name: 'Luyện tập' }).check({ force: true });
    await expect(page.getByRole('checkbox', { name: /Luôn dùng phiên bản mới nhất/ })).toBeEnabled();
    await page.getByRole('button', { name: 'Tiếp' }).click();
    await expect(page.getByTestId('wizard-review')).toContainText('Khối 3');
    const detail = page.waitForURL(/\/school-lms\/assignments\/[0-9a-f-]{36}$/, { timeout: 120_000 });
    await page.getByRole('button', { name: 'Giao bài', exact: true }).click();
    await detail;
    assignmentId = new URL(page.url()).pathname.split('/').pop();
  });

  test('9–11: extend one student, excuse another, remind twice, cancel with a reason', async () => {
    test.setTimeout(6 * 60_000);
    test.skip(!assignmentId, 'no assignment created');
    const { page } = s;
    const detail = page.getByTestId('assignment-detail');
    await expect(detail).toBeVisible({ timeout: 60_000 });
    // data rows by position (stable: the server orders them by student id; an excused row loses its action buttons)
    const rows = detail.locator('.ant-table-tbody tr.ant-table-row');
    await expect(rows.first()).toBeVisible({ timeout: 60_000 });
    const first = rows.nth(0);
    const second = rows.nth(1);
    // 9: extend (new due date + reason) and excuse (reason)
    await first.getByRole('button', { name: 'Gia hạn' }).click();
    let dialog = page.getByRole('dialog').filter({ hasText: 'Gia hạn cho' });
    await expect(dialog).toBeVisible();
    await setDate(page, 'Hạn nộp mới', new Date(Date.now() + 10 * 86_400_000));
    await dialog.getByLabel('Lý do').fill('Học sinh nghỉ ốm hai ngày (kiểm thử E2E)');
    const extended = page.waitForResponse((r) => /\/students\/[^/]+:extend|:extend/.test(r.url()) && r.request().method() === 'POST', { timeout: 60_000 });
    await dialog.getByRole('button', { name: /OK|Lưu|Xác nhận|Gia hạn/ }).last().click();
    expect((await extended).status()).toBeLessThan(300);
    await expect(first.locator('td').nth(5)).not.toHaveText('', { timeout: 60_000 }); // "Hạn riêng"
    await second.getByRole('button', { name: 'Miễn' }).click();
    dialog = page.getByRole('dialog').filter({ hasText: 'Miễn bài cho' });
    await expect(dialog).toBeVisible();
    await dialog.getByLabel('Lý do').fill('Học sinh chuyển chương trình học (kiểm thử E2E)');
    const excused = page.waitForResponse((r) => /:excuse/.test(r.url()) && r.request().method() === 'POST', { timeout: 60_000 });
    await dialog.getByRole('button', { name: /OK|Xác nhận|Miễn/ }).last().click();
    expect((await excused).status()).toBeLessThan(300);
    await expect(second).toContainText('Được miễn', { timeout: 60_000 });
    // 10: remind twice in a day → the second time is refused with the notice and the button is locked
    const remind = page.getByRole('button', { name: 'Nhắc học sinh chưa làm' });
    await remind.click();
    await expect(page.getByText(/Đã gửi nhắc cho \d+ học sinh\.|Bài này đã được nhắc hôm nay\./).first()).toBeVisible({ timeout: 60_000 });
    if (await remind.isEnabled().catch(() => false)) await remind.click();
    await expect(page.getByTestId('remind-limited')).toHaveText('Bài này đã được nhắc hôm nay.', { timeout: 60_000 });
    await expect(remind).toBeDisabled();
    // 11: cancel an assignment without submission (reason required)
    await page.getByRole('button', { name: 'Hủy bài', exact: true }).click();
    dialog = page.getByRole('dialog');
    await dialog.getByLabel('Lý do').fill('Dọn dữ liệu sau quickstart E2E F055');
    const cancelRes = page.waitForResponse((r) => new RegExp(`/lms/assignments/${assignmentId}:cancel`).test(r.url()), { timeout: 60_000 });
    await dialog.getByRole('button', { name: 'Hủy bài', exact: true }).click();
    expect((await cancelRes).status()).toBeLessThan(300);
    cancelled = true;
  });

  test('18: ad.donvi (read only) has no "Giao bài"', async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    const d = await openSession(browser, 'ad.donvi');
    try {
      const list = d.page.waitForResponse((r) => /\/api\/v1\/lms\/assignments(\?|$)/.test(r.url()) && r.request().method() === 'GET', { timeout: 60_000 });
      await spaGoto(d.page, '/school-lms/assignments');
      expect((await list).status()).toBe(200);
      await expect(d.page.getByRole('heading', { level: 1 })).toBeVisible();
      await expect(d.page.getByRole('button', { name: 'Giao bài', exact: true })).toHaveCount(0);
      await expect(d.page.getByRole('link', { name: 'Giao bài', exact: true })).toHaveCount(0);
    } finally {
      await d.close();
    }
  });
});
