// @screenshots — full-page PNG of every portal screen on the REAL stack (owner request 2026-11-21, review of the
// "Sân trường" sign-in and "Sổ tay giáo viên" working screens). Public pages (login desktop/phone, light/dark,
// forgot password, student refused) and, per demo role, every menu the generated catalog (ADR-0090) exposes to that
// role — read from the sidebar the portal renders after `getMyPermissions`, so nothing is listed by hand. Each shot
// gets a note: OK, empty (no data), or the error the screen shows (alerts, results, failed API calls, page errors),
// written to <EW_E2E_SCREENSHOT_DIR>/index.md. Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD, EW_E2E_SCREENSHOT_DIR (a
// folder outside the repo); every account signs in with its password only (ADR-0094).
// No secret is written to a file name, a note or the index (AGENTS R4).
import { expect, test, type Page } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { WORKSPACE_CATALOG } from '../../src/generated/menu-catalog';
import { e2eReady, signIn } from './support/auth';

const OUT = process.env.EW_E2E_SCREENSHOT_DIR ?? '';
const DESKTOP = { width: 1440, height: 900 };
const PHONE = { width: 390, height: 844 };
const NAV = 'Menu điều hướng';
/** Screens of the class concept removed by ADR-0095 (students by school grade): kept as the before state. */
const TO_BE_REMOVED = ['/school-lms/classes', '/school-lms/courses', '/school-lms/schedules', '/school-lms/study-groups', '/content-studio/learning-paths'];
const REMOVED_NOTE = 'sẽ bị gỡ (ADR-0095)';

interface Shot {
  folder: string;
  file: string;
  role: string;
  route: string;
  title: string;
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

/** What the screen says: blocking errors, warnings/infos, empty states. */
async function describeScreen(page: Page, failures: string[]): Promise<{ title: string; note: string }> {
  const s = await page.evaluate(() => {
    const txt = (sel: string) => [...document.querySelectorAll(sel)].map((e) => (e.textContent ?? '').replace(/\s+/g, ' ').trim()).filter(Boolean);
    return {
      h1: txt('h1')[0] ?? '',
      errors: txt('.ant-alert-error, .ant-result-error, .ant-result-403, .ant-result-404, .ant-result-500, .ant-result-warning .ant-result-title'),
      warnings: txt('.ant-alert-warning'),
      infos: txt('.ant-alert-info'),
      empty: document.querySelectorAll('.ant-empty').length,
    };
  });
  const parts: string[] = [];
  if (s.errors.length || failures.length) parts.push(`LỖI: ${[...s.errors, ...failures].join(' | ')}`);
  else if (s.empty) parts.push('Trống (chưa có dữ liệu)');
  else parts.push('OK');
  if (s.warnings.length) parts.push(`cảnh báo: ${s.warnings.join(' | ')}`);
  if (s.infos.length) parts.push(`thông tin: ${s.infos.join(' | ')}`);
  return { title: s.h1, note: parts.join(' — ').slice(0, 600) };
}

const slug = (route: string) => route.replace(/^\/+/, '').replace(/[?#].*$/, '').replace(/[^a-z0-9]+/gi, '-').replace(/^-|-$/g, '') || 'root';

async function shoot(page: Page, watch: Watch, folder: string, role: string, n: number, route: string, suffix = ''): Promise<void> {
  await settle(page);
  const name = `${role}-${String(n).padStart(2, '0')}-${slug(route)}${suffix}.png`;
  fs.mkdirSync(path.join(OUT, folder), { recursive: true });
  await page.screenshot({ path: path.join(OUT, folder, name), fullPage: true });
  const d = await describeScreen(page, watch.take());
  if (TO_BE_REMOVED.some((p) => route.startsWith(p))) d.note = `${REMOVED_NOTE} — ${d.note}`;
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

/** Visits every menu of every workspace the sidebar shows to the signed-in role. */
async function visitAllMenus(page: Page, watch: Watch, role: string, start: number): Promise<number> {
  let n = start;
  for (const ws of WORKSPACE_CATALOG) {
    await page.goto(ws.path);
    await settle(page);
    const nav = page.getByRole('navigation', { name: NAV });
    if (!(await nav.count())) continue;
    const links = await nav.first().locator('a[href]').evaluateAll((as) => as.map((a) => a.getAttribute('href') ?? '').filter(Boolean));
    if (!links.length) continue;
    watch.take();
    for (const href of links) {
      await page.locator(`nav[aria-label="${NAV}"] a[href="${href}"]`).first().click();
      await page.waitForURL((u) => u.pathname.startsWith(href), { timeout: 15_000 }).catch(() => undefined);
      await shoot(page, watch, role, role, n++, href);
    }
  }
  return n;
}

function writeIndex(): void {
  const shots = recorded();
  if (!OUT || !shots.length) return;
  const byFolder = new Map<string, Shot[]>();
  for (const s of shots) byFolder.set(s.folder, [...(byFolder.get(s.folder) ?? []), s]);
  const live = shots.filter((s) => !s.note.startsWith(REMOVED_NOTE));
  const errors = live.filter((s) => s.note.startsWith('LỖI'));
  const empty = live.filter((s) => s.note.startsWith('Trống'));
  const removed = shots.length - live.length;
  const lines = [
    '# Ảnh chụp Admin Portal',
    '',
    `Chụp ngày ${new Date().toISOString()} trên stack local (${process.env.EW_E2E_PORTAL_URL ?? ''}), spec \`@screenshots\`. Tổng ${shots.length} ảnh; ${errors.length} màn có lỗi; ${empty.length} màn trống; ${removed} ảnh của màn "${REMOVED_NOTE}" (chỉ lưu trạng thái trước, không tính vào lỗi/trống).`,
    '',
  ];
  if (errors.length) {
    lines.push('## Màn có lỗi', '', '| Ảnh | Vai trò | Route | Ghi chú |', '|---|---|---|---|');
    for (const s of errors) lines.push(`| ${s.folder}/${s.file} | ${s.role} | \`${s.route}\` | ${s.note.replace(/\|/g, '/')} |`);
    lines.push('');
  }
  for (const [folder, list] of byFolder) {
    lines.push(`## ${folder}`, '', '| Ảnh | Route | Tiêu đề (h1) | Ghi chú |', '|---|---|---|---|');
    for (const s of list) lines.push(`| ${s.file} | \`${s.route}\` | ${s.title.replace(/\|/g, '/')} | ${s.note.replace(/\|/g, '/')} |`);
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
    await signIn(page, 'hs.an.3a');
    // expected: 403 PORTAL_ACCESS_STAFF_ONLY → student message; any other answer is recorded as a finding in the note
    await expect(page.getByRole('alert').first()).toBeVisible();
    const refused = await page.getByText('Tài khoản học sinh không dùng được cổng quản trị').isVisible();
    const failures = watch.take().filter((f) => !(refused && f.startsWith('API 403')));
    watch.failures = failures;
    await shoot(page, watch, 'public', 'public', n++, '/login', '-student-refused');
    await ctx.close();
  });

  for (const role of ['ad.truong.a', 'qlcm.truong.a', 'sa.admin'] as const) {
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
      if (role === 'ad.truong.a') {
        // school dashboard with the first school picked: the KPI widget cards with data of the seed
        const dash = WORKSPACE_CATALOG.flatMap((w) => w.menus).find((m) => m.key === 'menu.school_lms.school_dashboard')!;
        await page.goto(dash.path);
        // the school list may be empty for the role (then the shot shows the open, empty picker as the finding)
        await page.getByRole('combobox', { name: 'Trường' }).click({ timeout: 10_000 }).catch(() => undefined);
        const picked = await page
          .getByRole('option')
          .first()
          .click({ timeout: 10_000 })
          .then(() => true)
          .catch(() => false);
        if (picked) await page.getByTestId('widget-assignments').waitFor({ timeout: 30_000 }).catch(() => undefined);
        await shoot(page, watch, role, role, n++, dash.path, picked ? '-school-selected' : '-school-picker-empty');
        // dark theme of the overview screens (owner review of direction B)
        await page.evaluate(() => {
          const p = JSON.parse(window.localStorage.getItem('ew.portal.prefs') ?? '{}');
          window.localStorage.setItem('ew.portal.prefs', JSON.stringify({ ...p, theme: 'dark' }));
        });
        for (const key of ['menu.school_lms.overview', 'menu.school_lms.school_dashboard'] as const) {
          const menu = WORKSPACE_CATALOG.flatMap((w) => w.menus).find((m) => m.key === key)!;
          await page.goto(menu.path);
          await shoot(page, watch, role, role, n++, menu.path, '-dark');
        }
        await page.evaluate(() => {
          const p = JSON.parse(window.localStorage.getItem('ew.portal.prefs') ?? '{}');
          window.localStorage.setItem('ew.portal.prefs', JSON.stringify({ ...p, theme: 'light' }));
        });
      }
      expect(n).toBeGreaterThan(2);
    });
  }
});
