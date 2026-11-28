// @F056 T049 — quickstart of F056 (specs/056-portal-school-dashboard/quickstart.md §2) on the REAL stack, the scenarios
// that run against the local compose stack and its seed (golden dataset of `make seed-attempts`, LOCAL_DEVELOPMENT §5.2):
//   1–2 overview: 5 widgets in order, "Cập nhật lúc" / grain / "Tạm tính" · 3 range presets, no "toàn bộ thời gian",
//   future days disabled · 4 429 DASHBOARD_BUSY (≤ 2 retries) and 504 DASHBOARD_QUERY_TIMEOUT (hint to narrow) —
//   simulated with a route on getSchoolDashboard · 5 "Làm quá nhanh" without ranking · 6 Tenant Admin summary "< 5" ·
//   7 safety widget = count + link · 8 grade link keeps the range, breadcrumb school → grade (ADR-0095: grade, not class)
//   · 9 heatmap cells reachable by keyboard with an accessible name, legend · 10 "Giao ôn tập theo LO" → wizard prefilled
//   · 11 student profile tabs, no IP / friends / chat · 12 results without XP · 13 analysis of a practice assignment
//   (no exam session → notice; the p-value table needs a graded blueprint, not seeded — Vitest AC-13) · 14 export XLSX →
//   preparing → link · 16 reference levels labelled "Tham khảo" · 17 platform summary with 2 tenants → Tenant column ·
//   21 search input bounded by maxLength · 22 action.school_lms.reports.export off for th-demo-a → 403 FEATURE_DISABLED
//   + permissions reload → export hidden, dashboard still readable (override restored).
// Scenarios 19–20 are dashboard/authz.spec.ts and dashboard/tenant-isolation.spec.ts; 18 (subject scope) needs a
// subject-scoped role that the local seed does not have (LOCAL_DEVELOPMENT §5: qlcm.truong.a is tenant-wide) — Vitest
// AC-19; 15 (export limit / expired link) is Vitest AC-15. Read only except the export jobs of 14/22 and the flag of 22.
// Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise (OQ F051-9). No secret or student name is written (AGENTS R4, R14).
import { expect, test, type Page, type Response, type Route } from '@playwright/test';
import { e2eReady } from '../support/auth';
import { holdPermissions, readFlag, setFlag, type FlagState, type FlagTarget } from '../support/featureFlags';
import { openOptions, spaGoto } from '../support/nav';
import { openSession, type Session } from '../support/session';

const BASE = '/school-lms/school-dashboard';
const SCHOOL_DASHBOARD = /\/api\/v1\/lms\/dashboards\/school(\?|$)/;
const GOLDEN = '[DEV] Phép chia có dư (golden)';
const isSchoolDashboard = (r: Response) => SCHOOL_DASHBOARD.test(r.url()) && r.request().method() === 'GET';

/** Picks a preset of the range (antd Segmented: the label is the clickable part). */
async function preset(page: Page, name: string): Promise<void> {
  await page.getByRole('radiogroup', { name: 'Khoảng thời gian' }).getByText(name, { exact: true }).click();
}

async function settle(page: Page): Promise<void> {
  await page
    .waitForFunction(() => !document.querySelector('.ant-skeleton-active, .ant-spin-spinning'), undefined, { timeout: 30_000 })
    .catch(() => undefined);
}

test.describe('@F056 quickstart — School Learning Dashboard trên stack thật', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
  test.describe.configure({ mode: 'serial' });

  let s: Session;
  let schoolId: string | undefined;
  /** Last getGradeDashboard body of grade 3 (the page may answer from its cache later). */
  let gradeBody = '';

  test.beforeAll(async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    s = await openSession(browser, 'ad.truong.a');
    s.page.on('response', (r) => {
      if (/\/grades\/3\/dashboard/.test(r.url()) && r.status() === 200) void r.text().then((t) => (gradeBody = t)).catch(() => undefined);
    });
  });
  test.afterAll(async () => s?.close());

  test('1–3, 7: overview widgets in order, meta, range presets, safety widget', async () => {
    test.setTimeout(3 * 60_000);
    const { page } = s;
    const first = page.waitForResponse(isSchoolDashboard, { timeout: 60_000 });
    await spaGoto(page, BASE);
    schoolId = new URL((await first).url()).searchParams.get('school_id') ?? undefined;
    await settle(page);
    const order = await page.locator('[data-testid^="widget-"]').evaluateAll((els) => els.map((e) => e.getAttribute('data-testid')));
    expect(order.filter((x) => ['widget-assignments', 'widget-weak', 'widget-attention', 'widget-selfStudy', 'widget-safety'].includes(x ?? ''))).toEqual([
      'widget-assignments',
      'widget-weak',
      'widget-attention',
      'widget-selfStudy',
      'widget-safety',
    ]);
    const meta = page.getByTestId('widget-assignments').getByTestId('dashboard-meta');
    await expect(meta).toContainText(/Cập nhật lúc \d{2}:\d{2}/);
    await expect(meta).toContainText('Theo ngày');
    await expect(meta).toContainText('Tạm tính'); // today is in the range
    const presets = await page.getByRole('radiogroup', { name: 'Khoảng thời gian' }).getByRole('radio').evaluateAll((els) => els.map((e) => e.getAttribute('aria-label') ?? e.parentElement?.textContent ?? ''));
    expect(presets.join('|')).not.toMatch(/Toàn bộ/i);
    await expect(page.getByRole('radio', { name: '7 ngày' })).toBeChecked();
    // custom range: days after today cannot be picked
    await page.getByRole('textbox', { name: 'Khoảng tùy chọn' }).first().click();
    await expect(page.locator('.ant-picker-dropdown:not(.ant-picker-dropdown-hidden) .ant-picker-cell-disabled').first()).toBeVisible();
    await page.keyboard.press('Escape');
    // ⑤ safety: a count and a link to the notifications, no chat content
    const safety = page.getByTestId('widget-safety');
    await expect(safety.getByTestId('metric-safety')).toBeVisible();
    await expect(safety.getByRole('link', { name: 'Xem thông báo an toàn' })).toHaveAttribute('href', '/school-lms/safety-notifications');
  });

  test('4: 429 DASHBOARD_BUSY is retried at most twice; 504 asks to narrow the range', async () => {
    test.setTimeout(3 * 60_000);
    const { page } = s;
    let calls = 0;
    let answer: { status: number; code: string } = { status: 429, code: 'DASHBOARD_BUSY' };
    const fake = (route: Route) => {
      calls++;
      return route.fulfill({ status: answer.status, contentType: 'application/problem+json', body: JSON.stringify({ type: 'about:blank', title: answer.code, status: answer.status, code: answer.code, trace_id: '00000000000000000000000000000000' }) });
    };
    await page.route(SCHOOL_DASHBOARD, fake);
    try {
      await preset(page, '30 ngày');
      await expect(page.getByTestId('widget-assignments')).toContainText('Hệ thống đang bận', { timeout: 60_000 });
      await page.waitForTimeout(10_000);
      expect(calls, 'first call + at most 2 retries').toBeLessThanOrEqual(3);
      answer = { status: 504, code: 'DASHBOARD_QUERY_TIMEOUT' };
      await preset(page, 'Hôm nay');
      await expect(page.getByTestId('widget-assignments')).toContainText('Hãy thu hẹp phạm vi', { timeout: 60_000 });
    } finally {
      await page.unroute(SCHOOL_DASHBOARD, fake);
    }
    // back to 7 days (cached or read again): the widgets show their data again
    await preset(page, '7 ngày');
    await expect(page.getByTestId('widget-assignments').getByRole('progressbar').first()).toBeVisible({ timeout: 60_000 });
  });

  test('8, 10: grade link keeps the range (breadcrumb school → grade); "Giao ôn tập theo LO" prefills the wizard', async () => {
    test.setTimeout(3 * 60_000);
    const { page } = s;
    await settle(page);
    const gradeLink = page.getByTestId('widget-assignments').getByRole('link', { name: 'Khối 3' }).first();
    const href = (await gradeLink.getAttribute('href')) ?? '';
    expect(href).toMatch(/\/school-dashboard\/grades\/3\?/);
    expect(new URL(href, process.env.EW_E2E_PORTAL_URL).searchParams.get('preset')).toBe('7d');
    await gradeLink.click();
    await expect(page.getByTestId('dashboard-grade')).toBeVisible({ timeout: 60_000 });
    const crumbs = page.getByTestId('dashboard-grade').locator('.ant-breadcrumb');
    await expect(crumbs.getByRole('link')).toHaveText('Tổng quan trường');
    await expect(crumbs).toContainText('Khối 3');
    await expect(page.getByRole('radio', { name: '7 ngày' })).toBeChecked();
    // back to the overview: LO to consolidate → practice wizard with the LO, school and grade prefilled
    await crumbs.getByRole('link', { name: 'Tổng quan trường' }).click();
    const practice = page.getByTestId('widget-weak').getByRole('link', { name: 'Giao ôn tập theo LO' }).first();
    await expect(practice).toBeVisible({ timeout: 60_000 });
    const target = new URL((await practice.getAttribute('href')) ?? '', process.env.EW_E2E_PORTAL_URL);
    expect(target.pathname).toBe('/school-lms/assignments/new');
    expect(Object.fromEntries(target.searchParams)).toMatchObject({ mode: 'practice', school_id: schoolId, grade: '3' });
    expect(target.searchParams.get('objective_id')).toMatch(/^[0-9a-f-]{36}$/);
    await practice.click();
    const contentPicker = page.getByTestId('content-picker');
    await expect(contentPicker).toBeVisible({ timeout: 60_000 });
    // the question sets / lessons / game tabs list the content of that LO (listCmsActivities has no LO filter)
    const picker = page.waitForRequest((r) => new URL(r.url()).searchParams.get('objective_id') === target.searchParams.get('objective_id'), { timeout: 60_000 });
    await contentPicker.getByRole('tab', { name: 'Bộ câu hỏi' }).click();
    await picker;
    await page.getByRole('button', { name: 'Hủy' }).first().click().catch(() => undefined);
  });

  test('5, 9, 21: grade page — rapid guess flag without ranking, heatmap by keyboard, bounded search', async () => {
    test.setTimeout(3 * 60_000);
    test.skip(!schoolId, 'no school on the overview');
    const { page } = s;
    await spaGoto(page, `${BASE}/grades/3?school=${schoolId}`);
    const students = page.getByTestId('widget-students');
    await expect(students).toBeVisible({ timeout: 60_000 });
    await settle(page);
    await expect.poll(() => gradeBody.length, { timeout: 60_000 }).toBeGreaterThan(0);
    const flagged = (gradeBody.match(/"rapid_guess_flag":true/g) ?? []).length;
    // every student the server flags (rapid_guess ≥ 30 % in 7 days, dashboard_attention_thresholds) carries the neutral
    // tag; the golden dataset flags nobody (1 rapid answer of 5 = 20 %), so the count is the server's, possibly 0
    await expect(students.getByText('Làm quá nhanh')).toHaveCount(flagged);
    const headers = (await students.locator('th').allTextContents()).map((x) => x.trim());
    expect(headers.join('|')).not.toMatch(/Điểm|Xếp hạng|XP/);
    // heatmap: every cell is focusable and named "LO · mức: n học sinh"; the legend lists the levels
    const heatmap = page.getByTestId('mastery-heatmap');
    const cell = heatmap.locator('td[data-level]').first();
    await cell.focus();
    await page.keyboard.press('Tab');
    const focused = await page.evaluate(() => document.activeElement?.getAttribute('aria-label') ?? '');
    expect(focused).toMatch(/^MATH\.G3\.[A-Z_.]+ · .+: \d+ học sinh$/);
    await expect(page.getByTestId('mastery-legend').getByRole('listitem')).toHaveCount(5);
    // search of a student: bounded by maxLength
    const search = page.getByRole('searchbox', { name: 'Tìm học sinh theo tên' }).or(page.getByLabel('Tìm học sinh theo tên')).first();
    const max = Number(await search.getAttribute('maxlength'));
    expect(max).toBeGreaterThan(0);
    await search.fill('x'.repeat(max + 50));
    expect((await search.inputValue()).length).toBeLessThanOrEqual(max);
    await search.fill('');
  });

  test('11: student profile — mastery, trend, history, recommendations, logins; no IP, friends or chat', async () => {
    test.setTimeout(3 * 60_000);
    test.skip(!schoolId, 'no school on the overview');
    const { page } = s;
    const link = page.getByTestId('widget-students').locator(`a[href*="${BASE}/students/"]`).first();
    await expect(link).toBeVisible({ timeout: 60_000 });
    await link.click();
    const profile = page.getByTestId('dashboard-student');
    await expect(profile).toBeVisible({ timeout: 60_000 });
    for (const tab of ['Mức thành thạo', 'Xu hướng 8 tuần', 'Lịch sử bài', 'Gợi ý đang hiển thị', 'Hoạt động đăng nhập']) await expect(profile.getByRole('tab', { name: tab })).toBeVisible();
    await profile.getByRole('tab', { name: 'Hoạt động đăng nhập' }).click();
    await settle(page);
    const text = (await profile.textContent()) ?? '';
    expect(text).not.toMatch(/\bIP\b|Bạn bè|Tin nhắn|Chat/);
  });

  test('12–14: results without XP; analysis of a practice assignment; export XLSX → link', async () => {
    test.setTimeout(5 * 60_000);
    const { page } = s;
    await spaGoto(page, '/school-lms/results');
    const table = page.getByTestId('assignment-results');
    await expect(table.getByRole('link', { name: `Mở phân tích bài ${GOLDEN}` })).toBeVisible({ timeout: 60_000 });
    const headers = [...new Set((await table.locator('th').allTextContents()).map((x) => x.trim()).filter(Boolean))];
    expect(headers).toEqual(['Bài giao', 'Đối tượng', 'Chế độ', 'Trạng thái', 'Hạn nộp', 'Đã nộp', 'Điểm trung bình']);
    await expect(table.locator('tr').filter({ hasText: GOLDEN })).toContainText('3/8');
    await table.getByRole('link', { name: `Mở phân tích bài ${GOLDEN}` }).click();
    const analysis = page.getByTestId('item-analysis');
    await expect(analysis).toBeVisible({ timeout: 60_000 });
    await expect(analysis.getByTestId('no-item-analysis')).toBeVisible();
    // export of the assignment results (createLmsReport, xlsx): preparing → download link with expiry
    const created = page.waitForResponse((r) => /\/api\/v1\/lms\/reports$/.test(new URL(r.url()).pathname) && r.request().method() === 'POST', { timeout: 60_000 });
    await analysis.getByTestId('export-button').getByRole('button').first().click();
    const res = await created;
    expect(res.status()).toBeLessThan(300);
    expect(res.request().headers()['idempotency-key']).toBeTruthy();
    await expect(analysis.getByTestId('export-link')).toBeVisible({ timeout: 3 * 60_000 });
    await expect(analysis.getByTestId('export-button')).toContainText('Link hết hạn lúc');
  });

  test('16: reference levels are labelled "Tham khảo — không thay đánh giá chính thức"', async () => {
    test.setTimeout(3 * 60_000);
    test.skip(!schoolId, 'no school on the overview');
    const { page } = s;
    await spaGoto(page, `${BASE}/reference?school=${schoolId}`);
    await expect(page.getByTestId('regulation-disclaimer').first()).toContainText('Tham khảo — không thay đánh giá chính thức', { timeout: 60_000 });
  });

  test('6: ad.donvi — tenant summary shows "< 5" exactly for the cells the server suppresses', async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    const d = await openSession(browser, 'ad.donvi');
    try {
      const summary = d.page.waitForResponse((r) => /\/tenant\/analytics\/learning\/summary/.test(r.url()) && r.request().method() === 'GET', { timeout: 60_000 });
      await spaGoto(d.page, '/school-lms/reports');
      await d.page.getByRole('tab', { name: 'Tổng hợp đơn vị' }).click();
      const res = await summary;
      expect(res.status()).toBe(200);
      const body = (await res.json()) as { cells?: { suppressed?: boolean }[] };
      const suppressed = (body.cells ?? []).filter((c) => c.suppressed).length;
      // the local seed has no learning activity in pgd-demo (no cell, none suppressed): the "< 5" rendering itself is
      // Vitest AC-06/AC-17; here the portal must show "< 5" for exactly the cells the server suppressed
      test.info().annotations.push({ type: 'cells', description: `${body.cells?.length ?? 0} cells, ${suppressed} suppressed` });
      const shown = d.page.getByTestId('tenant-summary').getByTestId('suppressed-cell');
      await expect(shown).toHaveCount(suppressed);
      if (suppressed) await expect(shown.first()).toHaveText('< 5');
    } finally {
      await d.close();
    }
  });

  test('17: sa.admin — platform summary of 2 tenants: tenant_ids, Tenant column', async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    const a = await openSession(browser, 'sa.admin');
    try {
      const { page } = a;
      await spaGoto(page, '/platform-admin/reports');
      const filter = page.getByRole('main').getByRole('combobox', { name: 'Lọc tenant' });
      await expect(filter).toBeVisible({ timeout: 60_000 });
      const two = page.waitForRequest((r) => /\/admin\/analytics\/learning\/summary/.test(r.url()) && new URL(r.url()).searchParams.getAll('tenant_ids').length + new URL(r.url()).searchParams.getAll('tenant_ids[]').length === 2, { timeout: 60_000 });
      for (const t of [/TH Demo A/, /PGD Demo|pgd-demo/i]) {
        await filter.click({ force: true });
        await openOptions(page).filter({ hasText: t }).first().click();
      }
      await page.keyboard.press('Escape');
      const req = await two;
      expect(req.headers()['x-tenant-id'], 'cross-tenant read: no X-Tenant-Id').toBeUndefined();
      await expect(page.getByTestId('widget-platform-summary').getByRole('columnheader', { name: 'Tenant' }).first()).toBeVisible({ timeout: 60_000 });
    } finally {
      await a.close();
    }
  });

  test('22: action.school_lms.reports.export off for th-demo-a — 403 FEATURE_DISABLED, export hidden, dashboard readable', async ({ browser }) => {
    test.setTimeout(10 * 60_000);
    const flag: FlagTarget = { key: 'action.school_lms.reports.export', kind: 'Thao tác', tenant: /TH Demo A/ };
    let hold: Awaited<ReturnType<typeof holdPermissions>> | undefined;
    const u = await openSession(browser, 'ad.truong.a', async (ctx) => {
      hold = await holdPermissions(ctx, /\/api\/v1\/lms\/reports$/);
    });
    const a = await openSession(browser, 'sa.admin');
    let original: FlagState | undefined;
    try {
      await spaGoto(u.page, '/school-lms/results');
      await u.page.getByTestId('assignment-results').getByRole('link', { name: `Mở phân tích bài ${GOLDEN}` }).click();
      const exportButton = u.page.getByTestId('item-analysis').getByTestId('export-button').getByRole('button').first();
      await expect(exportButton).toBeVisible({ timeout: 60_000 });
      original = await readFlag(a.page, flag);
      await setFlag(a.page, flag, 'off', 'Kiểm thử E2E F056 quickstart 22 tắt xuất báo cáo');
      // the backend applies the change within feature_toggle_cache_ttl (≤ principal_cache_ttl, 60 s): wait for it, then
      // the stale portal (permissions held) still offers the button — export until the backend refuses it. A click only
      // when the button is enabled (a queued export disables it), never waiting on it.
      await u.page.waitForTimeout(70_000);
      await expect
        .poll(
          async () => {
            if (!hold!.disabledSeen() && (await exportButton.isEnabled().catch(() => false))) await exportButton.click({ timeout: 5_000 }).catch(() => undefined);
            await u.page.waitForTimeout(3_000);
            return hold!.disabledSeen();
          },
          { timeout: 4 * 60_000, intervals: [15_000], message: 'createLmsReport answers 403 FEATURE_DISABLED' },
        )
        .toBe(true);
      expect(hold!.featureKey()).toBe(flag.key);
      await expect(u.page.getByText('Tính năng này đang tạm tắt.').first()).toBeVisible({ timeout: 30_000 });
      await expect.poll(() => hold!.reloadsAfterRelease(), { timeout: 60_000 }).toBeGreaterThan(0);
      await expect(u.page.getByTestId('item-analysis').getByTestId('export-button')).toHaveCount(0, { timeout: 60_000 });
      // the dashboard itself stays readable
      const dash = u.page.waitForResponse(isSchoolDashboard, { timeout: 60_000 });
      await spaGoto(u.page, BASE);
      expect((await dash).status()).toBe(200);
    } finally {
      if (original) await setFlag(a.page, flag, original, 'Khôi phục sau kiểm thử E2E F056 quickstart 22');
      await u.close();
      await a.close();
    }
  });
});

