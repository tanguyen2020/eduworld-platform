// @F056 T043 — F056-AC-21 (TC-DASH-009, TC-TEN-001) on the REAL stack:
// - `ad.truong.b` (pgd-demo) opens drill-downs of TH Demo A (th-demo-a) by URL — the grade page of its school and the
//   learning profile of one of its students: the API answers 404 (or 403) and the portal shows "Không tìm thấy"
//   without telling whether the school/student exists; every dashboard call carries X-Tenant-Id = the session tenant
//   and never `tenant_ids`;
// - the client query cache is keyed by tenant: `qlcm.truong.a` (two tenants) reads the overview in TH Demo A, switches
//   to pgd-demo and the overview is read again with the new X-Tenant-Id (never served from the TH Demo A cache) and no
//   school of TH Demo A is shown.
// Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise (OQ F051-9). No secret is written anywhere (AGENTS R4).
import { expect, test, type Page, type Request } from '@playwright/test';
import { e2eReady } from '../support/auth';
import { openOptions, openSelect, spaGoto } from '../support/nav';
import { apiPath, DASHBOARD_API, recordNetwork, type NetworkLog } from '../support/network';
import { enterTenant, openSession } from '../support/session';

const BASE = '/school-lms/school-dashboard';
const SCHOOL_DASHBOARD = /\/api\/v1\/lms\/dashboards\/school(\?|$)/;
const SCHOOL_A = /Demo A/;

/** Dashboard calls of one session: tenant headers and `tenant_ids` use. */
function dashboardUse(net: NetworkLog) {
  const calls = net.api().filter((r) => DASHBOARD_API.test(apiPath(r)));
  return {
    calls,
    headers: [...new Set(calls.map((r) => r.tenantHeader))],
    withTenantIds: net.api().filter((r) => r.queryNames.some((q) => q.startsWith('tenant_ids'))).map((r) => apiPath(r)),
  };
}

/** Opens the overview and returns the getSchoolDashboard request it sends (school picked by the portal). */
async function overview(page: Page): Promise<Request> {
  const req = page.waitForRequest((r) => SCHOOL_DASHBOARD.test(r.url()) && r.method() === 'GET', { timeout: 60_000 });
  await spaGoto(page, BASE);
  return req;
}

test.describe('@F056 Dashboard — cô lập tenant (F056-AC-21)', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
  test.describe.configure({ mode: 'serial' });

  let schoolOfA: string | undefined;
  let studentOfA: string | undefined;
  let tenantOfA: string | null = null;

  test('ad.truong.a: overview and grade 3 of TH Demo A (ids for the isolation checks)', async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    let net!: NetworkLog;
    const s = await openSession(browser, 'ad.truong.a', (c) => {
      net = recordNetwork(c);
    });
    try {
      const { page } = s;
      const req = await overview(page);
      schoolOfA = new URL(req.url()).searchParams.get('school_id') ?? undefined;
      expect(schoolOfA, 'school_id of getSchoolDashboard').toMatch(/^[0-9a-f-]{36}$/);
      expect((await req.response())?.status()).toBe(200);
      const students = page.waitForResponse((r) => new RegExp(`/tenant/schools/${schoolOfA}/grades/3/students`).test(r.url()), { timeout: 60_000 });
      const grade = page.waitForResponse((r) => new RegExp(`/lms/schools/${schoolOfA}/grades/3/dashboard`).test(r.url()), { timeout: 60_000 });
      await spaGoto(page, `${BASE}/grades/3?school=${schoolOfA}`);
      expect((await grade).status(), 'getGradeDashboard of the own school').toBe(200);
      const body = (await (await students).json()) as { items?: { student_id: string }[]; data?: { student_id: string }[] };
      studentOfA = (body.items ?? body.data ?? [])[0]?.student_id;
      const use = dashboardUse(net);
      expect(use.headers, 'one X-Tenant-Id for every dashboard call').toHaveLength(1);
      tenantOfA = use.headers[0]!;
      expect(tenantOfA).toMatch(/^[0-9a-f-]{36}$/);
      expect(use.withTenantIds).toEqual([]);
    } finally {
      await s.close();
    }
  });

  test('ad.truong.b: grade page and student profile of TH Demo A show "Không tìm thấy"', async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    test.skip(!schoolOfA, 'no school of TH Demo A captured by the ad.truong.a test');
    let net!: NetworkLog;
    const s = await openSession(browser, 'ad.truong.b', (c) => {
      net = recordNetwork(c);
    });
    try {
      const { page } = s;
      const own = await overview(page);
      expect(new URL(own.url()).searchParams.get('school_id'), 'ad.truong.b works on its own school').not.toBe(schoolOfA);
      // grade page of the TH Demo A school
      const grade = page.waitForResponse((r) => new RegExp(`/lms/schools/${schoolOfA}/grades/3/dashboard`).test(r.url()), { timeout: 60_000 });
      await spaGoto(page, `${BASE}/grades/3?school=${schoolOfA}`);
      expect([403, 404], 'getGradeDashboard of another tenant').toContain((await grade).status());
      await expect(page.getByTestId('portal-content').getByText('Không tìm thấy', { exact: true }).first()).toBeVisible({ timeout: 30_000 });
      await expect(page.getByTestId('portal-content').getByText(SCHOOL_A)).toHaveCount(0);
      // learning profile of a TH Demo A student
      if (studentOfA) {
        const profile = page.waitForResponse((r) => new RegExp(`/lms/students/${studentOfA}/analytics`).test(r.url()), { timeout: 60_000 });
        await spaGoto(page, `${BASE}/students/${studentOfA}`);
        expect([403, 404], 'getStudentLearningProfile of another tenant').toContain((await profile).status());
        await expect(page.getByTestId('portal-content').getByText('Không tìm thấy', { exact: true }).first()).toBeVisible({ timeout: 30_000 });
      } else {
        test.info().annotations.push({ type: 'skipped part', description: 'no student in grade 3 of TH Demo A: profile check not run' });
      }
      const use = dashboardUse(net);
      expect(use.headers, 'one X-Tenant-Id for every dashboard call').toHaveLength(1);
      expect(use.headers[0]).toMatch(/^[0-9a-f-]{36}$/);
      expect(use.headers[0]).not.toBe(tenantOfA);
      expect(use.withTenantIds, 'tenant user sent tenant_ids').toEqual([]);
    } finally {
      await s.close();
    }
  });

  test('qlcm.truong.a: switching tenant reads the overview again with the new tenant (cache keyed by tenant)', async ({ browser }) => {
    test.setTimeout(4 * 60_000);
    let net!: NetworkLog;
    const s = await openSession(browser, 'qlcm.truong.a', (c) => {
      net = recordNetwork(c);
    });
    try {
      const { page } = s;
      await enterTenant(page, /TH Demo A/);
      const inA = await overview(page);
      expect((await inA.response())?.status()).toBe(200);
      const headerA = (await inA.allHeaders())['x-tenant-id'];
      expect(headerA).toMatch(/^[0-9a-f-]{36}$/);
      // switch to pgd-demo with the shell switcher, then open the same overview URL
      await openSelect(page, 'Đơn vị làm việc');
      await openOptions(page).filter({ hasText: /pgd-demo|PGD/i }).first().click();
      const confirm = page.getByRole('button', { name: 'Đổi đơn vị' });
      if (await confirm.isVisible({ timeout: 2_000 }).catch(() => false)) await confirm.click();
      await page.getByRole('navigation', { name: 'Menu điều hướng' }).waitFor({ timeout: 30_000 });
      net.clear();
      const inB = await overview(page);
      const headerB = (await inB.allHeaders())['x-tenant-id'];
      expect(headerB).toMatch(/^[0-9a-f-]{36}$/);
      expect(headerB, 'overview read again with the new tenant').not.toBe(headerA);
      expect(new URL(inB.url()).searchParams.get('school_id')).not.toBe(new URL(inA.url()).searchParams.get('school_id'));
      expect((await inB.response())?.status()).toBe(200);
      await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => undefined);
      await expect(page.getByTestId('portal-content').getByText(SCHOOL_A)).toHaveCount(0);
      const use = dashboardUse(net);
      expect(use.headers).toEqual([headerB]);
      expect(use.withTenantIds).toEqual([]);
    } finally {
      await s.close();
    }
  });
});
