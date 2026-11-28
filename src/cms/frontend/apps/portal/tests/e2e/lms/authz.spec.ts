// @F055 T039 — F055-AC-20 (TC-CMS-WS-01) on the REAL stack: a user without any `lms.*` permission (Safety Moderator
// `sm.antoan`) who opens an LMS URL sees the portal 403 page, and the workspace C chunk is never loaded (network log of
// the whole session, sign-in included) nor any LMS API called.
// Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise (OQ F051-9). No secret is written anywhere (AGENTS R4).
import { expect, test } from '@playwright/test';
import { e2eReady } from '../support/auth';
import { spaGoto } from '../support/nav';
import { recordNetwork, SCHOOL_LMS_CHUNK, apiPath, type NetworkLog } from '../support/network';
import { openSession, type Session } from '../support/session';

const FORBIDDEN = 'Không có quyền truy cập';
const LMS_URLS = ['/school-lms/assignments', '/school-lms/assignments/new', '/school-lms/assignments/00000000-0000-4000-8000-000000000001'];

test.describe('@F055 LMS — không có quyền lms.* (F055-AC-20)', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
  test.describe.configure({ mode: 'serial' });

  let s: Session;
  let net: NetworkLog;

  test.beforeAll(async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    s = await openSession(browser, 'sm.antoan', (c) => {
      net = recordNetwork(c);
    });
  });
  test.afterAll(async () => s?.close());

  for (const url of LMS_URLS) {
    test(`sm.antoan opens ${url}: 403 page, no workspace C chunk, no LMS call`, async () => {
      const { page } = s;
      await spaGoto(page, url);
      await expect(page.locator('.ant-result-403')).toBeVisible({ timeout: 30_000 });
      await expect(page.getByText(FORBIDDEN)).toBeVisible();
      await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => undefined);
      const chunk = net.all().filter((r) => SCHOOL_LMS_CHUNK.test(r.path));
      expect(chunk.map((r) => r.path), 'workspace C chunk requested').toEqual([]);
      const lms = net.api().filter((r) => /^\/(lms|exams)\//.test(apiPath(r)));
      expect(lms.map((r) => `${r.method} ${apiPath(r)}`), 'LMS API called').toEqual([]);
    });
  }
});
