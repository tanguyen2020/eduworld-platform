// @F056 T021 — TC-LMS-003 on the REAL stack with the local golden dataset (LOCAL_DEVELOPMENT §5.2, `make seed-attempts
// VERIFY=1`): `ad.truong.a` opens the School Learning Dashboard overview (TH Demo A, grade 3, default range = the last
// 7 days) and the grade 3 page, and the numbers the backend answers and the portal shows are the golden ones:
// - getSchoolDashboard?grade_level=3: active_students 3, accuracy 0.643 (9/14, rapid_guess answers left out),
//   rapid_guess_ratio 0.067 (1/15), "[DEV] Phép chia có dư (golden)" and "[DEV] Chu vi hình vuông (chấm lại)" 3/8
//   submitted (progress bars of widget ①, F056-AC-01); the student who answered too fast carries "Làm quá nhanh" in
//   widget ③ (F056-AC-05);
// - getGradeDashboard (grade 3): heatmap MATH.G3.NUM.DIV_REMAINDER beginning 1 · developing 1 · proficient 1 ·
//   not_started 5 and MATH.G3.GEO.PERIMETER beginning 2 · developing 1 · not_started 5 (cells of the skill map).
// ORDER: run BEFORE content-studio/regrade.spec.ts (F058 T047) — the system regrade of the PERIMETER question v1 → v2
// changes that heatmap row (the golden values hold only before it, and only while no other grade 3 student of TH Demo A
// has made an attempt). Read only: nothing is created. Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise
// (OQ F051-9). No secret is written anywhere (AGENTS R4); no student name is printed (AGENTS R14).
import { expect, test, type Page, type Response } from '@playwright/test';
import { e2eReady } from '../support/auth';
import { openOptions, openSelect, spaGoto } from '../support/nav';
import { openSession, type Session } from '../support/session';

const GOLDEN = {
  active_students: 3,
  accuracy: 0.643,
  rapid_guess_ratio: 0.067,
  assignments: { '[DEV] Phép chia có dư (golden)': [3, 8], '[DEV] Chu vi hình vuông (chấm lại)': [3, 8] } as Record<string, [number, number]>,
  heatmap: {
    'MATH.G3.NUM.DIV_REMAINDER': { not_started: 5, beginning: 1, developing: 1, proficient: 1, mastered: 0 },
    'MATH.G3.GEO.PERIMETER': { not_started: 5, beginning: 2, developing: 1, proficient: 0, mastered: 0 },
  } as Record<string, Record<string, number>>,
};

/** Level names of the skill map (dashboard.mastery.* vi). */
const LEVELS: Record<string, string> = { not_started: 'Chưa bắt đầu', beginning: 'Mới bắt đầu', developing: 'Đang tiến bộ', proficient: 'Thành thạo', mastered: 'Xuất sắc' };

interface SchoolDashboardBody {
  widgets?: {
    active_students?: number;
    accuracy?: number;
    rapid_guess_ratio?: number;
    assignments?: { title?: string; submitted?: number; total?: number }[];
    students_needing_attention?: { reasons?: string[] }[];
  };
}
interface GradeDashboardBody {
  data?: { objective_distribution?: { code?: string; students_with_evidence?: number; level_counts?: Record<string, number> }[] }[];
  objective_distribution?: { code?: string; students_with_evidence?: number; level_counts?: Record<string, number> }[];
}

const SCHOOL_DASHBOARD = (r: Response) => /\/api\/v1\/lms\/dashboards\/school(\?|$)/.test(r.url()) && r.request().method() === 'GET';
const round3 = (v: number | undefined) => (v === undefined ? undefined : Math.round(v * 1000) / 1000);

async function settle(page: Page): Promise<void> {
  await page
    .waitForFunction(() => !document.querySelector('.ant-skeleton-active, .ant-spin-spinning'), undefined, { timeout: 30_000 })
    .catch(() => undefined);
}

test.describe('@F056 Dashboard — golden dataset (TC-LMS-003)', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');
  test.describe.configure({ mode: 'serial' });

  let s: Session;
  let schoolId: string | undefined;

  test.beforeAll(async ({ browser }) => {
    test.setTimeout(3 * 60_000);
    s = await openSession(browser, 'ad.truong.a');
  });
  test.afterAll(async () => s?.close());

  test('overview of grade 3 (7 days): golden widgets', async () => {
    test.setTimeout(3 * 60_000);
    const { page } = s;
    const first = page.waitForResponse(SCHOOL_DASHBOARD, { timeout: 60_000 });
    await spaGoto(page, '/school-lms/school-dashboard');
    let res = await first;
    schoolId = new URL(res.url()).searchParams.get('school_id') ?? undefined;
    expect(schoolId, 'the overview picks the only school of ad.truong.a (TH Demo A)').toBeTruthy();
    // filter grade 3 (getSchoolDashboard?grade_level=3, the golden source); the filters are kept in the session, so
    // grade 3 may already be the filter of the first call
    if (new URL(res.url()).searchParams.get('grade_level') !== '3') {
      const graded = page.waitForResponse((r) => SCHOOL_DASHBOARD(r) && new URL(r.url()).searchParams.get('grade_level') === '3', { timeout: 60_000 });
      await openSelect(page, 'Khối');
      await openOptions(page).filter({ hasText: /^Khối 3$/ }).first().click();
      res = await graded;
    }
    expect(res.status()).toBe(200);
    const q = new URL(res.url()).searchParams;
    const days = (Date.parse(q.get('to') ?? '') - Date.parse(q.get('from') ?? '')) / 86_400_000;
    expect(days, 'default range = today − 6 … today').toBe(6);
    const w = ((await res.json()) as SchoolDashboardBody).widgets ?? {};
    expect({ active: w.active_students, accuracy: round3(w.accuracy), rapid: round3(w.rapid_guess_ratio) }).toEqual({
      active: GOLDEN.active_students,
      accuracy: GOLDEN.accuracy,
      rapid: GOLDEN.rapid_guess_ratio,
    });
    for (const [title, [submitted, total]] of Object.entries(GOLDEN.assignments)) {
      const a = (w.assignments ?? []).find((x) => x.title === title);
      expect(a, `assignment ${title} in widget ①`).toBeTruthy();
      expect([a?.submitted, a?.total]).toEqual([submitted, total]);
    }
    // what the portal shows: progress bars of widget ① (accessible name = title + submitted/total)
    await settle(page);
    const widget = page.getByTestId('widget-assignments');
    for (const [title, [submitted, total]] of Object.entries(GOLDEN.assignments)) {
      await expect(widget.getByRole('progressbar', { name: `Tiến độ nộp bài ${title}: ${submitted}/${total} đã nộp` })).toBeVisible();
    }
    // the student who answered too fast is flagged neutrally in widget ③ (F056-AC-05), never ranked
    const rapid = (w.students_needing_attention ?? []).filter((x) => x.reasons?.includes('rapid_guess')).length;
    if (rapid > 0) await expect(page.getByTestId('widget-attention').getByText('Làm quá nhanh').first()).toBeVisible();
  });

  test('grade 3 page: heatmap of the golden objectives', async () => {
    test.setTimeout(3 * 60_000);
    test.skip(!schoolId, 'no school on the overview');
    const { page } = s;
    const grade = page.waitForResponse((r) => new RegExp(`/lms/schools/${schoolId}/grades/3/dashboard`).test(r.url()) && r.request().method() === 'GET', { timeout: 60_000 });
    await spaGoto(page, `/school-lms/school-dashboard/grades/3?school=${schoolId}`);
    const res = await grade;
    expect(res.status()).toBe(200);
    const body = (await res.json()) as GradeDashboardBody;
    const dist = body.objective_distribution ?? body.data?.[0]?.objective_distribution ?? [];
    for (const [code, counts] of Object.entries(GOLDEN.heatmap)) {
      const o = dist.find((x) => x.code === code);
      expect(o, `objective ${code} in the heatmap`).toBeTruthy();
      expect(o?.students_with_evidence).toBe(3);
      const got = Object.fromEntries(Object.keys(counts).map((k) => [k, o?.level_counts?.[k] ?? 0]));
      expect(got, code).toEqual(counts);
    }
    // the skill map shows the same counts (one cell per level, server numbers)
    const heatmap = page.getByTestId('mastery-heatmap');
    await expect(heatmap).toBeVisible({ timeout: 60_000 });
    for (const [code, counts] of Object.entries(GOLDEN.heatmap)) {
      const row = heatmap.locator('tr').filter({ has: page.locator('th', { hasText: code }) });
      await expect(row).toHaveCount(1);
      for (const [level, n] of Object.entries(counts)) {
        // accessible name of the cell (the visible text adds the level symbol, e.g. "○ 5")
        await expect(row.locator(`td[data-level="${level}"]`)).toHaveAttribute('aria-label', `${code} · ${LEVELS[level]}: ${n} học sinh`);
      }
    }
  });
});
