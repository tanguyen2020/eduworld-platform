// @F056 T046 @visual — TC-UI-004 visual regression and TC-UI-006 axe of the School Learning Dashboard on the REAL stack, as
// `ad.truong.a` (TH Demo A): overview, grade page (the class page is gone, ADR-0095) and the skill heatmap of the grade,
// in light, dark, compact and mobile (390 × 844, light). Viewport screenshots against reviewed baselines in
// __screenshots__/<platform>/; every seed- or time-dependent region is masked (greeting by hour, date range, "Cập nhật
// lúc" meta, widget numbers and lists), so the check covers the layout, theme and density of the screens — not data.
// axe (WCAG 2.0/2.1 A + AA, serious/critical fails — support/a11y.ts) runs on the overview and the grade page in light,
// dark and compact: it covers the warning tag "Tạm tính" of the widget meta line (Tag colorWarning tokens) and the
// accessible name of the submission progress bars of the overview (aria-progressbar-name) — findings of 2026-11-28.
// Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise (OQ F051-9). No secret is written anywhere (AGENTS R4).
import { expect, test, type Locator, type Page } from '@playwright/test';
import { axeViolations, describeFindings } from '../support/a11y';
import { applyAppearance, APPEARANCES, type Appearance } from '../support/appearance';
import { e2eReady } from '../support/auth';
import { spaGoto } from '../support/nav';
import { openSession, type Session } from '../support/session';

type Look = Appearance | 'mobile';
const LOOKS: readonly Look[] = ['light', 'dark', 'compact', 'mobile'];
const DESKTOP = { width: 1280, height: 720 };
const MOBILE = { width: 390, height: 844 };

let schoolId: string | undefined;

/** Regions that change with the seed or the clock. */
function dataMasks(page: Page): Locator[] {
  return [
    page.getByTestId('dashboard-greeting').locator('h1, h2, h3, h4, h5'),
    page.locator('.ant-picker-range'),
    page.getByTestId('dashboard-meta'),
    page.getByTestId('portal-content').locator('.ant-card-body'),
  ];
}

interface Shot {
  name: string;
  url: () => string;
  ready: (page: Page) => Locator;
  /** Element to capture (heatmap widget); the viewport otherwise. */
  element?: (page: Page) => Locator;
}

const SHOTS: Shot[] = [
  { name: 'dashboard-overview', url: () => '/school-lms/school-dashboard', ready: (p) => p.getByTestId('dashboard-greeting') },
  { name: 'dashboard-grade', url: () => `/school-lms/school-dashboard/grades/3?school=${schoolId}`, ready: (p) => p.getByTestId('widget-grade-summary') },
  {
    name: 'dashboard-heatmap',
    url: () => `/school-lms/school-dashboard/grades/3?school=${schoolId}`,
    ready: (p) => p.getByTestId('widget-heatmap'),
    element: (p) => p.getByTestId('widget-heatmap'),
  },
];

async function settle(page: Page): Promise<void> {
  await page
    .waitForFunction(() => !document.querySelector('.ant-skeleton-active, .ant-spin-spinning'), undefined, { timeout: 30_000 })
    .catch(() => undefined);
  await page.evaluate(() => document.fonts.ready);
}

test.describe('@F056 @visual Dashboard — axe và visual regression (TC-UI-004, TC-UI-006)', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
  test.describe.configure({ mode: 'serial' });

  let s: Session;

  test.beforeAll(async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    s = await openSession(browser, 'ad.truong.a');
    const req = s.page.waitForRequest((r) => /\/api\/v1\/lms\/dashboards\/school(\?|$)/.test(r.url()), { timeout: 60_000 });
    await spaGoto(s.page, '/school-lms/school-dashboard');
    schoolId = new URL((await req).url()).searchParams.get('school_id') ?? undefined;
  });
  test.afterAll(async () => s?.close());

  for (const shot of SHOTS) {
    for (const look of LOOKS) {
      test(`visual ${shot.name} — ${look}`, async () => {
        test.skip(!schoolId, 'no school of TH Demo A on the overview');
        const { page } = s;
        // the theme/density controls are in the desktop header: applied before the mobile viewport
        await page.setViewportSize(DESKTOP);
        await applyAppearance(page, look === 'mobile' ? 'light' : look);
        if (look === 'mobile') await page.setViewportSize(MOBILE);
        const url = shot.url();
        if (!`${new URL(page.url()).pathname}${new URL(page.url()).search}`.startsWith(url)) await spaGoto(page, url);
        await expect(shot.ready(page)).toBeVisible({ timeout: 60_000 });
        await settle(page);
        if (shot.element) {
          const el = shot.element(page);
          await el.scrollIntoViewIfNeeded();
          await expect(el).toHaveScreenshot(`${shot.name}--${look}.png`, { mask: [el.locator('.ant-card-body')] });
        } else {
          await page.evaluate(() => window.scrollTo(0, 0));
          await expect(page).toHaveScreenshot(`${shot.name}--${look}.png`, { mask: dataMasks(page) });
          // no horizontal page scroll (DESIGN_SYSTEM §4.2 "không cuộn ngang trang")
          const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
          expect(overflow, 'horizontal page scroll').toBeLessThanOrEqual(0);
        }
      });
    }
  }

  for (const shot of SHOTS.filter((x) => !x.element)) {
    for (const look of APPEARANCES) {
      test(`axe ${shot.name} — ${look}`, async () => {
        test.setTimeout(120_000); // navigates between the overview and the grade page (cold widgets on the shared stack)
        test.skip(!schoolId, 'no school of TH Demo A on the overview');
        const { page } = s;
        await page.setViewportSize(DESKTOP);
        await applyAppearance(page, look);
        const url = shot.url();
        // exact match: the overview URL is a prefix of the grade page URL
        if (`${new URL(page.url()).pathname}${new URL(page.url()).search}` !== url) await spaGoto(page, url);
        await expect(shot.ready(page)).toBeVisible({ timeout: 60_000 });
        await settle(page);
        // the period of the default range is not closed: the meta line shows "Tạm tính" (the pair axe must check)
        await expect(page.getByTestId('dashboard-meta').locator('.ant-tag-warning').first()).toBeVisible();
        const bars = await page.getByRole('progressbar').count();
        test.info().annotations.push({ type: 'progressbars', description: String(bars) });
        const v = await axeViolations(page);
        expect(v, describeFindings(v)).toEqual([]);
      });
    }
  }
});
