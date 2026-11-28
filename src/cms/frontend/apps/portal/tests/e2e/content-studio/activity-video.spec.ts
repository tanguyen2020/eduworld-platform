// @F058 T040 — US3 (F058-AC-15…AC-18, AC-21, TC-VID-004 MP4 part) on the REAL stack: `qlcm.truong.a` (TH Demo A)
// builds a `video_lesson` in the Video Manager from the seeded synthetic clip of `make seed-media` (3:20, ready, tenant
// media of th-demo-a): duration read from the asset (200 s, read only), 2 chapters (0–90 s, 90–200 s), the `vi` WebVTT
// subtitle as default track with "video có lời", a required embedded question at 01:30 (the seeded
// "[DEV] Câu chèn video: 3 × 4 = ?") → save (createCmsActivity with an Idempotency-Key) → preview (MP4 variant through the
// signed CDN URL: the chapter button jumps to 90 s, the player pauses at the cue, the `vi` track is there) → submit for
// review; `sa.admin` (another user than the author) approves the task of lane tenant from the review queue (with the
// audio exemption acknowledged) and the author publishes now → "Đã xuất bản" v1. Every run adds one published "[E2E] …" activity to th-demo-a (no delete of published
// content, ADR-0015). HLS preview (T039) stays open. Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise
// (OQ F051-9). No secret is written anywhere (AGENTS R4).
import { expect, test, type Locator, type Page } from '@playwright/test';
import { e2eReady } from '../support/auth';
import { openOptions, pickTenantFilter, spaGoto } from '../support/nav';
import { enterTenant, openSession, type Session } from '../support/session';

const VIDEO_FILE = 'devseed-g3-video-bang-nhan.mp4';
const SUBTITLE_VI = 'devseed-g3-video-bang-nhan.vi.vtt';
const CUE_QUESTION = '[DEV] Câu chèn video';
const TH_DEMO_A = /TH Demo A/;

/** Picks the option matching `option` of an antd select found by its accessible name. */
async function pick(page: Page, label: string, option: RegExp, scope: Page | Locator = page): Promise<void> {
  await scope.getByRole('combobox', { name: label, exact: true }).locator('xpath=ancestor::div[contains(@class,"ant-select")][1]').click();
  await openOptions(page).filter({ hasText: option }).first().click();
  await page.keyboard.press('Escape');
}

/** "Chọn từ thư viện" of a MediaPicker inside `scope`: search the library by file name and pick the file. */
async function pickMedia(page: Page, scope: Locator, filename: string): Promise<void> {
  await scope.getByRole('button', { name: 'Chọn từ thư viện' }).first().click();
  const dialog = page.getByRole('dialog', { name: 'Thư viện phương tiện' });
  await expect(dialog).toBeVisible();
  const search = dialog.getByLabel('Tìm theo tên tệp');
  await search.fill(filename);
  await search.press('Enter');
  const item = dialog.locator('.ant-list-item').filter({ hasText: filename }).first();
  await expect(item, `missing seed data: ${filename} of make seed-media`).toBeVisible({ timeout: 30_000 });
  await item.getByRole('button', { name: 'Chọn' }).click();
  await expect(dialog).toBeHidden();
}

test.describe('@F058 Video Manager — video 2 chương, phụ đề vi, cue 01:30 → xem trước → xuất bản', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
  test.describe.configure({ mode: 'serial' });

  const title = `[E2E] Video 2 chương ${Date.now().toString(36)}`;
  let activityId: string | undefined;
  let author: Session;

  test.beforeAll(async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    author = await openSession(browser, 'qlcm.truong.a');
    await enterTenant(author.page, TH_DEMO_A);
  });
  test.afterAll(async () => author?.close());

  test('qlcm.truong.a builds the video lesson and saves it', async () => {
    test.setTimeout(6 * 60_000);
    const { page } = author;
    const creates: string[] = [];
    page.on('request', (r) => {
      if (r.method() === 'POST' && /\/api\/v1\/cms\/learning\/activities$/.test(new URL(r.url()).pathname)) creates.push(r.headers()['idempotency-key'] ?? '');
    });
    await spaGoto(page, '/content-studio/activities/new');
    await page.getByRole('button', { name: 'Bài học video' }).click();
    await expect(page.getByTestId('video-lesson-editor')).toBeVisible({ timeout: 30_000 });
    // general information: Toán, grade 3, one LO, audio exemption (grade 1–3 instructions)
    await page.getByRole('textbox', { name: 'Tiêu đề (Tiếng Việt)' }).fill(title);
    await pick(page, 'Môn học', /^Toán/);
    await pick(page, 'Khối lớp', /3/);
    await pick(page, 'Chủ đề', /MATH\.G3\.T01/);
    await pick(page, 'Mục tiêu học tập (LO)', /MATH\.G3\.NUM\.MUL_TABLE/);
    await page.getByRole('textbox', { name: 'Lý do miễn âm thanh' }).fill('Dữ liệu kiểm thử E2E: chưa thu giọng đọc hướng dẫn');
    // the video: duration copied from the asset, read only (F058-AC-15)
    const editor = page.getByTestId('video-lesson-editor');
    await pickMedia(page, editor.locator('.ant-card').filter({ hasText: 'Thời lượng' }).first(), VIDEO_FILE);
    await expect(page.getByTestId('video-duration')).toContainText('200');
    // two chapters 0–90 s and 90–200 s (F058-AC-16)
    await editor.getByRole('button', { name: '+ Thêm chương' }).click();
    await editor.getByRole('button', { name: '+ Thêm chương' }).click();
    const chapters = page.getByTestId('video-chapters');
    for (const [n, name, start, end] of [
      [1, 'Phần 1: bảng nhân 3', '0', '90'],
      [2, 'Phần 2: luyện tập', '90', '200'],
    ] as const) {
      await chapters.getByRole('textbox', { name: `Tiêu đề chương ${n}` }).fill(name);
      await chapters.getByRole('spinbutton', { name: `Bắt đầu chương ${n}` }).fill(start);
      await chapters.getByRole('spinbutton', { name: `Kết thúc chương ${n}` }).fill(end);
    }
    // speech → a vi subtitle is required (F058-AC-17)
    await editor.getByRole('checkbox', { name: 'Video có lời (bắt buộc phụ đề tiếng Việt)' }).check();
    await editor.getByRole('button', { name: '+ Thêm phụ đề' }).click();
    await pickMedia(page, page.getByTestId('subtitle-0'), SUBTITLE_VI);
    // required embedded question at 01:30 (F058-AC-18)
    await editor.getByRole('button', { name: '+ Thêm câu hỏi chèn' }).click();
    const cues = page.getByTestId('video-cues');
    await cues.getByRole('spinbutton', { name: 'Thời điểm cue 1' }).fill('90');
    await cues.getByRole('button', { name: '+ Chọn câu hỏi đã xuất bản' }).click();
    const picker = page.getByRole('dialog', { name: 'Chọn câu hỏi đã xuất bản' });
    const search = picker.getByLabel('Tìm theo từ khóa (không cần dấu)');
    await search.fill(CUE_QUESTION);
    await search.press('Enter');
    // the picker lists the match by short id and type (the search runs on the stem)
    const q = picker.locator('.ant-list-item').filter({ hasText: 'single_choice' });
    await expect(q, 'missing seed data: embedded question of make seed-media').toHaveCount(1, { timeout: 30_000 });
    await q.getByRole('button', { name: 'Thêm' }).click();
    await expect(picker).toBeHidden();
    // save → createCmsActivity once (Idempotency-Key), then the editor of the new activity
    await page.getByRole('region', { name: 'Trạng thái và thao tác' }).getByRole('button', { name: 'Lưu' }).click();
    await page.waitForURL(/\/content-studio\/activities\/[0-9a-f-]{36}$/, { timeout: 60_000 });
    activityId = new URL(page.url()).pathname.split('/').pop();
    expect(creates.length).toBe(1);
    expect(creates[0]).toMatch(/.{8,}/);
  });

  test('preview: MP4 with the vi track, chapter jump to 01:30 pauses at the cue', async () => {
    test.setTimeout(3 * 60_000);
    test.skip(!activityId, 'no activity saved');
    const { page } = author;
    const preview = page.getByTestId('video-preview');
    await expect(preview).toBeVisible({ timeout: 60_000 });
    await expect(preview.locator('track[srclang="vi"]')).toHaveCount(1);
    const video = preview.locator('video');
    await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.readyState), { timeout: 60_000, message: 'MP4 variant loads from the CDN' }).toBeGreaterThanOrEqual(1);
    expect(await video.evaluate((v: HTMLVideoElement) => Math.round(v.duration))).toBe(200);
    await preview.getByRole('button', { name: 'Phần 2: luyện tập' }).click();
    await video.evaluate((v: HTMLVideoElement) => {
      v.muted = true;
      return v.play();
    });
    await expect(page.getByTestId('preview-cue')).toBeVisible({ timeout: 30_000 });
    expect(await video.evaluate((v: HTMLVideoElement) => v.paused)).toBe(true);
    expect(await video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThanOrEqual(90);
  });

  test('submit for review; sa.admin approves from the review queue; the author publishes v1', async ({ browser }) => {
    test.setTimeout(6 * 60_000);
    test.skip(!activityId, 'no activity saved');
    const { page } = author;
    const bar = page.getByRole('region', { name: 'Trạng thái và thao tác' });
    await bar.getByRole('button', { name: 'Gửi duyệt' }).click();
    await expect(bar).toContainText('Đang duyệt', { timeout: 60_000 });
    // the author never approves its own version
    await expect(bar.getByRole('button', { name: 'Duyệt', exact: true })).toHaveCount(0);
    const sa = await openSession(browser, 'sa.admin');
    try {
      const p = sa.page;
      // TH Demo A has no tenant reviewer in the seed: the System Admin reviews the task of lane "tenant" for its tenant
      // (the task page sends the X-Tenant-Id of the task — fixed with this suite)
      await spaGoto(p, '/content-studio/reviews');
      await pickTenantFilter(p, TH_DEMO_A);
      const reads: string[] = [];
      p.on('request', (r) => {
        if (new URL(r.url()).pathname.endsWith(`/content/activity/${activityId}/versions/1`)) reads.push(r.headers()['x-tenant-id'] ?? '');
      });
      await p.getByRole('button', { name: title, exact: true }).click();
      const task = p.getByTestId('review-task');
      await expect(task).toBeVisible({ timeout: 60_000 });
      expect(reads.length).toBeGreaterThan(0);
      expect(new Set(reads).size, 'every read of the version carries the tenant of the task').toBe(1);
      expect(reads[0]).toMatch(/^[0-9a-f-]{36}$/);
      // the version declares an audio exemption (grade 3 instructions): the reviewer acknowledges it
      await task.getByRole('checkbox').first().check();
      const approved = p.waitForResponse((r) => r.url().endsWith(`/content/activity/${activityId}/versions/1:transition`), { timeout: 60_000 });
      await task.getByRole('button', { name: 'Duyệt', exact: true }).click();
      expect((await approved).status()).toBeLessThan(300);
    } finally {
      await sa.close();
    }
    // approved → the author publishes now (publish is not a review decision)
    await page.reload();
    await expect(bar).toContainText('Đã duyệt', { timeout: 60_000 });
    await bar.getByRole('button', { name: 'Xuất bản ngay' }).click();
    const dialog = page.getByRole('dialog', { name: /Xuất bản phiên bản v1/ });
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', { name: 'Xuất bản ngay' }).click();
    await expect(bar).toContainText('Đã xuất bản', { timeout: 60_000 });
  });
});
