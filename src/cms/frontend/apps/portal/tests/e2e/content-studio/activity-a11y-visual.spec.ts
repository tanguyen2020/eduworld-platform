// @F058 T057 @visual — TC-UI-006 (axe) and TC-UI-004 (visual regression) on the REAL stack, as `qlcm.truong.a`
// (TH Demo A), in light, dark and compact (theme/density switched in the shell without reload):
// - quiz builder: a new `quiz` (nothing is saved);
// - Video Manager: the seeded "[DEV] Video bảng nhân (3:20)" of `make seed-media` (published, read only: chapters,
//   subtitles, cue, preview);
// - system re-evaluation dialog: version 1 of the seeded regrade question (`make seed-attempts`) → "Chấm lại hệ
//   thống…" (opened and closed, nothing is sent).
// axe: WCAG 2.0/2.1 A + AA, a serious/critical violation fails (support/a11y.ts). Visual: viewport (dialog: the modal
// only; Video Manager scrolled to the top of the viewport) against reviewed baselines in __screenshots__/<platform>/, data regions masked (ids, times, video frame).
// Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise (OQ F051-9). No secret is written anywhere (AGENTS R4).
import { expect, test, type Locator, type Page } from '@playwright/test';
import { axeViolations, describeFindings } from '../support/a11y';
import { applyAppearance, APPEARANCES, type Appearance } from '../support/appearance';
import { e2eReady } from '../support/auth';
import { spaGoto } from '../support/nav';
import { enterTenant, openSession, type Session } from '../support/session';

const VIDEO = '[DEV] Video bảng nhân (3:20)';
const QUESTION = 'Chu vi hình vuông cạnh 4 cm';

let videoId: string | undefined;

async function settle(page: Page): Promise<void> {
  await page
    .waitForFunction(() => !document.querySelector('.ant-skeleton-active, .ant-spin-spinning'), undefined, { timeout: 30_000 })
    .catch(() => undefined);
  await page.evaluate(() => document.fonts.ready);
}

interface Screen {
  name: string;
  open: (page: Page) => Promise<void>;
  /** Element of the screenshot (the viewport otherwise). */
  shot?: (page: Page) => Locator;
  mask: (page: Page) => Locator[];
  /** axe context. */
  scope?: string;
  /** Runs after the appearance is applied, before axe/screenshot (scroll to the part under test). */
  settle?: (page: Page) => Promise<void>;
  /** Regions left out of axe, with the reason next to the value. */
  exclude?: string[];
}

const SCREENS: Screen[] = [
  {
    name: 'activity-quiz-builder',
    open: async (page) => {
      await spaGoto(page, '/content-studio/activities/new?type=quiz');
      await expect(page.getByRole('region', { name: 'Trạng thái và thao tác' })).toBeVisible({ timeout: 60_000 });
    },
    mask: () => [],
  },
  {
    name: 'activity-video-manager',
    open: async (page) => {
      test.skip(!videoId, `missing seed data: ${VIDEO} (make seed-media)`);
      await spaGoto(page, `/content-studio/activities/${videoId}`);
      await expect(page.getByTestId('video-lesson-editor')).toBeVisible({ timeout: 60_000 });
    },
    // the published version is read only: the subtitle language selects are DISABLED (inactive controls are exempt
    // from WCAG 1.4.3; axe cannot tie the value span to the disabled input)
    exclude: ['.ant-select-disabled .ant-select-selection-item'],
    // the Video Manager at the top of the viewport (scrollIntoView start: deterministic, unlike IfNeeded)
    settle: (page) => page.getByTestId('video-lesson-editor').evaluate((el) => el.scrollIntoView({ block: 'start' })),
    mask: (page) => [page.getByTestId('video-preview').locator('video'), page.getByTestId('video-cues').getByRole('button'), page.getByRole('region', { name: 'Trạng thái và thao tác' })],
  },
  {
    name: 'regrade-dialog',
    open: async (page) => {
      await spaGoto(page, '/content-studio/question-bank');
      const search = page.getByLabel('Tìm theo từ khóa (không cần dấu)').first();
      await search.fill(QUESTION);
      await search.press('Enter');
      const rows = page.locator('.ant-table-tbody tr.ant-table-row');
      await expect(rows).toHaveCount(1, { timeout: 60_000 });
      await rows.first().getByRole('button', { name: 'Mở' }).click();
      await page.getByRole('button', { name: 'Phiên bản · Phát hành · Sử dụng · Xem trước' }).click();
      const v1 = page.locator('tr').filter({ hasText: /v1 / }).filter({ has: page.getByRole('button', { name: 'Xem' }) }).first();
      // the history table may re-render while it loads: click 'Xem' until the drawer of v1 is open
      await expect(async () => {
        await v1.getByRole('button', { name: 'Xem' }).click({ timeout: 5_000 });
        await expect(page.getByTestId('version-view')).toBeVisible({ timeout: 5_000 });
      }).toPass({ timeout: 60_000 });
      await page.getByTestId('regrade-entry').getByRole('button', { name: 'Chấm lại hệ thống…' }).click();
      await expect(page.getByTestId('regrade-dialog')).toBeVisible({ timeout: 60_000 });
    },
    shot: (page) => page.locator('.ant-modal-content').filter({ has: page.getByTestId('regrade-dialog') }),
    mask: () => [],
    scope: '.ant-modal-content',
  },
];

test.describe('@F058 @visual Activity Builder — axe và visual regression (TC-UI-004, TC-UI-006)', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
  test.describe.configure({ mode: 'serial' });

  let s: Session;
  let opened: string | undefined;

  test.beforeAll(async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    s = await openSession(browser, 'qlcm.truong.a');
    await enterTenant(s.page, /TH Demo A/);
    const list = s.page.waitForResponse((r) => /\/api\/v1\/cms\/learning\/activities(\?|$)/.test(r.url()) && r.request().method() === 'GET', { timeout: 60_000 });
    await spaGoto(s.page, '/content-studio/activities');
    const rows = ((await (await list).json()) as { data: { activity_id: string; title?: { vi?: string } }[] }).data;
    videoId = rows.find((a) => a.title?.vi === VIDEO)?.activity_id;
  });
  test.afterAll(async () => s?.close());

  /** Opens the screen (once; the dialog after each appearance switch — the shell controls sit behind the modal). */
  async function prepare(screen: Screen, appearance: Appearance): Promise<Page> {
    const { page } = s;
    if (screen.name === 'regrade-dialog') {
      // leave the question page (unmounts the drawer and the modal that cover the shell controls)
      await spaGoto(page, '/content-studio/question-bank');
      await applyAppearance(page, appearance);
      await screen.open(page);
      opened = screen.name;
    } else {
      if (opened !== screen.name) {
        await screen.open(page);
        opened = screen.name;
      }
      await applyAppearance(page, appearance);
    }
    await settle(page);
    await screen.settle?.(page);
    return page;
  }

  for (const screen of SCREENS) {
    for (const appearance of APPEARANCES) {
      const title = `${screen.name} — ${appearance}`;
      test(`axe ${title}`, async () => {
        test.setTimeout(2 * 60_000);
        const page = await prepare(screen, appearance);
        const v = await axeViolations(page, screen.scope ?? 'body', screen.exclude);
        expect(v, describeFindings(v)).toEqual([]);
      });
      test(`visual ${title}`, async () => {
        test.setTimeout(2 * 60_000);
        const page = await prepare(screen, appearance);
        const target = screen.shot?.(page);
        if (target) await expect(target).toHaveScreenshot(`${screen.name}--${appearance}.png`, { mask: screen.mask(page) });
        else await expect(page).toHaveScreenshot(`${screen.name}--${appearance}.png`, { mask: screen.mask(page) });
      });
    }
  }
});
