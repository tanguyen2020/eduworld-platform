// @F058 T051 — F058-AC-27 (TC-CMS-TEN-03, TC-CMS-ASG-02) on the REAL stack:
// - `qlcm.truong.a` (TH Demo A): platform activities assigned to th-demo-a carry "Từ EduWorld" (`assigned`) in the
//   list; opened, the editor is read only (badge, inputs disabled, no "Gửi duyệt"); the tenant's own activities have
//   no badge; every Content Studio call carries X-Tenant-Id = the session tenant, never `tenant_ids`;
// - `sa.admin` creating an activity must pick the owner (a target tenant or Platform): "Lưu" without one is refused
//   in the form ("Chọn một tenant đích.") and nothing is sent (no createCmsActivity);
// - `ad.truong.b` (pgd-demo) never sees an activity of th-demo-a: not in its list, and its URL shows "Không tìm thấy"
//   (getCmsActivity 404) without any of its data.
// Read only: nothing is created. Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise (OQ F051-9). No secret is
// written anywhere (AGENTS R4).
import { expect, test } from '@playwright/test';
import { e2eReady } from '../support/auth';
import { spaGoto } from '../support/nav';
import { apiPath, recordNetwork } from '../support/network';
import { enterTenant, openSession } from '../support/session';

const LIST = /\/api\/v1\/cms\/learning\/activities(\?|$)/;

interface ActivityRow {
  activity_id: string;
  title?: { vi?: string };
  assigned?: boolean;
  tenant_id?: string | null;
}

test.describe('@F058 Activity Builder — cô lập tenant và quyền (F058-AC-27)', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
  test.describe.configure({ mode: 'serial' });

  /** An activity owned by th-demo-a (not assigned), found by qlcm.truong.a, for the isolation check of ad.truong.b. */
  let ownOfA: ActivityRow | undefined;

  test('qlcm.truong.a: assigned platform activities are "Từ EduWorld" and read only', async ({ browser }) => {
    test.setTimeout(4 * 60_000);
    let tenantOfSession: string | null = null;
    const s = await openSession(browser, 'qlcm.truong.a');
    const net = recordNetwork(s.context);
    try {
      await enterTenant(s.page, /TH Demo A/);
      const list = s.page.waitForResponse((r) => LIST.test(r.url()) && r.request().method() === 'GET', { timeout: 60_000 });
      await spaGoto(s.page, '/content-studio/activities');
      const res = await list;
      tenantOfSession = res.request().headers()['x-tenant-id'] ?? null;
      expect(tenantOfSession).toMatch(/^[0-9a-f-]{36}$/);
      const rows = ((await res.json()) as { data: ActivityRow[] }).data;
      const assigned = rows.find((a) => a.assigned);
      ownOfA = rows.find((a) => !a.assigned && a.tenant_id === tenantOfSession);
      expect(assigned, 'missing seed data: platform activity assigned to th-demo-a (make seed-content)').toBeTruthy();
      expect(ownOfA, 'missing seed data: tenant activity of th-demo-a (make seed-media)').toBeTruthy();
      const table = s.page.getByTestId('activity-list');
      await expect(table.locator('tr').filter({ has: s.page.locator(`a[href$="/${assigned!.activity_id}"]`) }).getByTestId('assigned-badge')).toHaveText('Từ EduWorld');
      await expect(table.locator('tr').filter({ has: s.page.locator(`a[href$="/${ownOfA!.activity_id}"]`) }).getByTestId('assigned-badge')).toHaveCount(0);
      // the assigned activity opens read only
      await table.locator(`a[href$="/${assigned!.activity_id}"]`).click();
      await expect(s.page.getByTestId('assigned-badge').first()).toBeVisible({ timeout: 60_000 });
      await expect(s.page.getByRole('textbox', { name: 'Tiêu đề (Tiếng Việt)' })).toBeDisabled();
      const bar = s.page.getByRole('region', { name: 'Trạng thái và thao tác' });
      await expect(bar.getByRole('button', { name: 'Gửi duyệt' })).toHaveCount(0);
      // one tenant per session: X-Tenant-Id everywhere, never tenant_ids (F051-AC-26)
      const cms = net.api().filter((r) => /^\/cms\//.test(apiPath(r)));
      expect(cms.length).toBeGreaterThan(0);
      expect(cms.filter((r) => r.tenantHeader !== tenantOfSession).map((r) => `${r.method} ${apiPath(r)}`)).toEqual([]);
      expect(cms.filter((r) => r.queryNames.some((q) => q.startsWith('tenant_ids')))).toEqual([]);
    } finally {
      await s.close();
    }
  });

  test('sa.admin: a new activity needs a target tenant or Platform before anything is sent', async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    const s = await openSession(browser, 'sa.admin');
    try {
      const creates: string[] = [];
      s.page.on('request', (r) => {
        if (r.method() === 'POST' && LIST.test(r.url())) creates.push(r.url());
      });
      await spaGoto(s.page, '/content-studio/activities/new?type=quiz');
      const owner = s.page.getByRole('combobox', { name: /^(\* )?Tenant$/ });
      await expect(owner).toBeVisible({ timeout: 60_000 });
      await s.page.getByRole('textbox', { name: 'Tiêu đề (Tiếng Việt)' }).fill('[E2E] Không được gửi khi chưa chọn tenant');
      await s.page.getByRole('region', { name: 'Trạng thái và thao tác' }).getByRole('button', { name: 'Lưu' }).click();
      await expect(s.page.getByText('Chọn một tenant đích.')).toBeVisible();
      await s.page.waitForTimeout(2000);
      expect(creates, 'no createCmsActivity without an owner').toEqual([]);
      // the choices: Platform and the tenants (one target, never "all tenants")
      await owner.locator('xpath=ancestor::div[contains(@class,"ant-select")][1]').click();
      const options = s.page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').last().locator('.ant-select-item-option');
      await expect(options.filter({ hasText: 'Platform (dữ liệu dùng chung)' })).toHaveCount(1);
      await expect(options.filter({ hasText: /TH Demo A/ }).first()).toBeVisible();
      expect(await options.filter({ hasText: /Tất cả/ }).count()).toBe(0);
      await s.page.keyboard.press('Escape');
    } finally {
      await s.close();
    }
  });

  test('ad.truong.b: activities of th-demo-a are not listed and their URL shows "Không tìm thấy"', async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    test.skip(!ownOfA, 'no activity of th-demo-a found');
    const s = await openSession(browser, 'ad.truong.b');
    try {
      const list = s.page.waitForResponse((r) => LIST.test(r.url()) && r.request().method() === 'GET', { timeout: 60_000 });
      await spaGoto(s.page, '/content-studio/activities');
      const rows = ((await (await list).json()) as { data: ActivityRow[] }).data;
      expect(rows.map((a) => a.activity_id)).not.toContain(ownOfA!.activity_id);
      const one = s.page.waitForResponse((r) => r.url().endsWith(`/cms/learning/activities/${ownOfA!.activity_id}`) && r.request().method() === 'GET', { timeout: 60_000 });
      await spaGoto(s.page, `/content-studio/activities/${ownOfA!.activity_id}`);
      expect([403, 404]).toContain((await one).status());
      await expect(s.page.getByText('Không tìm thấy', { exact: false }).first()).toBeVisible({ timeout: 30_000 });
      if (ownOfA!.title?.vi) await expect(s.page.getByText(ownOfA!.title.vi)).toHaveCount(0);
    } finally {
      await s.close();
    }
  });
});
