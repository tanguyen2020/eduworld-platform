// Filters of the School Learning Dashboard (F056 T018, F056-AC-01, F056-AC-08, F056-AC-19): school, grade, subject
// and range live in the URL so a drill-down (school → grade → student, ADR-0095: no class) keeps them; the last grade/subject/preset
// is remembered per viewer and tenant in localStorage (convenience only, no student data — data-model F056). The
// subject list of a subject-scoped academic manager only holds the subjects of their scope (display filter: the server
// answers 403 for any other subject).
import { presetRange, type DashboardRange, type RangePreset } from '@eduworld/ops-ui/dashboard';
import dayjs from 'dayjs';
import { readItem, writeItem } from '../../../../shared/storage';
import type { Schemas } from '../../common/api';

export interface DashboardFilters {
  schoolId?: string;
  grade?: number;
  subjectId?: string;
  range: DashboardRange;
}

const PRESETS: RangePreset[] = ['today', '7d', '30d', 'term', 'school_year', '12m', 'custom'];
const DATE = /^\d{4}-\d{2}-\d{2}$/;
const GRAINS = ['day', 'week', 'month'] as const;
/** Grades of the system (ADR-0095: khối 1–9). */
export const GRADES = [1, 2, 3, 4, 5, 6, 7, 8, 9] as const;

export const todayIso = () => dayjs().format('YYYY-MM-DD');

/** Default range: the last 7 days ("Tuần này" of the quickstart). */
export function defaultRange(today: string): DashboardRange {
  return { preset: '7d', ...presetRange('7d', today)! };
}

export function filtersFromSearch(p: URLSearchParams, today: string): DashboardFilters {
  const preset = PRESETS.includes(p.get('preset') as RangePreset) ? (p.get('preset') as RangePreset) : undefined;
  const from = p.get('from');
  const to = p.get('to');
  const grain = GRAINS.find((g) => g === p.get('grain'));
  let range = defaultRange(today);
  if (preset && from && to && DATE.test(from) && DATE.test(to)) range = { preset, from, to, ...(grain ? { grain } : {}) };
  else if (preset && preset !== 'custom' && preset !== 'term' && preset !== 'school_year') range = { preset, ...presetRange(preset, today)! };
  const grade = Number(p.get('grade'));
  return {
    schoolId: p.get('school') ?? undefined,
    grade: Number.isInteger(grade) && grade >= GRADES[0] && grade <= GRADES[GRADES.length - 1]! ? grade : undefined,
    subjectId: p.get('subject') ?? undefined,
    range,
  };
}

export function filtersToSearch(f: DashboardFilters): URLSearchParams {
  const p = new URLSearchParams();
  if (f.schoolId) p.set('school', f.schoolId);
  if (f.grade) p.set('grade', String(f.grade));
  if (f.subjectId) p.set('subject', f.subjectId);
  p.set('preset', f.range.preset);
  p.set('from', f.range.from);
  p.set('to', f.range.to);
  if (f.range.grain) p.set('grain', f.range.grain);
  return p;
}

const storageKey = (userKey: string, tenant: string) => `ew.portal.dashboard.filters.${userKey}.${tenant}`;

/** Last grade/subject/preset of this viewer in this tenant (never student data). */
export function rememberFilters(userKey: string | null, tenant: string, f: DashboardFilters): void {
  if (!userKey) return;
  writeItem('local', storageKey(userKey, tenant), JSON.stringify({ grade: f.grade, subjectId: f.subjectId, preset: f.range.preset === 'custom' ? undefined : f.range.preset }));
}

export function recalledSearch(userKey: string | null, tenant: string, today: string): URLSearchParams | null {
  if (!userKey) return null;
  try {
    const raw = readItem('local', storageKey(userKey, tenant));
    if (!raw) return null;
    const v = JSON.parse(raw) as { grade?: number; subjectId?: string; preset?: RangePreset };
    const p = new URLSearchParams();
    if (v.grade) p.set('grade', String(v.grade));
    if (v.subjectId) p.set('subject', v.subjectId);
    if (v.preset && v.preset !== 'term' && v.preset !== 'school_year' && v.preset !== 'custom') {
      const r = presetRange(v.preset, today);
      if (r) {
        p.set('preset', v.preset);
        p.set('from', r.from);
        p.set('to', r.to);
      }
    }
    return p;
  } catch {
    return null;
  }
}

type Subject = Schemas['Subject'];

/** Display name of a subject (LearningName: vi/en optional) in the UI language, else its code. */
export function subjectLabel(s: Pick<Subject, 'name' | 'code'>, locale: string): string {
  return (locale.startsWith('en') ? s.name.en || s.name.vi : s.name.vi || s.name.en) || s.code;
}
type EffectiveRole = Schemas['EffectiveRole'];

/**
 * Subjects offered in the filter (F056-AC-19): when every role of the session is scoped to subjects (academic manager
 * by subject), only those subjects; otherwise all subjects of the tenant. Display only — the server checks the scope.
 */
export function subjectsInScope(subjects: readonly Subject[], roles: readonly EffectiveRole[]): Subject[] {
  const scoped = roles.length > 0 && roles.every((r) => r.scope_type === 'subject');
  if (!scoped) return [...subjects];
  const ids = new Set(roles.map((r) => r.scope_id));
  return subjects.filter((s) => ids.has(s.subject_id));
}
