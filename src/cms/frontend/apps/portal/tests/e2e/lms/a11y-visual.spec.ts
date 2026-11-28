// @F055 T043 @visual — TC-UI-006 (axe) and TC-UI-004 (visual regression) of the assignment wizard and the assignment
// detail on the REAL stack, as `ad.truong.a` (TH Demo A), in light, dark and compact (theme/density switched in the
// shell without reload). axe: WCAG 2.0/2.1 A + AA, a serious/critical violation fails (same rule set as the ops-ui
// Storybook runner). Visual: viewport screenshots with the seed-dependent regions masked (content list, assignment
// title, facts, submissions) against reviewed baselines in __screenshots__/<platform>/.
// "Lịch" of T043 is gone with the calendar (ADR-0095). The wizard stays on step 1: nothing is created.
// Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise (OQ F051-9). No secret is written anywhere (AGENTS R4).
import { expect, test, type Locator, type Page } from '@playwright/test';
import { axeViolations, describeFindings } from '../support/a11y';
import { applyAppearance, APPEARANCES, type Appearance } from '../support/appearance';
import { e2eReady } from '../support/auth';
import { spaGoto } from '../support/nav';
import { openSession, type Session } from '../support/session';

interface Screen {
  name: string;
  url: () => string;
  ready: (page: Page) => Locator;
  mask: (page: Page) => Locator[];
}

let assignmentId: string | undefined;

const SCREENS: Screen[] = [
  {
    name: 'lms-wizard',
    url: () => '/school-lms/assignments/new',
    ready: (page) => page.getByTestId('content-picker'),
    mask: (page) => [page.getByTestId('content-picker').getByRole('tabpanel')],
  },
  {
    name: 'lms-assignment-detail',
    url: () => `/school-lms/assignments/${assignmentId}`,
    ready: (page) => page.getByTestId('assignment-detail'),
    mask: (page) => {
      const d = page.getByTestId('assignment-detail');
      return [d.locator('h1, h2, h3, h4, h5'), d.locator('.ant-descriptions'), d.locator('.ant-table-tbody')];
    },
  },
];

async function settle(page: Page): Promise<void> {
  await page
    .waitForFunction(() => !document.querySelector('.ant-skeleton-active, .ant-spin-spinning'), undefined, { timeout: 30_000 })
    .catch(() => undefined);
  await page.evaluate(() => document.fonts.ready);
}

test.describe('@F055 @visual LMS — axe và visual regression (TC-UI-004, TC-UI-006)', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
  test.describe.configure({ mode: 'serial' });

  let s: Session;

  test.beforeAll(async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    s = await openSession(browser, 'ad.truong.a');
    const list = s.page.waitForResponse((r) => /\/api\/v1\/lms\/assignments(\?|$)/.test(r.url()) && r.request().method() === 'GET', { timeout: 60_000 });
    await spaGoto(s.page, '/school-lms/assignments');
    const body = (await (await list).json()) as { data?: { assignment_id?: string }[] };
    assignmentId = body.data?.find((a) => a.assignment_id)?.assignment_id;
  });
  test.afterAll(async () => s?.close());

  /** Opens the screen (once) and applies the appearance. */
  async function prepare(screen: Screen, appearance: Appearance): Promise<Page> {
    test.skip(screen.name === 'lms-assignment-detail' && !assignmentId, 'missing seed data: no assignment in TH Demo A (seed-content DEV assignment)');
    const { page } = s;
    if (!page.url().endsWith(screen.url())) {
      await spaGoto(page, screen.url());
      await expect(screen.ready(page)).toBeVisible({ timeout: 60_000 });
    }
    await applyAppearance(page, appearance);
    await settle(page);
    return page;
  }

  for (const screen of SCREENS) {
    for (const appearance of APPEARANCES) {
      const title = `${screen.name} — ${appearance}`;
      // every rule in every appearance, color-contrast of the dark theme included (selected tab, "← Quay lại" link and
      // danger button text: dark admin tokens of tokens@0.22.0 — ops.color.text.{accent,link,danger})
      test(`axe ${title}`, async () => {
        const page = await prepare(screen, appearance);
        const v = await axeViolations(page);
        expect(v, describeFindings(v)).toEqual([]);
      });
      test(`visual ${title}`, async () => {
        const page = await prepare(screen, appearance);
        await expect(page).toHaveScreenshot(`${screen.name}--${appearance}.png`, { mask: screen.mask(page) });
      });
    }
  }
});
