// @ADR0095 — portal on the REAL stack after "no class concept" (ADR-0095) and password-only sign-in (ADR-0094): the
// student account is refused with the student message (403 PORTAL_ACCESS_STAFF_ONLY); "Học sinh theo khối" lists the
// seeded grade 3 of TH Demo A for ad.truong.a and qlcm.truong.a (school picker filled); ad.truong.b never sees TH Demo A
// nor a student of it (404); Content Studio (Curriculum included) loads for ad.truong.a and sa.admin; /platform-admin/*
// no longer shows the VPN notice; the assignment wizard targets a grade of a school (opened and filled up to the
// "Thời gian" step — nothing is created or published; skipped as missing seed data when no content is published);
// the grade page of the School Learning Dashboard opens.
// Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD (password only for every account, ADR-0094); skipped otherwise (OQ F051-9).
// No secret is written anywhere (AGENTS R4).
import { expect, test, type Page, type Response } from '@playwright/test';
import { e2eReady, signIn } from '../support/auth';
import { openOptions, openSelect, spaGoto } from '../support/nav';

const NAV = 'Menu điều hướng';
const VPN = 'Cần kết nối mạng quản trị (VPN)';
/** TH Demo A of the seed, shown as "Trường Tiểu học Demo A". */
const SCHOOL_A = /Demo A$/;
const PLACEHOLDER = 'Màn hình này được bổ sung ở tính năng sau.';

/** Student of TH Demo A seen by ad.truong.a, reused for the isolation check of ad.truong.b. */
let studentOfSchoolA: string | undefined;

/** Failed API calls (>= 400, token refresh aside) and page errors seen by the page. */
function watchFailures(page: Page): () => string[] {
  const failures: string[] = [];
  page.on('response', (r) => {
    const u = new URL(r.url());
    if (r.status() >= 400 && u.pathname.includes('/api/') && !u.pathname.endsWith('/auth/portal/token:refresh')) {
      failures.push(`API ${r.status()} ${r.request().method()} ${u.pathname}`);
    }
  });
  page.on('pageerror', (e) => failures.push(`page error: ${e.message.split('\n')[0]}`));
  return () => failures.splice(0);
}

async function settle(page: Page): Promise<void> {
  await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => undefined);
  await page
    .waitForFunction(() => !document.querySelector('.ant-skeleton-active, .ant-spin-spinning'), undefined, { timeout: 15_000 })
    .catch(() => undefined);
}

/** Error alerts / results the screen shows. */
function screenErrors(page: Page): Promise<string[]> {
  return page
    .locator('.ant-alert-error, .ant-result-error, .ant-result-403, .ant-result-404, .ant-result-500')
    .evaluateAll((els) => els.map((e) => (e.textContent ?? '').replace(/\s+/g, ' ').trim()));
}

/** Opens a select by its accessible name and clicks the option with that title (or the first one). */
async function pick(page: Page, label: string, option?: string | RegExp): Promise<string> {
  await openSelect(page, label);
  const opt = (option ? openOptions(page).filter({ hasText: option }) : openOptions(page)).first();
  await opt.waitFor({ timeout: 60_000 });
  const title = (await opt.textContent()) ?? '';
  await opt.click();
  return title.trim();
}

/** Text shown by an antd select (selected item). */
function selected(page: Page, label: string) {
  return page.getByRole('combobox', { name: label, exact: true }).locator('xpath=ancestor::div[contains(@class,"ant-select")][1]').locator('.ant-select-selection-item');
}

/** Sidebar links of one workspace path prefix. */
async function sidebarLinks(page: Page, prefix: string): Promise<string[]> {
  await spaGoto(page, prefix);
  await expect(page.getByRole('navigation', { name: NAV })).toBeVisible({ timeout: 30_000 });
  await settle(page);
  const hrefs = await page
    .getByRole('navigation', { name: NAV })
    .first()
    .locator('a[href]')
    .evaluateAll((as) => as.map((a) => a.getAttribute('href') ?? ''));
  return [...new Set(hrefs.filter((h) => h.startsWith(`${prefix}/`)))];
}

/** Opens every sidebar menu of a workspace and returns the screens with an error, the VPN notice or a placeholder. */
async function visitWorkspace(page: Page, prefix: string): Promise<{ errors: string[]; vpn: string[]; placeholders: string[]; visited: number }> {
  const take = watchFailures(page);
  const links = await sidebarLinks(page, prefix);
  const out = { errors: [] as string[], vpn: [] as string[], placeholders: [] as string[], visited: links.length };
  take();
  for (const href of links) {
    await page.locator(`nav[aria-label="${NAV}"] a[href="${href}"]`).first().click();
    await page.waitForURL((u) => u.pathname.startsWith(href), { timeout: 15_000 }).catch(() => undefined);
    await settle(page);
    const errs = [...(await screenErrors(page)), ...take()];
    if (errs.length) out.errors.push(`${href}: ${[...new Set(errs)].join(' | ')}`);
    if (await page.getByText(VPN).count()) out.vpn.push(href);
    if (await page.getByText(PLACEHOLDER).count()) out.placeholders.push(href);
  }
  return out;
}

/** Signs in as qlcm.truong.a (two tenants) and enters TH Demo A from the tenant picker. */
async function signInAcademicManager(page: Page): Promise<void> {
  await signIn(page, 'qlcm.truong.a');
  await expect(page.getByRole('heading', { name: 'Chọn đơn vị làm việc' })).toBeVisible();
  await page.getByRole('radio', { name: /TH Demo A/ }).click();
  await page.getByRole('button', { name: 'Vào' }).click();
  await expect(page.getByRole('navigation', { name: NAV })).toBeVisible({ timeout: 30_000 });
}

/** "Học sinh theo khối": school picker filled with TH Demo A, grade 3 lists the seeded students; returns the page body. */
async function gradeThreeOfSchoolA(page: Page): Promise<{ items: { student_id: string; username?: string }[] }> {
  await spaGoto(page, '/school-lms/students');
  await expect(selected(page, 'Trường')).toHaveText(SCHOOL_A, { timeout: 120_000 });
  const res: Promise<Response> = page.waitForResponse((r) => /\/tenant\/schools\/[^/]+\/grades\/3\/students/.test(r.url()), { timeout: 60_000 });
  await pick(page, 'Khối', 'Khối 3');
  const r = await res;
  expect(r.status(), 'listTenantGradeStudents of grade 3').toBe(200);
  await expect(page.getByText('hs.an.3a').first()).toBeVisible({ timeout: 30_000 });
  return (await r.json()) as { items: { student_id: string; username?: string }[] };
}

test.describe('@ADR0095 Portal theo trường và khối (không còn lớp)', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');

  test('student account is refused with the student message (403 PORTAL_ACCESS_STAFF_ONLY)', async ({ page }) => {
    const login = page.waitForResponse((r) => r.url().endsWith('/auth/portal/login'), { timeout: 30_000 });
    await signIn(page, 'hs.an.3a');
    const r = await login;
    expect(r.status()).toBe(403);
    expect(((await r.json()) as { code?: string }).code).toBe('PORTAL_ACCESS_STAFF_ONLY');
    await expect(page.getByText('Tài khoản học sinh không dùng được cổng quản trị')).toBeVisible();
    await expect(page).toHaveURL(/\/login/);
  });

  test('ad.truong.a: "Học sinh theo khối" lists grade 3 of TH Demo A', async ({ page }) => {
    test.setTimeout(5 * 60_000);
    await signIn(page, 'ad.truong.a');
    const body = await gradeThreeOfSchoolA(page);
    expect(body.items.length).toBeGreaterThan(0);
    studentOfSchoolA = (body.items.find((s) => s.username === 'hs.an.3a') ?? body.items[0])!.student_id;
  });

  test('ad.truong.b: never sees TH Demo A nor a student of it', async ({ page }) => {
    test.setTimeout(5 * 60_000);
    await signIn(page, 'ad.truong.b');
    await spaGoto(page, '/school-lms/students');
    await expect(selected(page, 'Trường')).not.toHaveText('', { timeout: 120_000 });
    await openSelect(page, 'Trường');
    await openOptions(page).first().waitFor({ timeout: 30_000 });
    const options = await openOptions(page).allTextContents();
    expect(options.length).toBeGreaterThan(0);
    expect(options.filter((o) => SCHOOL_A.test(o.trim()))).toEqual([]);
    await page.keyboard.press('Escape');
    await spaGoto(page, '/school-lms/schools');
    await settle(page);
    await expect(page.getByText(SCHOOL_A)).toHaveCount(0);
    test.skip(!studentOfSchoolA, 'no student id of TH Demo A captured by the ad.truong.a test');
    // login history of a TH Demo A student opened by URL: the API hides it (404, no existence leak)
    const res = page.waitForResponse((r) => r.url().includes(`/tenant/students/${studentOfSchoolA}/`), { timeout: 60_000 });
    await spaGoto(page, `/school-lms/student-activity/${studentOfSchoolA}`);
    expect([403, 404]).toContain((await res).status());
    await expect(page.getByText('hs.an.3a')).toHaveCount(0);
  });

  test('qlcm.truong.a: tenant picker, then the school picker of TH Demo A is filled', async ({ page }) => {
    test.setTimeout(5 * 60_000);
    await signInAcademicManager(page);
    const body = await gradeThreeOfSchoolA(page);
    expect(body.items.length).toBeGreaterThan(0);
  });

  test('ad.truong.a: every Content Studio screen (Curriculum included) loads', async ({ page }) => {
    test.setTimeout(10 * 60_000);
    await signIn(page, 'ad.truong.a');
    const r = await visitWorkspace(page, '/content-studio');
    expect(r.visited).toBeGreaterThan(0);
    expect(r.errors, r.errors.join('\n')).toEqual([]);
    expect(r.vpn).toEqual([]);
    await spaGoto(page, '/content-studio/curriculum');
    await settle(page);
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    expect(await screenErrors(page)).toEqual([]);
  });

  test('sa.admin: no VPN notice on /platform-admin/*, Curriculum loads', async ({ page }) => {
    test.setTimeout(15 * 60_000);
    await signIn(page, 'sa.admin');
    const r = await visitWorkspace(page, '/platform-admin');
    expect(r.visited).toBeGreaterThan(0);
    expect(r.vpn, `VPN notice on ${r.vpn.join(', ')}`).toEqual([]);
    test.info().annotations.push({ type: 'platform-admin errors', description: r.errors.join('\n') || 'none' });
    const take = watchFailures(page);
    await spaGoto(page, '/content-studio/curriculum');
    await settle(page);
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    const errs = [...(await screenErrors(page)), ...take()];
    expect(errs, errs.join('\n')).toEqual([]);
  });

  test('ad.truong.a: assignment wizard targets a grade of a school (nothing is published)', async ({ page }) => {
    test.setTimeout(10 * 60_000);
    await signIn(page, 'ad.truong.a');
    await spaGoto(page, '/school-lms/assignments/new');
    const picker = page.getByTestId('content-picker');
    await expect(picker).toBeVisible({ timeout: 60_000 });
    let chosen = false;
    for (const tab of await picker.getByRole('tab').all()) {
      await tab.click();
      await settle(page);
      const pickButton = picker.getByRole('button', { name: 'Chọn', exact: true }).first();
      if (await pickButton.isVisible().catch(() => false)) {
        await pickButton.click();
        chosen = true;
        break;
      }
    }
    // every tab answered (published lists of learning, game catalog); with none of them holding content the seed has
    // nothing to assign yet: a missing-seed finding, not a portal failure
    expect(await screenErrors(page)).toEqual([]);
    test.skip(!chosen, 'missing seed data: no published content in any tab of the content picker (activities, question sets, blueprints, lessons, game activities)');
    await expect(page.getByTestId('selected-content')).toBeVisible();
    await page.getByRole('button', { name: 'Tiếp' }).click();
    // step "Đối tượng": school of the scope picked automatically, subject, grade 3
    await expect(selected(page, 'Trường')).toHaveText(SCHOOL_A, { timeout: 120_000 });
    await pick(page, 'Môn học');
    await pick(page, 'Khối được giao', 'Khối 3');
    await page.getByRole('button', { name: 'Tiếp' }).click();
    // reaching "Thời gian" proves the targets step validated; the wizard is left there (no createAssignment)
    await expect(page.locator('.ant-steps-item-active')).toContainText('Thời gian');
    expect(await screenErrors(page)).toEqual([]);
  });

  test('ad.truong.a: grade page of the School Learning Dashboard opens', async ({ page }) => {
    test.setTimeout(5 * 60_000);
    await signIn(page, 'ad.truong.a');
    const take = watchFailures(page);
    await spaGoto(page, '/school-lms/school-dashboard');
    await settle(page);
    const gradeLink = page.locator('a[href*="/school-dashboard/grades/"]').first();
    if (!(await gradeLink.isVisible().catch(() => false))) {
      await pick(page, 'Trường', SCHOOL_A).catch(() => undefined);
      await settle(page);
    }
    await expect(gradeLink).toBeVisible({ timeout: 60_000 });
    await gradeLink.click();
    await expect(page).toHaveURL(/\/school-dashboard\/grades\/\d+/);
    await settle(page);
    const errs = [...(await screenErrors(page)), ...take()];
    expect(errs, errs.join('\n')).toEqual([]);
  });
});
