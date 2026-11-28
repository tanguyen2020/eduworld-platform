// @F055 T015 — TC-LMS-001 on the REAL stack: `ad.truong.a` assigns through the 5-step wizard a published activity to
// grade 3 of TH Demo A leaving `hs.chi.3b` out (`exclude_student_ids`; both students are grade 3 since ADR-0095, spec
// Clarifications 2026-11-28), publishes it once (createAssignment + publishAssignment with their Idempotency-Key), and:
// - the submissions of the new assignment (listAssignmentSubmissions, detail page) hold `hs.an.3a` and not `hs.chi.3b`, and
//   the "Học sinh" column shows each name (joined from listTenantGradeStudents, F055-AC-08);
// - the students themselves: `listMyAssignments` of `hs.an.3a` has the new assignment and the seeded
//   "[DEV] Xem video bảng nhân" (`make seed-media`: grade 3 except hs.chi.3b), the one of `hs.chi.3b` has neither but
//   still the seeded "[DEV] Phép chia có dư (golden)" given to the whole grade. Each student signs in to the game API
//   ONCE for the whole suite (POST /auth/game/login, password only from EW_E2E_PASSWORD) and signs out at once;
// - the new assignment is then cancelled with a reason (no submission), so a rerun leaves the dashboard of the grade as
//   it was (golden dataset, F056 T021).
// The gateway base URL is the `apiBaseUrl` of the portal's /config.json (AGENTS R3). Needs EW_E2E_PORTAL_URL,
// EW_E2E_PASSWORD; skipped otherwise (OQ F051-9). No secret, token or student name is written anywhere (AGENTS R4, R14).
import { randomUUID } from 'node:crypto';
import { expect, request as pwRequest, test, type APIRequestContext, type Page } from '@playwright/test';
import { fetchAllPages, type CursorPageLike } from '../../../src/shared/api/fetchAllPages';
import { e2eReady } from '../support/auth';
import { openOptions, selectedItem, spaGoto } from '../support/nav';
import { openSession, type Session } from '../support/session';

/** Seeded assignment of `make seed-media` (grade 3 of DEMOA1 except hs.chi.3b) and one given to the whole grade. */
const SEEDED_EXCLUDING_CHI = '[DEV] Xem video bảng nhân';
const SEEDED_WHOLE_GRADE = '[DEV] Phép chia có dư (golden)';
/** Published platform activity of `make seed-content` assigned to th-demo-a. */
const ACTIVITY = '[DEV] Quiz bảng nhân 6–9';
/** Game client identity of the sign-in (≥ EW_GAME_MIN_CLIENT_VERSION of the local stack, like `make seed-attempts`). */
const CLIENT_VERSION = process.env.EW_E2E_CLIENT_VERSION ?? '0.1.0';

const pad = (n: number) => String(n).padStart(2, '0');
/** Local date-time in the format of the antd DatePicker with time (YYYY-MM-DD HH:mm:ss). */
const pickerValue = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;

interface MyAssignment {
  assignment_id: string;
  title: string;
}

async function apiBaseUrl(): Promise<string> {
  const portal = process.env.EW_E2E_PORTAL_URL!;
  const ctx = await pwRequest.newContext();
  try {
    const cfg = (await (await ctx.get(new URL('/config.json', portal).toString())).json()) as { apiBaseUrl: string };
    return cfg.apiBaseUrl.replace(/\/$/, '');
  } finally {
    await ctx.dispose();
  }
}

/** Signs `username` in to the game API once, reads every page of listMyAssignments, signs out. */
async function myAssignments(base: string, username: string): Promise<MyAssignment[]> {
  const api: APIRequestContext = await pwRequest.newContext();
  const device = randomUUID();
  const client = { 'X-Device-Id': device, 'X-Client-Version': CLIENT_VERSION, 'X-Client-Platform': 'web' };
  try {
    const login = await api.post(`${base}/auth/game/login`, {
      data: { username, password: process.env.EW_E2E_PASSWORD, device_id: device, client_version: CLIENT_VERSION, platform: 'web', locale: 'vi-VN' },
    });
    expect(login.status(), `game login of ${username}`).toBe(200);
    const body = (await login.json()) as { tokens: { access_token: string; refresh_token: string }; bootstrap: { tenant_id: string } };
    const headers = { ...client, Authorization: `Bearer ${body.tokens.access_token}`, 'X-Tenant-Id': body.bootstrap.tenant_id };
    try {
      // every cursor page (the whole list is asserted on)
      const all = await fetchAllPages<MyAssignment>(async (query) => {
        const r = await api.get(`${base}/learning/my-assignments`, { headers, params: { ...query } });
        expect(r.status(), `listMyAssignments of ${username}`).toBe(200);
        return (await r.json()) as CursorPageLike<MyAssignment>;
      });
      expect(all.truncated).toBe(false);
      return all.items;
    } finally {
      await api.post(`${base}/auth/game/logout`, { headers, data: { refresh_token: body.tokens.refresh_token } }).catch(() => undefined);
    }
  } finally {
    await api.dispose();
  }
}

/** Picks the option of an antd select (by accessible name) whose label matches `option`. */
async function pick(page: Page, label: string, option: RegExp): Promise<void> {
  await page.getByRole('combobox', { name: label, exact: true }).click({ force: true });
  await openOptions(page).filter({ hasText: option }).first().click();
}

test.describe('@F055 LMS — giao bài khối 3 trừ một học sinh (TC-LMS-001)', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
  test.describe.configure({ mode: 'serial' });

  let s: Session;
  const title = `[E2E] Giao bài khối 3 trừ hs.chi.3b ${Date.now().toString(36)}`;
  let assignmentId: string | undefined;
  const ids: Record<string, string> = {};
  /** Name shown for a student (display name of listTenantGradeStudents, else the username) — read at run time, never written. */
  const names: Record<string, string> = {};

  test.beforeAll(async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    s = await openSession(browser, 'ad.truong.a');
  });
  test.afterAll(async () => s?.close());

  test('ad.truong.a: wizard → grade 3 minus hs.chi.3b → published once', async () => {
    test.setTimeout(6 * 60_000);
    const { page } = s;
    const creates: string[] = [];
    const publishes: string[] = [];
    page.on('request', (r) => {
      const p = new URL(r.url()).pathname;
      if (r.method() !== 'POST') return;
      if (/\/api\/v1\/lms\/assignments$/.test(p)) creates.push(r.headers()['idempotency-key'] ?? '');
      if (/\/api\/v1\/lms\/assignments\/[^/]+:publish$/.test(p)) publishes.push(r.headers()['idempotency-key'] ?? '');
    });
    await spaGoto(page, '/school-lms/assignments/new');
    // step 1 "Nội dung": the published quiz of the seed (tab Activity)
    const picker = page.getByTestId('content-picker');
    await expect(picker).toBeVisible({ timeout: 60_000 });
    await picker.getByRole('tab', { name: 'Activity' }).click();
    const item = picker.locator('.ant-list-item').filter({ hasText: ACTIVITY }).first();
    await expect(item, 'missing seed data: published activity of make seed-content').toBeVisible({ timeout: 60_000 });
    await item.getByRole('button', { name: 'Chọn', exact: true }).click();
    await expect(page.getByTestId('selected-content')).toContainText(ACTIVITY);
    await page.getByRole('button', { name: 'Tiếp' }).click();
    // step 2 "Đối tượng": school picked automatically, subject, grade 3, leave hs.chi.3b out
    await expect(selectedItem(page, 'Trường')).toHaveText(/Demo A$/, { timeout: 120_000 });
    await pick(page, 'Môn học', /^Toán/);
    await pick(page, 'Khối được giao', /^Khối 3$/);
    // the exclude picker lists grade 3 first (the targeted grade) and searches on the server (≥ 2 characters); the
    // search of hs.an.3a only gives its id (it stays in the assignment), hs.chi.3b is picked
    const exclude = page.getByRole('combobox', { name: 'Bớt học sinh', exact: true });
    await exclude.click({ force: true });
    for (const username of ['hs.an.3a', 'hs.chi.3b']) {
      const students = page.waitForResponse((r) => /\/tenant\/schools\/[^/]+\/grades\/3\/students/.test(r.url()) && new URL(r.url()).searchParams.get('q') === username, { timeout: 60_000 });
      await exclude.fill(username);
      const found = (await (await students).json()) as { items: { student_id: string; username?: string; display_name?: string }[] };
      const hit = found.items.find((x) => x.username === username);
      ids[username] = hit?.student_id ?? '';
      names[username] = hit?.display_name || username;
      expect(ids[username], `${username} is a grade 3 student of TH Demo A`).toBeTruthy();
    }
    await openOptions(page).filter({ hasText: '(hs.chi.3b)' }).first().click();
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: 'Tiếp' }).click();
    // step 3 "Thời gian"
    await expect(page.locator('.ant-steps-item-active')).toContainText('Thời gian');
    await page.locator('#asg-title').fill(title);
    const now = new Date();
    for (const [label, at] of [
      ['Mở bài lúc', now],
      ['Hạn nộp', new Date(now.getTime() + 7 * 86_400_000)],
    ] as const) {
      const input = page.getByRole('textbox', { name: label, exact: true }).or(page.getByLabel(label, { exact: true })).first();
      await input.click();
      await input.fill(pickerValue(at));
      await input.press('Enter');
    }
    await page.getByRole('button', { name: 'Tiếp' }).click();
    // step 4 "Chính sách": practice, 1 attempt (defaults)
    await expect(page.locator('.ant-steps-item-active')).toContainText('Chính sách');
    await page.getByRole('button', { name: 'Tiếp' }).click();
    // step 5 "Xem lại" → "Giao bài": one createAssignment + one publishAssignment, each with an Idempotency-Key
    const review = page.getByTestId('wizard-review');
    await expect(review).toContainText('Khối 3 · bớt 1 học sinh');
    const detail = page.waitForURL(/\/school-lms\/assignments\/[0-9a-f-]{36}$/, { timeout: 120_000 });
    await page.getByRole('button', { name: 'Giao bài', exact: true }).click();
    await detail;
    assignmentId = new URL(page.url()).pathname.split('/').pop();
    expect(creates.length, 'one createAssignment').toBe(1);
    expect(creates[0]).toMatch(/.{8,}/);
    expect(publishes.length, 'one publishAssignment').toBe(1);
    expect(publishes[0]).toMatch(/.{8,}/);
  });

  test('detail: the submissions hold hs.an.3a, not hs.chi.3b', async () => {
    test.setTimeout(3 * 60_000);
    test.skip(!assignmentId, 'no assignment created');
    const { page } = s;
    const subs = page.waitForResponse((r) => new RegExp(`/lms/assignments/${assignmentId}/submissions`).test(r.url()) && r.request().method() === 'GET', { timeout: 60_000 });
    await page.reload();
    await expect(page.getByTestId('assignment-detail')).toBeVisible({ timeout: 60_000 });
    const rows = ((await (await subs).json()) as { data: { student_id: string }[] }).data;
    expect(rows.map((r) => r.student_id)).toContain(ids['hs.an.3a']);
    expect(rows.map((r) => r.student_id)).not.toContain(ids['hs.chi.3b']);
    await expect(page.getByTestId('assignment-detail').locator('.ant-table-tbody tr')).not.toHaveCount(0);
    // F055-AC-08: the "Học sinh" column shows the name joined from listTenantGradeStudents (module learning keeps no name),
    // no row is left blank or numbered for ad.truong.a (lms.* in the school)
    const table = page.getByTestId('submission-table');
    await expect(table.getByRole('cell', { name: names['hs.an.3a'], exact: true })).toBeVisible({ timeout: 60_000 });
    const firstCells = await table.locator('.ant-table-tbody tr td:first-child').allTextContents();
    expect(firstCells.length).toBe(rows.length);
    for (const c of firstCells) expect(c.trim(), 'every submission row has a student name').not.toMatch(/^$|^Học sinh \d+$/);
  });

  test('students: hs.an.3a sees it (and the seeded video), hs.chi.3b does not', async () => {
    test.setTimeout(3 * 60_000);
    test.skip(!assignmentId, 'no assignment created');
    const base = await apiBaseUrl();
    const an = await myAssignments(base, 'hs.an.3a');
    expect(an.map((a) => a.assignment_id)).toContain(assignmentId);
    expect(an.map((a) => a.title)).toContain(SEEDED_EXCLUDING_CHI);
    const chi = await myAssignments(base, 'hs.chi.3b');
    expect(chi.map((a) => a.assignment_id)).not.toContain(assignmentId);
    expect(chi.map((a) => a.title)).not.toContain(SEEDED_EXCLUDING_CHI);
    expect(chi.map((a) => a.title), 'hs.chi.3b still gets the assignments of the whole grade').toContain(SEEDED_WHOLE_GRADE);
  });

  test('ad.truong.a cancels the E2E assignment (reason, no submission)', async () => {
    test.setTimeout(3 * 60_000);
    test.skip(!assignmentId, 'no assignment created');
    const { page } = s;
    await page.getByRole('button', { name: 'Hủy bài', exact: true }).click();
    const dialog = page.getByRole('dialog');
    await dialog.getByLabel('Lý do').fill('Dọn dữ liệu sau kiểm thử E2E F055 T015');
    const cancelled = page.waitForResponse((r) => new RegExp(`/lms/assignments/${assignmentId}:cancel`).test(r.url()), { timeout: 60_000 });
    await dialog.getByRole('button', { name: 'Hủy bài', exact: true }).click();
    expect((await cancelled).status()).toBeLessThan(300);
  });
});
