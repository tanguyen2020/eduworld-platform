// @F055 T040 — F055-AC-21 (TC-CMS-AUTHZ-08, TC-TEN-011) on the REAL stack:
// - `ad.truong.b` (pgd-demo) opens an assignment of TH Demo A (th-demo-a, seeded DEV assignment of `ad.truong.a`) by
//   URL: the API answers 404 (or 403) and the portal shows "Không tìm thấy" without any data of the assignment;
// - every LMS request of a tenant user carries X-Tenant-Id = the session tenant and never `tenant_ids`;
// - a stale X-Tenant-Id (rewritten in flight to a tenant the user has no membership in) gets the real 403
//   TENANT_MISMATCH of the backend and the portal clears its cache and goes back to the tenant picker
//   (`qlcm.truong.a`, two tenants).
// Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise (OQ F051-9). No secret is written anywhere (AGENTS R4).
import { randomUUID } from 'node:crypto';
import { expect, test, type Page, type Response } from '@playwright/test';
import { e2eReady } from '../support/auth';
import { spaGoto } from '../support/nav';
import { apiPath, recordNetwork, SCHOOL_LMS_CHUNK, type NetworkLog } from '../support/network';
import { enterTenant, openSession, TENANT_PICKER } from '../support/session';

const LIST = /\/api\/v1\/lms\/assignments(\?|$)/;
const ONE = (id: string) => new RegExp(`/api/v1/lms/assignments/${id}$`);

interface AssignmentItem {
  assignment_id?: string;
  id?: string;
  title?: string;
}

/** Tenant header and `tenant_ids` use of the LMS calls of one session. */
function tenantUse(net: NetworkLog) {
  const lms = net.api().filter((r) => /^\/(lms|exams)\//.test(apiPath(r)));
  return {
    lms,
    headers: [...new Set(lms.map((r) => r.tenantHeader))],
    withTenantIds: net.api().filter((r) => r.queryNames.some((q) => q.startsWith('tenant_ids'))).map((r) => apiPath(r)),
  };
}

async function listOnce(page: Page): Promise<Response> {
  const res = page.waitForResponse((r) => LIST.test(r.url()) && r.request().method() === 'GET', { timeout: 60_000 });
  await spaGoto(page, '/school-lms/assignments');
  return res;
}

test.describe('@F055 LMS — cô lập tenant (F055-AC-21)', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
  test.describe.configure({ mode: 'serial' });

  /** Assignment of TH Demo A seen by ad.truong.a (seed-content DEV assignment). */
  let assignmentOfA: { id: string; title: string } | undefined;
  let tenantOfA: string | null = null;

  test('ad.truong.a: lists the assignments of TH Demo A with its own X-Tenant-Id (workspace C chunk loaded)', async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    let net!: NetworkLog;
    const s = await openSession(browser, 'ad.truong.a', (c) => {
      net = recordNetwork(c);
    });
    try {
      const r = await listOnce(s.page);
      expect(r.status(), 'listAssignments').toBe(200);
      const body = (await r.json()) as { data?: AssignmentItem[] };
      const first = (body.data ?? []).find((a) => a.assignment_id ?? a.id);
      if (first) assignmentOfA = { id: (first.assignment_id ?? first.id)!, title: first.title ?? '' };
      const use = tenantUse(net);
      expect(use.lms.length).toBeGreaterThan(0);
      expect(use.headers, 'one X-Tenant-Id for every LMS call').toHaveLength(1);
      tenantOfA = use.headers[0]!;
      expect(tenantOfA).toMatch(/^[0-9a-f-]{36}$/);
      expect(use.withTenantIds, 'tenant user sent tenant_ids').toEqual([]);
      // control of the chunk pattern used by authz.spec.ts: a user of workspace C does load its chunk
      expect(net.all().some((q) => SCHOOL_LMS_CHUNK.test(q.path))).toBe(true);
    } finally {
      await s.close();
    }
  });

  test('ad.truong.b: an assignment of TH Demo A opened by URL shows "Không tìm thấy"', async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    test.skip(!assignmentOfA, 'missing seed data: ad.truong.a lists no assignment in TH Demo A (seed-content DEV assignment)');
    let net!: NetworkLog;
    const s = await openSession(browser, 'ad.truong.b', (c) => {
      net = recordNetwork(c);
    });
    try {
      const { page } = s;
      // the own list first: the session works in pgd-demo, not th-demo-a
      expect((await listOnce(page)).status()).toBe(200);
      const one = page.waitForResponse((r) => ONE(assignmentOfA!.id).test(r.url()) && r.request().method() === 'GET', { timeout: 60_000 });
      await spaGoto(page, `/school-lms/assignments/${assignmentOfA!.id}`);
      expect([403, 404], 'getAssignment of another tenant').toContain((await one).status());
      await expect(page.getByTestId('school-lms-not-found')).toBeVisible({ timeout: 30_000 });
      await expect(page.getByTestId('school-lms-not-found')).toContainText('Không tìm thấy');
      if (assignmentOfA!.title) await expect(page.getByText(assignmentOfA!.title, { exact: true })).toHaveCount(0);
      const use = tenantUse(net);
      expect(use.headers, 'one X-Tenant-Id for every LMS call').toHaveLength(1);
      expect(use.headers[0]).toMatch(/^[0-9a-f-]{36}$/);
      expect(use.headers[0], 'ad.truong.b works in its own tenant').not.toBe(tenantOfA);
      expect(use.withTenantIds, 'tenant user sent tenant_ids').toEqual([]);
    } finally {
      await s.close();
    }
  });

  test('qlcm.truong.a: a stale X-Tenant-Id gets TENANT_MISMATCH, the portal goes back to the tenant picker', async ({ browser }) => {
    test.setTimeout(4 * 60_000);
    let net!: NetworkLog;
    const s = await openSession(browser, 'qlcm.truong.a', (c) => {
      net = recordNetwork(c);
    });
    try {
      const { page } = s;
      await enterTenant(page, /TH Demo A/);
      expect((await listOnce(page)).status()).toBe(200);
      const before = tenantUse(net).headers;
      expect(before).toHaveLength(1);
      // the next LMS read leaves with the header of a tenant the user is no member of (stale tab, membership revoked)
      let rewritten = false;
      await page.route(/\/api\/v1\/lms\//, async (route) => {
        const headers = route.request().headers();
        if (rewritten || !headers['x-tenant-id']) return route.continue();
        rewritten = true;
        return route.continue({ headers: { ...headers, 'x-tenant-id': randomUUID() } });
      });
      const mismatch = page.waitForResponse((r) => /\/api\/v1\/lms\//.test(r.url()) && r.status() === 403, { timeout: 60_000 });
      // a detail URL is never in the query cache: it always sends a fresh getAssignment
      await spaGoto(page, `/school-lms/assignments/${randomUUID()}`);
      const r = await mismatch;
      expect(((await r.json()) as { code?: string }).code).toBe('TENANT_MISMATCH');
      await page.unroute(/\/api\/v1\/lms\//);
      await expect(page.getByRole('heading', { name: TENANT_PICKER })).toBeVisible({ timeout: 30_000 });
      // cache cleared: entering again reads the list anew with the chosen tenant
      net.clear();
      await page.getByRole('radio', { name: /TH Demo A/ }).click();
      await page.getByRole('button', { name: 'Vào' }).click();
      await page.getByRole('navigation', { name: 'Menu điều hướng' }).waitFor({ timeout: 30_000 });
      expect((await listOnce(page)).status()).toBe(200);
      const after = tenantUse(net);
      expect(after.headers).toEqual(before);
      expect(after.withTenantIds).toEqual([]);
    } finally {
      await s.close();
    }
  });
});
