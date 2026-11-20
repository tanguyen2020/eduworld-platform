// F056-AC-08 / F056-AC-19: dashboard filters survive the URL round trip (drill-down keeps range, subject, grade); the
// subject filter of a subject-scoped academic manager lists only its subjects; dashboard i18n vi/en have the same keys.
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import en from '../../../../i18n/dashboard.en.json';
import vi from '../../../../i18n/dashboard.vi.json';
import { defaultRange, filtersFromSearch, filtersToSearch, subjectLabel, subjectsInScope } from './filters';

const keys = (o: unknown, p = ''): string[] => (o && typeof o === 'object' ? Object.entries(o).flatMap(([k, v]) => keys(v, p ? `${p}.${k}` : k)) : [p]);

describe('dashboard filters', () => {
  it('URL round trip keeps school, grade, class, subject, range and grain; bad values fall back', () => {
    const f = { schoolId: 's', grade: 3, classId: 'c', subjectId: 'm', range: { preset: 'custom' as const, from: '2026-01-01', to: '2026-03-31', grain: 'month' as const } };
    expect(filtersFromSearch(filtersToSearch(f), '2026-11-21')).toEqual(f);
    const bad = filtersFromSearch(new URLSearchParams('grade=99&preset=all_time&from=x'), '2026-11-21');
    expect(bad.grade).toBeUndefined();
    expect(bad.range).toEqual(defaultRange('2026-11-21'));
    expect(filtersFromSearch(new URLSearchParams('preset=30d'), '2026-11-21').range).toEqual({ preset: '30d', from: '2026-10-23', to: '2026-11-21' });
  });

  it('subjects in scope: subject-scoped roles only; school/tenant roles see all', () => {
    const subjects = [
      { subject_id: 'math', code: 'math', name: { vi: 'Toán', en: 'Math' } },
      { subject_id: 'viet', code: 'viet', name: { vi: 'Tiếng Việt' } },
    ];
    expect(subjectsInScope(subjects, [{ role: 'academic_manager', scope_type: 'subject', scope_id: 'math' }]).map((s) => s.subject_id)).toEqual(['math']);
    expect(subjectsInScope(subjects, [{ role: 'school_admin', scope_type: 'school', scope_id: 's' }])).toHaveLength(2);
    expect(subjectsInScope(subjects, [])).toHaveLength(2);
    expect(subjectLabel(subjects[1]!, 'en-US')).toBe('Tiếng Việt');
    expect(subjectLabel(subjects[0]!, 'en-US')).toBe('Math');
  });

  it('dashboard translations: vi and en have the same keys', () => {
    expect(keys(en).sort()).toEqual(keys(vi).sort());
  });

  it('every `dashboard.*` key used by the F056 screens exists (dynamic keys by prefix)', () => {
    const known = keys(vi);
    const files = (dir: string): string[] =>
      fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => (e.isDirectory() ? files(path.join(dir, e.name)) : /\.tsx?$/.test(e.name) && !/\.test\./.test(e.name) ? [path.join(dir, e.name)] : []));
    const missing: string[] = [];
    for (const f of [...files(path.resolve(__dirname, '..')), path.resolve(__dirname, '../../../platform-admin/reports/PlatformReportsPage.tsx')]) {
      for (const m of fs.readFileSync(f, 'utf8').matchAll(/t\(['`](dashboard\.[A-Za-z0-9_.]+)(\$\{)?/g)) {
        const key = m[1]!;
        const dynamic = !!m[2];
        if (dynamic ? !known.some((k) => k.startsWith(key)) : !known.includes(key)) missing.push(`${path.basename(f)}: ${key}`);
      }
    }
    expect(missing).toEqual([]);
  });
});
