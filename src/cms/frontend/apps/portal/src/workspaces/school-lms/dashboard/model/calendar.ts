// School calendar of the range picker (F056 T052, F056-AC-03, F010 T047): "Năm học này" = the academic year of the school
// that contains today (else the current one already started), "Học kỳ này" = the term of that year containing today.
// Dates come from listSchoolAcademicYears (module identity); nothing is derived here — no year or term → the preset stays
// disabled (ops-ui DashboardRangePicker). Pure.
import type { CalendarPeriod } from '@eduworld/ops-ui/dashboard';
import type { Schemas } from '../../common/api';

export type SchoolAcademicYear = Schemas['SchoolAcademicYear'];

export interface SchoolCalendar {
  term?: CalendarPeriod;
  school_year?: CalendarPeriod;
}

const within = (p: { start_date: string; end_date: string }, day: string) => p.start_date <= day && day <= p.end_date;

export function schoolCalendar(years: readonly SchoolAcademicYear[] | undefined, today: string): SchoolCalendar {
  const year = years?.find((y) => within(y, today)) ?? years?.find((y) => y.is_current && y.start_date <= today);
  if (!year) return {};
  const term = year.terms.find((t) => within(t, today));
  return {
    school_year: { from: year.start_date, to: year.end_date },
    ...(term ? { term: { from: term.start_date, to: term.end_date } } : {}),
  };
}
