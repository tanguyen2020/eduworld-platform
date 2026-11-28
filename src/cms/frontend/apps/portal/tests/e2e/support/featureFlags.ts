// Feature flag helpers of the Playwright E2E suites (real stack, OQ F051-9; F059 screen "Feature flag · Bật/tắt menu &
// action", ADR-0090): read and change the tenant override (or the platform default) of a menu/action flag as a System
// Admin, and hold the permissions of a tenant user so that the stale portal really calls the API and gets
// `403 FEATURE_DISABLED` (F051-AC-37: notice + permissions reload). Suites always put the flag back to the value they
// read first (try/finally).
import { expect, type BrowserContext, type Locator, type Page, type Route } from '@playwright/test';
import { openOptions, openSelect, spaGoto } from './nav';

export type FlagKind = 'Menu' | 'Thao tác' | 'Tính năng';
/** State of a flag for a tenant: its own override (on/off) or none (inherit the platform default). */
export type FlagState = 'on' | 'off' | 'inherit';

export interface FlagTarget {
  key: string;
  kind: FlagKind;
  /** Tenant of the override (option of "Xem ghi đè của tenant"); the platform default otherwise. */
  tenant?: RegExp;
}

const STATE_LABEL: Record<FlagState, string> = { on: 'Bật', off: 'Tắt', inherit: 'Theo mặc định platform (gỡ ghi đè)' };
const FLAGS_PATH = '/platform-admin/feature-toggles';

/** Opens the flag list filtered by kind (and tenant) and loads pages until the row of the key is there. */
async function flagRow(page: Page, f: FlagTarget): Promise<Locator> {
  await spaGoto(page, FLAGS_PATH);
  await openSelect(page, 'Loại');
  await openOptions(page).filter({ hasText: new RegExp(`^${f.kind}$`) }).first().click();
  if (f.tenant) {
    // the tenant select is narrow (its input may sit outside the viewport): open it from its container
    await page.getByRole('combobox', { name: 'Xem ghi đè của tenant', exact: true }).locator('xpath=ancestor::div[contains(@class,"ant-select")][1]').click();
    await openOptions(page).filter({ hasText: f.tenant }).first().click();
    await expect(page.getByRole('columnheader', { name: 'Ghi đè của tenant' })).toBeVisible({ timeout: 30_000 });
  }
  const row = page.locator('tr').filter({ has: page.getByText(f.key, { exact: true }) });
  const more = page.getByRole('button', { name: 'Tải thêm' });
  await expect(row.or(more).first()).toBeVisible({ timeout: 30_000 });
  while (!(await row.isVisible()) && (await more.isVisible())) {
    await more.click();
    await expect(row.or(more).first()).toBeVisible({ timeout: 30_000 });
  }
  await expect(row).toBeVisible();
  return row;
}

/** Current state of the flag: the tenant override (inherit when none) or the platform default. */
export async function readFlag(page: Page, f: FlagTarget): Promise<FlagState> {
  const row = await flagRow(page, f);
  // columns: key, kind, description, platform default, [tenant override,] effective, …
  const text = ((await row.locator('td').nth(f.tenant ? 4 : 3).textContent()) ?? '').trim();
  if (text === 'Không ghi đè') return 'inherit';
  return text === 'Tắt' ? 'off' : 'on';
}

/** Saves the flag in the editor (scope tenant when `f.tenant`, platform default otherwise) with a reason. */
export async function setFlag(page: Page, f: FlagTarget, state: FlagState, reason: string): Promise<void> {
  const row = await flagRow(page, f);
  await row.getByRole('button', { name: 'Sửa' }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  await dialog.getByLabel(STATE_LABEL[state], { exact: true }).check();
  await dialog.getByLabel('Lý do').fill(reason);
  const saved = page.waitForResponse((r) => /\/feature-flags\//.test(r.url()) && r.request().method() === 'PUT', { timeout: 30_000 });
  await dialog.getByRole('button', { name: 'Lưu' }).click();
  expect((await saved).status(), `PUT feature flag ${f.key}`).toBeLessThan(300);
  await expect(dialog).toBeHidden();
}

export interface PermissionHold {
  /** True once the held API answered 403 FEATURE_DISABLED (the permissions are released at that moment). */
  disabledSeen(): boolean;
  /** getMyPermissions calls that went to the backend after the release (the reload of F051-AC-37). */
  reloadsAfterRelease(): number;
  /** `feature_key` of the 403 Problem. */
  featureKey(): string | undefined;
  /** Calls of the held API seen by the route (diagnostics of a run). */
  apiCalls(): string[];
}

/**
 * Serves the portal of `page` the permissions it read first (the state before a flag change) until `api` answers
 * `403 FEATURE_DISABLED`; from that response on, getMyPermissions goes to the backend again. This makes the stale
 * portal call the API (instead of hiding the menu on the periodic permissions refresh) and lets the suite check that
 * the 403 triggers a permissions reload.
 */
export async function holdPermissions(page: Page | BrowserContext, api: RegExp): Promise<PermissionHold> {
  let cached: { status: number; headers: Record<string, string>; body: Buffer } | undefined;
  let released = false;
  let reloads = 0;
  let key: string | undefined;
  await page.route(/\/api\/v1\/me\/permissions(\?|$)/, async (route: Route) => {
    if (released) {
      reloads++;
      return route.continue();
    }
    if (!cached) {
      const r = await route.fetch();
      cached = { status: r.status(), headers: r.headers(), body: await r.body() };
    }
    return route.fulfill(cached);
  });
  const calls: string[] = [];
  await page.route(api, async (route: Route) => {
    const r = await route.fetch();
    const body = await r.body();
    calls.push(`${route.request().method()} ${r.status()}`);
    if (r.status() === 403) {
      try {
        const p = JSON.parse(body.toString('utf8')) as { code?: string; feature_key?: string };
        if (p.code === 'FEATURE_DISABLED') {
          key = p.feature_key;
          released = true;
        }
      } catch {
        // not a Problem body
      }
    }
    return route.fulfill({ response: r, body });
  });
  return { disabledSeen: () => released, reloadsAfterRelease: () => reloads, featureKey: () => key, apiCalls: () => [...calls] };
}

export interface MenuToggleCase {
  /** Catalog menu key turned off for the tenant. */
  menuKey: string;
  /** Path of the menu (sidebar link href). */
  path: string;
  /** API the screen of the menu calls on load. */
  api: RegExp;
  /** Tenant user who works in the tenant. */
  user: string;
  tenant: RegExp;
  reason: string;
}

/**
 * F051-AC-37 / TC-CMS-MENU-02 on the real stack: `sa.admin` turns the menu off for the tenant (tenant override), the
 * stale portal of the tenant user (permissions held, `holdPermissions`) calls the API of the screen → `403
 * FEATURE_DISABLED` with the menu as `feature_key` → notice "Tính năng đang tắt" + getMyPermissions reloaded → the
 * sidebar link is gone and the URL shows "Tính năng đang tạm tắt". The override found first is always put back.
 */
export async function runMenuToggle(open: (user: string, onContext?: (c: BrowserContext) => Promise<void>) => Promise<{ page: Page; close(): Promise<void> }>, c: MenuToggleCase): Promise<void> {
  const flag: FlagTarget = { key: c.menuKey, kind: 'Menu', tenant: c.tenant };
  let hold: PermissionHold | undefined;
  const user = await open(c.user, async (ctx) => {
    hold = await holdPermissions(ctx, c.api);
  });
  const admin = await open('sa.admin');
  const link = user.page.locator(`nav[aria-label="Menu điều hướng"] a[href="${c.path}"]`);
  let original: FlagState | undefined;
  try {
    // before: the menu is there and its screen loads
    const first = user.page.waitForResponse((r) => c.api.test(r.url()) && r.request().method() === 'GET', { timeout: 60_000 });
    await spaGoto(user.page, c.path);
    expect((await first).status(), `${c.menuKey} on: the screen loads`).toBe(200);
    await expect(link.first()).toBeVisible();
    original = await readFlag(admin.page, flag);
    await setFlag(admin.page, flag, 'off', c.reason);
    // the backend applies the change within tens of seconds: reload the stale portal until its API call is refused
    await expect
      .poll(
        async () => {
          if (!hold!.disabledSeen()) {
            await user.page.reload();
            await user.page.getByRole('navigation', { name: 'Menu điều hướng' }).first().waitFor({ timeout: 30_000 }).catch(() => undefined);
            await user.page.waitForResponse((r) => c.api.test(r.url()), { timeout: 20_000 }).catch(() => undefined);
          }
          return hold!.disabledSeen();
        },
        { timeout: 4 * 60_000, intervals: [15_000] },
      )
      .toBe(true);
    expect(hold!.featureKey(), 'feature_key of the Problem').toBe(c.menuKey);
    await expect(user.page.getByText('Tính năng đang tắt').first()).toBeVisible({ timeout: 30_000 });
    await expect.poll(() => hold!.reloadsAfterRelease(), { timeout: 60_000, message: 'getMyPermissions reloaded after the 403' }).toBeGreaterThan(0);
    await expect(link).toHaveCount(0, { timeout: 60_000 });
    await expect(user.page.getByTestId('feature-disabled')).toBeVisible({ timeout: 30_000 });
  } finally {
    if (original) await setFlag(admin.page, flag, original, `${c.reason} — khôi phục sau kiểm thử`);
    await user.close();
    await admin.close();
  }
}
