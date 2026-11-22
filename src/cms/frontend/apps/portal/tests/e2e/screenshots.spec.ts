// @screenshots — full-page PNG of every portal screen on the REAL stack (owner request 2026-11-21, review of the
// "Sân trường" sign-in and "Sổ tay giáo viên" working screens). Public pages (login desktop/phone, light/dark,
// forgot password, student refused) and, per demo role, every menu the generated catalog (ADR-0090) exposes to that
// role — read from the sidebar the portal renders after `getMyPermissions`, so nothing is listed by hand. Each shot
// gets a status: OK, empty (no data), placeholder (screen of a later feature) or error (alerts, results, the VPN
// notice, failed API calls, page errors), written to <EW_E2E_SCREENSHOT_DIR>/index.md. Students by school grade
// (ADR-0095): no class screen exists any more. Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD, EW_E2E_SCREENSHOT_DIR (a
// folder outside the repo); every account signs in with its password only (ADR-0094).
// No secret is written to a file name, a note or the index (AGENTS R4).
import { expect, test, type Page } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { WORKSPACE_CATALOG } from '../../src/generated/menu-catalog';
import { e2eReady, signIn } from './support/auth';
import { openOptions, openSelect, spaGoto } from './support/nav';

const OUT = process.env.EW_E2E_SCREENSHOT_DIR ?? '';
const DESKTOP = { width: 1440, height: 900 };
const PHONE = { width: 390, height: 844 };
const NAV = 'Menu điều hướng';
/** Text of MenuPlaceholder (shell.menuPlaceholder): a menu whose screen comes with a later feature. */
const PLACEHOLDER = 'Màn hình này được bổ sung ở tính năng sau.';
/** Title of the VPN notice (shell.vpn.title): only for a network error of the internal gateway (F051-AC-14). */
const VPN = 'Cần kết nối mạng quản trị (VPN)';

type Status = 'OK' | 'empty' | 'placeholder' | 'error';
const STATUS_LABEL: Record<Status, string> = {
  OK: 'OK',
  empty: 'Trống (chưa có dữ liệu)',
  placeholder: 'Placeholder (tính năng chưa làm)',
  error: 'LỖI',
};

interface Shot {
  folder: string;
  file: string;
  role: string;
  route: string;
  title: string;
  status: Status;
  note: string;
}

/** Shots are appended to <OUT>/shots.jsonl so that a failing test (new worker) does not lose the others. */
const LOG = () => path.join(OUT, 'shots.jsonl');

function record(s: Shot): void {
  fs.mkdirSync(OUT, { recursive: true });
  fs.appendFileSync(LOG(), `${JSON.stringify(s)}\n`);
}

function recorded(): Shot[] {
  if (!fs.existsSync(LOG())) return [];
  const byFile = new Map<string, Shot>();
  for (const line of fs.readFileSync(LOG(), 'utf8').split('\n').filter(Boolean)) {
    const s = JSON.parse(line) as Shot;
    byFile.set(`${s.folder}/${s.file}`, s);
  }
  return [...byFile.values()];
}

/** Collects failed API calls and page errors between two screenshots. */
class Watch {
  failures: string[] = [];
  constructor(page: Page) {
    page.on('response', (r) => {
      const u = new URL(r.url());
      if (r.status() >= 400 && u.pathname.includes('/api/') && !u.pathname.endsWith('/auth/portal/token:refresh')) {
        this.failures.push(`API ${r.status()} ${r.request().method()} ${u.pathname}`);
      }
    });
    page.on('pageerror', (e) => this.failures.push(`page error: ${e.message.split('\n')[0]}`));
  }
  take(): string[] {
    const f = [...new Set(this.failures)];
    this.failures = [];
    return f;
  }
}

async function settle(page: Page): Promise<void> {
  await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => undefined);
  await page
    .waitForFunction(() => !document.querySelector('.ant-skeleton-active, .ant-spin-spinning'), undefined, { timeout: 10_000 })
    .catch(() => undefined);
  await page.waitForTimeout(400);
}

/** What the screen says: blocking errors (VPN notice included), placeholder, warnings/infos, empty states. */
async function describeScreen(page: Page, failures: string[]): Promise<{ title: string; status: Status; note: string }> {
  const s = await page.evaluate(
    ([placeholder, vpn]) => {
      const txt = (sel: string) => [...document.querySelectorAll(sel)].map((e) => (e.textContent ?? '').replace(/\s+/g, ' ').trim()).filter(Boolean);
      const body = document.body.textContent ?? '';
      return {
        h1: txt('h1')[0] ?? '',
        errors: txt('.ant-alert-error, .ant-result-error, .ant-result-403, .ant-result-404, .ant-result-500, .ant-result-warning .ant-result-title'),
        warnings: txt('.ant-alert-warning'),
        infos: txt('.ant-alert-info'),
        placeholder: body.includes(placeholder),
        vpn: body.includes(vpn),
        empty: document.querySelectorAll('.ant-empty').length,
      };
    },
    [PLACEHOLDER, VPN] as const,
  );
  const errors = [...(s.vpn ? [VPN] : []), ...s.errors, ...failures];
  const status: Status = errors.length ? 'error' : s.placeholder ? 'placeholder' : s.empty ? 'empty' : 'OK';
  const parts: string[] = [];
  if (errors.length) parts.push([...new Set(errors)].join(' | '));
  if (s.warnings.length) parts.push(`cảnh báo: ${s.warnings.join(' | ')}`);
  if (s.infos.length) parts.push(`thông tin: ${s.infos.join(' | ')}`);
  return { title: s.h1, status, note: parts.join(' — ').slice(0, 600) };
}

const slug = (route: string) => route.replace(/^\/+/, '').replace(/[?#].*$/, '').replace(/[^a-z0-9]+/gi, '-').replace(/^-|-$/g, '') || 'root';

async function shoot(page: Page, watch: Watch, folder: string, role: string, n: number, route: string, suffix = ''): Promise<void> {
  await settle(page);
  const name = `${role}-${String(n).padStart(2, '0')}-${slug(route)}${suffix}.png`;
  fs.mkdirSync(path.join(OUT, folder), { recursive: true });
  await page.screenshot({ path: path.join(OUT, folder, name), fullPage: true });
  const d = await describeScreen(page, watch.take());
  record({ folder, file: name, role, route, ...d });
}

function setTheme(page: Page, theme: 'light' | 'dark') {
  return page.addInitScript((t) => {
    try {
      const raw = window.localStorage.getItem('ew.portal.prefs');
      const p = raw ? JSON.parse(raw) : {};
      window.localStorage.setItem('ew.portal.prefs', JSON.stringify({ locale: 'vi-VN', density: 'default', ...p, theme: t }));
    } catch {
      // storage blocked: default theme
    }
  }, theme);
}

/** Switches the stored theme of the signed-in page (owner review of the overview screens in dark). */
function storeTheme(page: Page, theme: 'light' | 'dark') {
  return page.evaluate((t) => {
    const p = JSON.parse(window.localStorage.getItem('ew.portal.prefs') ?? '{}');
    window.localStorage.setItem('ew.portal.prefs', JSON.stringify({ ...p, theme: t }));
  }, theme);
}

/** Visits every menu of the catalog the sidebar shows to the signed-in role, each one once. */
async function visitAllMenus(page: Page, watch: Watch, role: string, start: number): Promise<number> {
  let n = start;
  const seen = new Set<string>();
  const catalogPaths = new Set(WORKSPACE_CATALOG.flatMap((w) => w.menus.map((m) => m.path)));
  for (const ws of WORKSPACE_CATALOG) {
    await spaGoto(page, ws.path);
    await settle(page);
    const nav = page.getByRole('navigation', { name: NAV });
    if (!(await nav.count())) continue;
    const links = await nav.first().locator('a[href]').evaluateAll((as) => as.map((a) => a.getAttribute('href') ?? '').filter(Boolean));
    watch.take();
    for (const href of links) {
      if (seen.has(href) || !catalogPaths.has(href)) continue;
      seen.add(href);
      await page.locator(`nav[aria-label="${NAV}"] a[href="${href}"]`).first().click();
      await page.waitForURL((u) => u.pathname.startsWith(href), { timeout: 15_000 }).catch(() => undefined);
      await shoot(page, watch, role, role, n++, href);
    }
  }
  return n;
}

/** Opens a select by its accessible name and clicks the option holding that text (or the first one). */
async function pick(page: Page, label: string, option?: RegExp): Promise<boolean> {
  try {
    await openSelect(page, label);
    const opts = openOptions(page);
    await (option ? opts.filter({ hasText: option }) : opts).first().click({ timeout: 30_000 });
    return true;
  } catch {
    return false;
  }
}

/** Extra screens of ad.truong.a: students of grade 3, the dashboard with a school, a grade page, overviews in dark. */
async function schoolAdminExtras(page: Page, watch: Watch, role: string, start: number): Promise<number> {
  let n = start;
  const menu = (key: string) => WORKSPACE_CATALOG.flatMap((w) => w.menus).find((m) => m.key === key)!;
  const students = menu('menu.school_lms.students');
  await spaGoto(page, students.path);
  await settle(page);
  const grade3 = await pick(page, 'Khối', /^Khối 3$/);
  await shoot(page, watch, role, role, n++, students.path, grade3 ? '-grade-3' : '-grade-picker-empty');
  const dash = menu('menu.school_lms.school_dashboard');
  await spaGoto(page, dash.path);
  await settle(page);
  // the school list may be empty for the role (then the shot shows the open, empty picker as the finding)
  const picked = await pick(page, 'Trường');
  if (picked) await page.getByTestId('widget-assignments').waitFor({ timeout: 30_000 }).catch(() => undefined);
  await shoot(page, watch, role, role, n++, dash.path, picked ? '-school-selected' : '-school-picker-empty');
  const gradeLink = page.locator('a[href*="/school-dashboard/grades/"]').first();
  if (await gradeLink.isVisible().catch(() => false)) {
    await gradeLink.click();
    await page.waitForURL(/\/school-dashboard\/grades\/\d+/, { timeout: 15_000 }).catch(() => undefined);
    await shoot(page, watch, role, role, n++, new URL(page.url()).pathname);
  }
  await storeTheme(page, 'dark');
  for (const key of ['menu.school_lms.school_dashboard'] as const) {
    await spaGoto(page, menu(key).path);
    await shoot(page, watch, role, role, n++, menu(key).path, '-dark');
  }
  await storeTheme(page, 'light');
  return n;
}

function writeIndex(): void {
  const shots = recorded();
  if (!OUT || !shots.length) return;
  const byFolder = new Map<string, Shot[]>();
  for (const s of shots) byFolder.set(s.folder, [...(byFolder.get(s.folder) ?? []), s]);
  const count = (st: Status) => shots.filter((s) => s.status === st).length;
  const cell = (v: string) => v.replace(/\|/g, '/');
  const lines = [
    '# Ảnh chụp Admin Portal',
    '',
    `Chụp ngày ${new Date().toISOString()} trên stack local (${process.env.EW_E2E_PORTAL_URL ?? ''}), spec \`@screenshots\`. Tổng ${shots.length} ảnh: ${count('OK')} OK, ${count('empty')} trống (chưa có dữ liệu), ${count('placeholder')} placeholder (tính năng chưa làm), ${count('error')} lỗi.`,
    '',
    '| Thư mục | Số ảnh | OK | Trống | Placeholder | Lỗi |',
    '|---|---|---|---|---|---|',
  ];
  for (const [folder, list] of byFolder) {
    const c = (st: Status) => list.filter((s) => s.status === st).length;
    lines.push(`| ${folder} | ${list.length} | ${c('OK')} | ${c('empty')} | ${c('placeholder')} | ${c('error')} |`);
  }
  lines.push('');
  const errors = shots.filter((s) => s.status === 'error');
  if (errors.length) {
    lines.push('## Màn có lỗi', '', '| Route | Vai trò | Ảnh | Lỗi |', '|---|---|---|---|');
    for (const s of errors) lines.push(`| \`${s.route}\` | ${s.role} | ${s.folder}/${s.file} | ${cell(s.note)} |`);
    lines.push('');
  }
  for (const [folder, list] of byFolder) {
    lines.push(`## ${folder}`, '', '| Route | Vai trò | Trạng thái | Tiêu đề (h1) | Ghi chú | Ảnh |', '|---|---|---|---|---|---|');
    for (const s of list) lines.push(`| \`${s.route}\` | ${s.role} | ${STATUS_LABEL[s.status]} | ${cell(s.title)} | ${cell(s.note)} | ${s.file} |`);
    lines.push('');
  }
  fs.mkdirSync(OUT, { recursive: true });
  fs.writeFileSync(path.join(OUT, 'index.md'), lines.join('\n'));
}

test.describe('@screenshots Ảnh chụp mọi màn portal', () => {
  test.skip(!e2eReady || !OUT, 'real stack or EW_E2E_SCREENSHOT_DIR not configured — OQ F051-9');
  test.use({ viewport: DESKTOP });
  test.afterAll(() => writeIndex());

  test('public pages: login (desktop/phone, light/dark), forgot password, student refused', async ({ browser }) => {
    test.setTimeout(5 * 60_000);
    if (fs.existsSync(LOG())) fs.writeFileSync(LOG(), ''); // first test of a run: start a fresh index
    let n = 1;
    for (const [theme, size, suffix] of [
      ['light', DESKTOP, '-desktop-light'],
      ['light', PHONE, '-phone-light'],
      ['dark', DESKTOP, '-desktop-dark'],
      ['dark', PHONE, '-phone-dark'],
    ] as const) {
      const ctx = await browser.newContext({ viewport: size, locale: 'vi-VN' });
      const page = await ctx.newPage();
      await setTheme(page, theme);
      const watch = new Watch(page);
      await page.goto('/login');
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      await shoot(page, watch, 'public', 'public', n++, '/login', suffix);
      await ctx.close();
    }
    const ctx = await browser.newContext({ viewport: DESKTOP, locale: 'vi-VN' });
    const page = await ctx.newPage();
    const watch = new Watch(page);
    await page.goto('/forgot-password');
    await expect(page.getByRole('heading', { level: 1, name: 'Quên mật khẩu' })).toBeVisible();
    await shoot(page, watch, 'public', 'public', n++, '/forgot-password');
    // hs.binh.3a, not hs.an.3a: the @ADR0095 suite signs hs.an.3a in just before, and a second refused sign-in of the
    // same student within a short time answered 401 INVALID_CREDENTIALS on the local stack (Keycloak brute force)
    await signIn(page, 'hs.binh.3a');
    // expected: 403 PORTAL_ACCESS_STAFF_ONLY → student message; any other answer is recorded as a finding in the note
    await expect(page.getByRole('alert').first()).toBeVisible();
    const refused = await page.getByText('Tài khoản học sinh không dùng được cổng quản trị').isVisible();
    const failures = watch.take().filter((f) => !(refused && f.startsWith('API 403')));
    watch.failures = failures;
    await shoot(page, watch, 'public', 'public', n++, '/login', '-student-refused');
    await ctx.close();
  });

  for (const role of ['ad.truong.a', 'qlcm.truong.a', 'ad.truong.b', 'sa.admin'] as const) {
    test(`${role}: every menu of the catalog visible to the role`, async ({ page }) => {
      test.setTimeout(30 * 60_000);
      const watch = new Watch(page);
      await signIn(page, role);
      let n = 1;
      if (role === 'qlcm.truong.a') {
        await expect(page.getByRole('heading', { name: 'Chọn đơn vị làm việc' })).toBeVisible();
        await shoot(page, watch, role, role, n++, '/select-tenant');
        await page.getByRole('radio', { name: /TH Demo A/ }).click();
        await page.getByRole('button', { name: 'Vào' }).click();
      }
      await expect(page.getByRole('navigation', { name: NAV })).toBeVisible({ timeout: 30_000 });
      n = await visitAllMenus(page, watch, role, n);
      if (role === 'ad.truong.a') n = await schoolAdminExtras(page, watch, role, n);
      expect(n).toBeGreaterThan(2);
    });
  }
});
