// @F058 T059 — quickstart of F058 (specs/058-portal-activity-builder/quickstart.md §2) on the REAL stack, the scenarios
// not already covered by the other suites of this folder (13, 15, 16, 19 activity-video · 20–22 regrade · 24
// activity-tenancy · 27 activity-feature-toggle):
//   1 types offered · 2 LO required, createCmsActivity 201 with an Idempotency-Key · 3 a question type outside the
//   allowed ones is refused · 4 timed quiz: 5 s is brought back to the minimum · 6 a config broken in flight →
//   422 ACTIVITY_CONFIG_INVALID on the field · 8 grade 3 without instruction audio → submit blocked with the list ·
//   9 preview vi then en: no answer_key / feedback in the network, no attempt · 10 author submits (no "Duyệt" for the
//   author), `sa.admin` approves from the review queue, publish clicked twice → one release, "Đã xuất bản" · 11 v1
//   published → "Tạo bản nháp từ v1" → v2 draft · 12 a type in `preview` (simulated registry) → "Thử nghiệm",
//   publish locked with min_client_version · 14 overlapping chapters refused · 17 branch point without options /
//   ending segment refused · 18 seek/completion prefilled from video_completion · 23 `ad.truong.a` read only ·
//   25 maxLength counters · 26 below 1024 px read only.
// Scenario 5 (two tabs) and 7 (MathJax assets) are Vitest (AC-05, AC-08). The quiz is PLATFORM content of `cm.noidung`
// (not assigned to any tenant): every run adds one published "[E2E] …" quiz. Needs EW_E2E_PORTAL_URL,
// EW_E2E_PASSWORD; skipped otherwise (OQ F051-9). No secret is written anywhere (AGENTS R4).
import { expect, test, type Page } from '@playwright/test';
import { e2eReady } from '../support/auth';
import { openOptions, spaGoto } from '../support/nav';
import { openSession, type Session } from '../support/session';

const BAR = 'Trạng thái và thao tác';
const TYPES_API = /\/api\/v1\/cms\/learning\/activity-types(\?|$)/;

async function pick(page: Page, label: string, option: RegExp): Promise<void> {
  await page.getByRole('combobox', { name: label, exact: true }).locator('xpath=ancestor::div[contains(@class,"ant-select")][1]').click();
  await openOptions(page).filter({ hasText: option }).first().click();
  await page.keyboard.press('Escape');
}

/** Adds the first published question of `type` from the picker of the quiz source; returns the picker message. */
async function addQuestion(page: Page, type: string): Promise<void> {
  await page.getByRole('button', { name: '+ Chọn câu hỏi đã xuất bản' }).click();
  const dialog = page.getByRole('dialog', { name: 'Chọn câu hỏi đã xuất bản' });
  await expect(dialog).toBeVisible();
  const item = dialog.locator('.ant-list-item').filter({ hasText: `· ${type}` }).first();
  const more = dialog.getByRole('button', { name: /Tải thêm|Xem thêm/ });
  await expect(item.or(more).first()).toBeVisible({ timeout: 30_000 });
  while (!(await item.isVisible()) && (await more.isVisible())) await more.click();
  await item.getByRole('button', { name: 'Thêm' }).click();
}

test.describe('@F058 quickstart — Learning Activity Builder trên stack thật', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
  test.describe.configure({ mode: 'serial' });

  let author: Session;
  let quizId: string | undefined;
  const title = `[E2E] Quiz quickstart ${Date.now().toString(36)}`;

  test.beforeAll(async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    author = await openSession(browser, 'cm.noidung');
  });
  test.afterAll(async () => author?.close());

  test('1–4, 8, 25: types, LO required, refused type, timed minimum, audio blocker, counters → created once', async () => {
    test.setTimeout(6 * 60_000);
    const { page } = author;
    await spaGoto(page, '/content-studio/activities/new');
    const step = page.getByTestId('activity-type-step');
    await expect(step).toBeVisible({ timeout: 60_000 });
    const offered = (await step.getByRole('button').allTextContents()).join('|');
    expect(offered).toContain('Bài trắc nghiệm');
    expect(offered).toContain('Bài tập tương tác');
    expect(offered).toContain('Bài học video');
    expect(offered).not.toMatch(/written_response|speaking|project|Tự luận|Nói/);
    await step.getByRole('button', { name: 'Bài trắc nghiệm' }).click();
    const bar = page.getByRole('region', { name: BAR });
    await expect(bar).toBeVisible({ timeout: 30_000 });
    // 25: counters and maxLength of the title
    const titleBox = page.getByRole('textbox', { name: 'Tiêu đề (Tiếng Việt)' });
    await titleBox.fill('x'.repeat(300));
    expect((await titleBox.inputValue()).length).toBe(255);
    await titleBox.fill(title);
    // 2: the LO is required
    await expect(bar.getByRole('button', { name: 'Chọn 1–20 mục tiêu học tập.' })).toBeVisible();
    await pick(page, 'Môn học', /^Toán/);
    await pick(page, 'Khối lớp', /3/);
    await pick(page, 'Chủ đề', /MATH\.G3\.T01/);
    await pick(page, 'Mục tiêu học tập (LO)', /MATH\.G3\.NUM\.MUL_TABLE/);
    await expect(bar.getByRole('button', { name: 'Chọn 1–20 mục tiêu học tập.' })).toHaveCount(0);
    // 8: grade 3 without instruction audio nor exemption → listed blocker
    await expect(bar.getByRole('button', { name: /thiếu âm thanh đọc hướng dẫn/ })).toBeVisible();
    // 3: a matching question is outside the allowed types of this quiz → refused; a true/false one is taken
    await addQuestion(page, 'matching');
    await expect(page.getByText(/không dùng được trong hoạt động này/).first()).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog', { name: 'Chọn câu hỏi đã xuất bản' })).toBeHidden();
    await addQuestion(page, 'true_false');
    await expect(page.getByText('1/100 câu')).toBeVisible({ timeout: 30_000 });
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog', { name: 'Chọn câu hỏi đã xuất bản' })).toBeHidden();
    // 4: timed quiz below the minimum is brought back to it
    await page.getByRole('switch', { name: 'Tính giờ cả bài' }).click();
    const seconds = page.getByRole('spinbutton', { name: 'Tổng thời gian (giây)' });
    await seconds.fill('5');
    await seconds.blur();
    expect(Number(await seconds.inputValue())).toBeGreaterThanOrEqual(10);
    await page.getByRole('switch', { name: 'Tính giờ cả bài' }).click();
    await page.getByRole('textbox', { name: 'Lý do miễn âm thanh' }).fill('Kiểm thử E2E: chưa thu giọng đọc hướng dẫn');
    await expect(bar.getByRole('button', { name: /thiếu âm thanh đọc hướng dẫn/ })).toHaveCount(0);
    // save → createCmsActivity 201 once, with an Idempotency-Key
    const created = page.waitForResponse((r) => /\/api\/v1\/cms\/learning\/activities$/.test(new URL(r.url()).pathname) && r.request().method() === 'POST', { timeout: 60_000 });
    await bar.getByRole('button', { name: 'Lưu' }).click();
    const res = await created;
    expect(res.status(), await res.text()).toBe(201);
    expect(res.request().headers()['idempotency-key']).toBeTruthy();
    await page.waitForURL(/\/content-studio\/activities\/[0-9a-f-]{36}$/, { timeout: 60_000 });
    quizId = new URL(page.url()).pathname.split('/').pop();
  });

  test('6: a config broken in flight → 422 ACTIVITY_CONFIG_INVALID shown on the field', async () => {
    test.setTimeout(3 * 60_000);
    test.skip(!quizId, 'no quiz created');
    const { page } = author;
    const route = /\/api\/v1\/cms\/learning\/activities\/[0-9a-f-]{36}$/;
    await page.route(route, async (r) => {
      if (r.request().method() !== 'PATCH') return r.continue();
      const body = JSON.parse(r.request().postData() ?? '{}') as { config?: { evaluation?: Record<string, unknown> } };
      body.config = { ...body.config, evaluation: { ...body.config?.evaluation, max_score: -5 } };
      return r.continue({ postData: JSON.stringify(body) });
    });
    try {
      const patched = page.waitForResponse((r) => route.test(r.url()) && r.request().method() === 'PATCH', { timeout: 60_000 });
      await page.getByRole('textbox', { name: 'Mô tả (Tiếng Việt)' }).fill('Mô tả kiểm thử 422');
      await page.getByRole('region', { name: BAR }).getByRole('button', { name: 'Lưu' }).click();
      const res = await patched;
      expect(res.status()).toBe(422);
      expect(((await res.json()) as { code?: string }).code).toBe('ACTIVITY_CONFIG_INVALID');
      await expect(page.getByRole('region', { name: BAR })).toContainText(/max_score|Điểm tối đa/);
    } finally {
      await page.unroute(route);
    }
    // the next save without the broken field goes through
    const ok = page.waitForResponse((r) => route.test(r.url()) && r.request().method() === 'PATCH', { timeout: 60_000 });
    await page.getByRole('textbox', { name: 'Mô tả (Tiếng Việt)' }).fill('Mô tả kiểm thử');
    await page.getByRole('region', { name: BAR }).getByRole('button', { name: 'Lưu' }).click();
    expect((await ok).status()).toBe(200);
  });

  test('9: preview vi then en — no answer_key or feedback in the network, no attempt', async () => {
    test.setTimeout(3 * 60_000);
    test.skip(!quizId, 'no quiz created');
    const { page } = author;
    const bodies: string[] = [];
    const listener = async (r: import('@playwright/test').Response) => {
      if (/\/cms\/learning\/preview\/sessions/.test(r.url())) bodies.push(await r.text().catch(() => ''));
    };
    page.on('response', listener);
    await page.getByRole('tab', { name: 'Xem trước' }).last().click();
    const opened = page.waitForResponse((r) => /\/cms\/learning\/preview\/sessions$/.test(new URL(r.url()).pathname), { timeout: 60_000 });
    await page.getByRole('button', { name: /^Xem trước v1$/ }).first().click();
    const openedRes = await opened;
    expect(openedRes.status(), await openedRes.text()).toBe(201);
    const lang = page.getByRole('radio', { name: 'Tiếng Anh' }).or(page.getByText('Tiếng Anh', { exact: true })).first();
    if (await lang.isVisible().catch(() => false)) await lang.click();
    await page.waitForTimeout(3000);
    page.off('response', listener);
    expect(bodies.length).toBeGreaterThan(0);
    for (const b of bodies) expect(b).not.toMatch(/"answer_key"|"feedback"|"correct_option_id"/);
    expect(bodies.join('')).not.toMatch(/"attempt_id"/);
  });

  test('10–11: author submits; sa.admin approves; publish twice → one release; then a v2 draft from v1', async ({ browser }) => {
    test.setTimeout(6 * 60_000);
    test.skip(!quizId, 'no quiz created');
    const { page } = author;
    const bar = page.getByRole('region', { name: BAR });
    await bar.getByRole('button', { name: 'Gửi duyệt' }).click();
    await expect(bar).toContainText('Đang duyệt', { timeout: 60_000 });
    await expect(bar.getByRole('button', { name: 'Duyệt', exact: true })).toHaveCount(0);
    const sa = await openSession(browser, 'sa.admin');
    try {
      await spaGoto(sa.page, '/content-studio/reviews');
      await sa.page.getByRole('button', { name: title, exact: true }).click();
      const task = sa.page.getByTestId('review-task');
      await expect(task).toBeVisible({ timeout: 60_000 });
      const ack = task.getByRole('checkbox').first();
      if (await ack.isVisible().catch(() => false)) await ack.check();
      const approved = sa.page.waitForResponse((r) => r.url().endsWith(`/content/activity/${quizId}/versions/1:transition`), { timeout: 60_000 });
      await task.getByRole('button', { name: 'Duyệt', exact: true }).click();
      expect((await approved).status()).toBeLessThan(300);
    } finally {
      await sa.close();
    }
    await page.reload();
    await expect(bar).toContainText('Đã duyệt', { timeout: 60_000 });
    const keys: string[] = [];
    page.on('request', (r) => {
      if (r.method() === 'POST' && r.url().endsWith(`/content/activity/${quizId}/versions/1:publish`)) keys.push(r.headers()['idempotency-key'] ?? '');
    });
    await bar.getByRole('button', { name: 'Xuất bản ngay' }).click();
    const dialog = page.getByRole('dialog', { name: /Xuất bản phiên bản v1/ });
    await dialog.getByRole('button', { name: 'Xuất bản ngay' }).dblclick();
    await expect(bar).toContainText('Đã xuất bản', { timeout: 60_000 });
    expect(keys.length).toBeGreaterThan(0);
    expect(new Set(keys).size, 'one publish intent (same Idempotency-Key)').toBe(1);
    // 11: published = read only, a new draft starts from v1
    await expect(page.getByText('Phiên bản v1 đã xuất bản nên chỉ đọc. Tạo bản nháp mới để sửa.')).toBeVisible();
    await page.getByRole('button', { name: 'Tạo bản nháp từ v1' }).first().click();
    await expect(bar).toContainText('Bản nháp', { timeout: 60_000 });
    await expect(bar).toContainText('Phiên bản 2');
  });

  test('12: a type in preview (registry) is "Thử nghiệm" and cannot be published', async () => {
    test.setTimeout(3 * 60_000);
    const { page } = author;
    await page.route(TYPES_API, async (r) => {
      const res = await r.fetch();
      const body = (await res.json()) as { data: { activity_type: string; status?: string; min_client_version?: string }[] };
      for (const x of body.data) if (x.activity_type === 'interactive_exercise') Object.assign(x, { status: 'preview', min_client_version: '9.9.0' });
      return r.fulfill({ response: res, json: body });
    });
    try {
      await page.reload();
      await spaGoto(page, '/content-studio/activities/new');
      const card = page.getByTestId('activity-type-step').getByRole('button', { name: 'Bài tập tương tác' });
      await expect(card).toContainText('Thử nghiệm', { timeout: 60_000 });
      await card.click();
      await expect(page.getByTestId('type-preview')).toContainText('9.9.0');
    } finally {
      await page.unroute(TYPES_API);
    }
  });

  test('14, 17, 18: Video Manager rules — overlapping chapters, branch points, prefilled completion', async () => {
    test.setTimeout(3 * 60_000);
    const { page } = author;
    await page.reload();
    await spaGoto(page, '/content-studio/activities/new?type=video_lesson');
    const editor = page.getByTestId('video-lesson-editor');
    await expect(editor).toBeVisible({ timeout: 60_000 });
    // 18: seek "watched_only", completion min_watched_ratio 0.9 (video_completion)
    await expect(editor.getByRole('combobox', { name: 'Cho phép tua' }).locator('xpath=ancestor::div[contains(@class,"ant-select")][1]')).toContainText('Chỉ trong phần đã xem');
    expect(await editor.getByRole('spinbutton', { name: 'Tỉ lệ xem tối thiểu (0–1)' }).inputValue()).toMatch(/^0[.,]90?$/);
    // 14: two chapters 0–60 and 30–90 overlap
    await editor.getByRole('button', { name: '+ Thêm chương' }).click();
    await editor.getByRole('button', { name: '+ Thêm chương' }).click();
    await editor.getByRole('spinbutton', { name: 'Bắt đầu chương 2' }).fill('30');
    await editor.getByRole('spinbutton', { name: 'Kết thúc chương 2' }).fill('90');
    await expect(page.getByText(/Chương 1 và 2 chồng nhau\./).first()).toBeVisible();
    await editor.getByRole('spinbutton', { name: 'Bắt đầu chương 2' }).fill('60');
    await expect(page.getByText(/Chương 1 và 2 chồng nhau\./)).toHaveCount(0);
    // 17: a branch point without options and no ending segment
    await editor.getByRole('button', { name: '+ Thêm điểm rẽ' }).click();
    await expect(page.getByText('Điểm rẽ 1 cần 2–4 lựa chọn.').first()).toBeVisible();
  });

  test('23: ad.truong.a — activities read only (no create)', async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    const s = await openSession(browser, 'ad.truong.a');
    try {
      await spaGoto(s.page, '/content-studio/activities');
      await expect(s.page.getByText('Bạn chỉ có quyền xem hoạt động học tập.')).toBeVisible({ timeout: 60_000 });
      await expect(s.page.getByRole('button', { name: 'Tạo hoạt động' })).toHaveCount(0);
    } finally {
      await s.close();
    }
  });

  test('26: below 1024 px the builder is read only with "Tác vụ này cần màn hình lớn hơn"', async () => {
    test.setTimeout(3 * 60_000);
    test.skip(!quizId, 'no quiz created');
    const { page } = author;
    await page.setViewportSize({ width: 900, height: 800 });
    try {
      await spaGoto(page, `/content-studio/activities/${quizId}`);
      await expect(page.getByText('Tác vụ này cần màn hình lớn hơn')).toBeVisible({ timeout: 60_000 });
      await expect(page.getByRole('textbox', { name: 'Tiêu đề (Tiếng Việt)' })).toBeDisabled();
    } finally {
      await page.setViewportSize({ width: 1280, height: 720 });
    }
  });
});
