// TC-PERF-025 — School Learning Dashboard on 2 years of data (F132-AC-04, NFR-PERF-017/018, ADR-0062): 1,500 school
// staff open the dashboard at grade level within 10 min and 200 sessions pick a 1–2 year range, run together with
// TC-PERF-014; repeat with a cold cache (CACHE=cold only changes the report label). STAFF_FILE is a JSON array of
// {token, tenant_id, school_id} of perf-* staff (datagen staff.csv accounts, tokens obtained through the portal login
// by the runner; never committed). The 50,000 students × 730 days of rollups come from the dashboard data generator.
import { check } from 'k6';
import { SharedArray } from 'k6/data';
import exec from 'k6/execution';
import { call } from './lib/auth.js';
import { baseOptions, checkEnv, dur, scaled } from './lib/thresholds.js';

const gen = JSON.parse(open('./scenarios.gen.json'));
checkEnv(__ENV);
const BASE = __ENV.BASE_URL;
const staff = new SharedArray('staff', () => JSON.parse(open(__ENV.STAFF_FILE)));
const WINDOW = dur('10m', __ENV);
const WINDOW_S = parseInt(WINDOW, 10);
const OPENS = scaled(1500, __ENV);
const LONG = scaled(200, __ENV);

export const options = Object.assign(baseOptions(gen, 'dashboard_2y', __ENV), {
  scenarios: {
    grade_level: { executor: 'constant-arrival-rate', rate: Math.max(1, Math.ceil((OPENS * 60) / WINDOW_S)), timeUnit: '1m', duration: WINDOW,
      preAllocatedVUs: 50, maxVUs: 400, exec: 'gradeLevel' },
    long_range: { executor: 'constant-arrival-rate', rate: Math.max(1, Math.ceil((LONG * 60) / WINDOW_S)), timeUnit: '1m', duration: WINDOW,
      preAllocatedVUs: 20, maxVUs: 100, exec: 'longRange' },
  },
});

function day(offsetDays) {
  return new Date(Date.now() - offsetDays * 86400000).toISOString().slice(0, 10);
}

function open1(fromDays, grade) {
  const p = staff[exec.scenario.iterationInTest % staff.length];
  const s = { token: p.token, tenantId: p.tenant_id };
  const g = grade ? `&grade_level=${grade}` : '';
  const r = call('GET', `${BASE}/lms/dashboards/school?school_id=${p.school_id}${g}&from=${day(fromDays)}&to=${day(0)}&page_size=50`, s, 'dashboard', 'getSchoolDashboard');
  check(r, { 'dashboard ok': (x) => x.status === 200 });
}

export function gradeLevel() {
  open1(30, 1 + Math.floor(Math.random() * 9));
}

export function longRange() {
  open1(365 + Math.floor(Math.random() * 365), 0);
}
