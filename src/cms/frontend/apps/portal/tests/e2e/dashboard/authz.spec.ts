// @F056 T042 — F056-AC-20 (TC-CMS-WS-01) on the REAL stack: users without any analytics permission (Safety Moderator
// `sm.antoan`, Content Admin `cm.noidung`) who open a School Learning Dashboard URL see the portal 403 page and the
// portal never calls a dashboard API (network log of the whole session, sign-in included).
// Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise (OQ F051-9). No secret is written anywhere (AGENTS R4).
import { expect, test } from '@playwright/test';
import { e2eReady } from '../support/auth';
import { spaGoto } from '../support/nav';
import { apiPath, DASHBOARD_API, recordNetwork, type NetworkLog } from '../support/network';
import { openSession, type Session } from '../support/session';

const FORBIDDEN = 'Không có quyền truy cập';
const DASHBOARD_URLS = [
  '/school-lms/school-dashboard',
  '/school-lms/school-dashboard/grades/3',
  '/school-lms/school-dashboard/students/00000000-0000-4000-8000-000000000001',
  '/school-lms/school-dashboard/reference',
];

for (const username of ['sm.antoan', 'cm.noidung']) {
  test.describe(`@F056 Dashboard — ${username} không có quyền analytics (F056-AC-20)`, () => {
    test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
    test.describe.configure({ mode: 'serial' });

    let s: Session;
    let net: NetworkLog;

    test.beforeAll(async ({ browser }) => {
      test.setTimeout(3 * 60_000);
      s = await openSession(browser, username, (c) => {
        net = recordNetwork(c);
      });
    });
    test.afterAll(async () => s?.close());

    for (const url of DASHBOARD_URLS) {
      test(`${username} opens ${url}: 403 page, no dashboard request`, async () => {
        const { page } = s;
        await spaGoto(page, url);
        await expect(page.locator('.ant-result-403')).toBeVisible({ timeout: 30_000 });
        await expect(page.getByText(FORBIDDEN)).toBeVisible();
        await expect(page).not.toHaveURL(/\/login/);
        await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => undefined);
        const calls = net.api().filter((r) => DASHBOARD_API.test(apiPath(r)));
        expect(calls.map((r) => `${r.method} ${apiPath(r)}`), 'dashboard API called').toEqual([]);
      });
    }
  });
}
