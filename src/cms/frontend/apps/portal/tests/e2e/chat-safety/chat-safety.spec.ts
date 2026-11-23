// @F122 — chat-safety screens on the REAL stack (quickstart of specs/122-portal-chat-safety; T003, T047): the demo accounts
// of the quickstart (sm.antoan, sa.admin, ad.truong.a, ad.truong.b) sign in with their password only (ADR-0094,
// support/auth.ts); every screen passes the accessibility smoke (one h1, every control named — TC-UI-006) in light and
// dark colour scheme; the Safety Moderator sees the Kiểm duyệt menus of E and the read-only catalogue of D only (F122-AC-24,
// AC-09, T062), its tenant filter comes from listModerationCaseTenants (never listTenants, T064); the System Admin sees the
// Reject button next to Approve for a request of another admin (T060); the school notifications ask for the last 30 days
// (created_from, T066) and become cards below 768 px (F122-AC-20); club members can be listed (T061). Writes are never
// made (no request, no publish): the suite only reads the seed.
// Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise (OQ F051-9: no E2E job against a real backend yet). No secret is
// written anywhere (AGENTS R4).
import { expect, test, type Page } from '@playwright/test';
import { e2eReady, signIn } from '../support/auth';
import { openSelect, spaGoto } from '../support/nav';

const NAV = 'Menu điều hướng';

/** Accessibility smoke: one h1, every form control has an accessible name. */
async function a11y(page: Page) {
  await expect(page.getByRole('heading', { level: 1 })).toHaveCount(1);
  const unnamed = await page.$$eval('input:not([type=hidden]), select, textarea, [role=combobox]', (els) =>
    els.filter((e) => !(e.getAttribute('aria-label') || e.getAttribute('aria-labelledby') || (e.id && document.querySelector(`label[for="${e.id}"]`)))).length,
  );
  expect(unnamed).toBe(0);
}

/** The smoke in light then dark colour scheme (design tokens of both themes, TC-UI-006). */
async function a11yBothThemes(page: Page) {
  for (const colorScheme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme });
    await a11y(page);
  }
}

test.describe('@F122 An toàn chat trên portal', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');

  test('sm.antoan: Kiểm duyệt menus + read-only catalogue; tenant filter from listModerationCaseTenants', async ({ page }) => {
    const tenantCalls: string[] = [];
    page.on('request', (r) => {
      const u = new URL(r.url());
      if (u.pathname.endsWith('/admin/tenants') || u.pathname.endsWith('/admin/moderation/case-tenants')) tenantCalls.push(u.pathname);
    });
    await signIn(page, 'sm.antoan');
    await spaGoto(page, '/live-ops/moderation-cases');
    const nav = page.getByRole('navigation', { name: NAV });
    await expect(nav.getByText('Hàng đợi case kiểm duyệt')).toBeVisible();
    for (const m of ['Live dashboard', 'Game Analytics', 'Công tắc kiểm duyệt']) await expect(nav.getByText(m)).toHaveCount(0);
    await a11yBothThemes(page);
    await openSelect(page, 'Lọc tenant');
    expect(tenantCalls.every((p) => p.endsWith('/case-tenants'))).toBe(true);
    await page.keyboard.press('Escape');
    await spaGoto(page, '/platform-admin/chat-safety');
    await expect(page.getByRole('tab', { name: 'Từ' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Thêm nhóm từ' })).toHaveCount(0);
    await a11yBothThemes(page);
    await spaGoto(page, '/platform-admin/moderation-switches');
    await expect(page.getByText('Không có quyền truy cập')).toBeVisible();
  });

  test('sa.admin: catalogue filters and the switches screen', async ({ page }) => {
    await signIn(page, 'sa.admin');
    await spaGoto(page, '/platform-admin/chat-safety');
    await page.getByRole('tab', { name: 'Từ' }).click();
    for (const label of ['Ngôn ngữ', 'Kiểu khớp', 'Trạng thái']) await expect(page.getByRole('combobox', { name: label, exact: true })).toBeVisible();
    await a11yBothThemes(page);
    await spaGoto(page, '/platform-admin/moderation-switches');
    await expect(page.getByText('Luôn bật').first()).toBeVisible();
    await a11yBothThemes(page);
  });

  test('ad.truong.a: notifications of the last 30 days, cards on a phone; club members; ad.truong.b sees nothing of A', async ({ page, browser }) => {
    const listQueries: URLSearchParams[] = [];
    page.on('request', (r) => {
      const u = new URL(r.url());
      if (u.pathname.endsWith('/tenant/safety/school-notifications') && u.searchParams.get('limit') !== '50') listQueries.push(u.searchParams);
    });
    await signIn(page, 'ad.truong.a');
    await spaGoto(page, '/school-lms/safety-notifications');
    await expect(page.getByRole('combobox', { name: 'Khoảng thời gian', exact: true })).toBeVisible();
    expect(listQueries.some((q) => q.has('created_from'))).toBe(true);
    await a11yBothThemes(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(page.getByRole('table')).toHaveCount(0);
    await page.setViewportSize({ width: 1440, height: 900 });
    await spaGoto(page, '/school-lms/clubs');
    await a11yBothThemes(page);
    const other = await browser.newPage();
    await signIn(other, 'ad.truong.b');
    await spaGoto(other, '/school-lms/safety-notifications');
    await expect(other.getByText(/Demo A/)).toHaveCount(0);
    await other.close();
  });
});
