// F056 T052 — school calendar of the "Năm học này" / "Học kỳ này" presets from listSchoolAcademicYears (F010 T047).
import { describe, expect, it } from 'vitest';
import { schoolCalendar, type SchoolAcademicYear } from './calendar';

const year = (start: string, end: string, current: boolean, terms: SchoolAcademicYear['terms'] = []): SchoolAcademicYear => ({
  academic_year_id: `0193a4d2-af00-7000-8000-${start.replace(/-/g, '').padStart(12, '0')}`,
  name: `${start.slice(0, 4)}-${end.slice(0, 4)}`,
  start_date: start,
  end_date: end,
  is_current: current,
  terms,
});

describe('schoolCalendar (F056 T052)', () => {
  it('no year → both presets disabled', () => {
    expect(schoolCalendar(undefined, '2026-11-21')).toEqual({});
    expect(schoolCalendar([], '2026-11-21')).toEqual({});
  });

  it('the year containing today; no term stored → only the school year', () => {
    const ys = [year('2026-09-01', '2027-05-31', true), year('2025-09-01', '2026-05-31', false)];
    expect(schoolCalendar(ys, '2026-11-21')).toEqual({ school_year: { from: '2026-09-01', to: '2027-05-31' } });
  });

  it('the term containing today', () => {
    const ys = [year('2026-09-01', '2027-05-31', true, [
      { name: 'Học kỳ I', start_date: '2026-09-01', end_date: '2027-01-15' },
      { name: 'Học kỳ II', start_date: '2027-01-16', end_date: '2027-05-31' },
    ])];
    expect(schoolCalendar(ys, '2027-02-01')?.term).toEqual({ from: '2027-01-16', to: '2027-05-31' });
  });

  it('summer: the current year already started (range capped at today by the picker); a future year is ignored', () => {
    expect(schoolCalendar([year('2025-09-01', '2026-05-31', true)], '2026-07-10')).toEqual({ school_year: { from: '2025-09-01', to: '2026-05-31' } });
    expect(schoolCalendar([year('2026-09-01', '2027-05-31', true)], '2026-07-10')).toEqual({});
  });
});
