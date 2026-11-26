// F081-AC-04, AC-05, AC-07 (pure helpers): errors[].field of 422 VALIDATION_FAILED / EVENT_TEMPLATE_PARAM_NOT_ALLOWED go
// to their form field with a generic message (never a matched word); 409/412/503 give the screen messages; ICT wall times
// are sent in UTC; only the allowed parameters leave the form.
import dayjs from 'dayjs';
import { describe, expect, it } from 'vitest';
import { ApiError } from '../../../../shared/api/middleware/problem';
import { mapEventProblem } from '../mapProblem';
import { eventParams } from '../SchoolEventForm';
import { durationDays, ictDayjs, ictIso, isCancelable, isEditable } from '../ui';

const err = (status: number, code: string, fields: Record<string, string> = {}) =>
  new ApiError({ status, code, i18nKey: `error.${code}`, traceId: 'trace-1', fieldErrors: Object.fromEntries(Object.entries(fields).map(([k, c]) => [k, { code: c }])) });
const t = (k: string) => k;

describe('mapEventProblem', () => {
  it('422 VALIDATION_FAILED → the field, generic message (no word)', () => {
    const m = mapEventProblem(err(422, 'VALIDATION_FAILED', { 'name.vi': 'invalid' }), 'create', t);
    expect(m.fields).toEqual([{ name: ['name', 'vi'], errors: ['portal.school_lms.schoolEvents.errors.fieldInvalid'] }]);
    expect(m.banner).toBeNull();
  });
  it('422 EVENT_TEMPLATE_PARAM_NOT_ALLOWED → the parameter field', () => {
    const m = mapEventProblem(err(422, 'EVENT_TEMPLATE_PARAM_NOT_ALLOWED', { 'audience.school_ids': 'invalid' }), 'create', t);
    expect(m.fields).toEqual([{ name: ['audience', 'school_ids'], errors: ['portal.school_lms.schoolEvents.errors.paramNotAllowed'] }]);
  });
  it('a field the form does not have → banner', () => {
    expect(mapEventProblem(err(422, 'EVENT_TEMPLATE_PARAM_NOT_ALLOWED', { unknown: 'x' }), 'create', t).banner).toBe('portal.school_lms.schoolEvents.errors.paramNotAllowed');
  });
  it('409 on create = limit, on update = changed; 412 = changed; 503 = retry, form kept; FEATURE_DISABLED', () => {
    expect(mapEventProblem(err(409, 'CONFLICT'), 'create', t).banner).toBe('portal.school_lms.schoolEvents.errors.tooMany');
    expect(mapEventProblem(err(409, 'CONFLICT'), 'update', t).banner).toBe('portal.school_lms.schoolEvents.errors.changed');
    expect(mapEventProblem(err(412, 'VERSION_MISMATCH'), 'update', t).banner).toBe('portal.school_lms.schoolEvents.errors.changed');
    expect(mapEventProblem(err(503, 'UNAVAILABLE'), 'create', t).banner).toBe('portal.school_lms.schoolEvents.errors.unavailable');
    expect(mapEventProblem(err(403, 'FEATURE_DISABLED'), 'create', t).banner).toBe('portal.school_lms.errors.FEATURE_DISABLED');
    expect(mapEventProblem(new Error('x'), 'create', t).banner).toBe('error.generic');
  });
});

describe('form helpers', () => {
  it('ICT wall time ↔ UTC; duration in days', () => {
    const d = dayjs(new Date(2026, 11, 1, 8, 0));
    expect(ictIso(d)).toBe('2026-12-01T01:00:00.000Z');
    expect(ictDayjs('2026-12-01T01:00:00Z')!.format('DD/MM/YYYY HH:mm')).toBe('01/12/2026 08:00');
    expect(durationDays(d, d.add(7, 'day'))).toBe(7);
    expect(durationDays(null, d)).toBe(0);
  });
  it('only the allowed parameters leave the form', () => {
    const tpl = { allowed_params: ['audience.grade_levels', 'reward_option_id'] };
    const out = eventParams(
      { name: { vi: ' Tên ', en: '' }, description: { vi: 'Mô tả' }, audience: { school_ids: ['s'], grade_levels: [5, 4] }, community_goal_target: 9, quest_ids: ['q'], reward_option_id: 'r' },
      tpl,
      ictIso,
    );
    expect(out).toEqual({ name: { vi: 'Tên' }, audience: { grade_levels: [4, 5] }, reward_option_id: 'r' });
  });
  it('edit only while scheduled; cancel while scheduled or live (never suspended)', () => {
    expect(['scheduled', 'live', 'suspended', 'ended', 'cancelled'].filter((s) => isEditable(s as never))).toEqual(['scheduled']);
    expect(['scheduled', 'live', 'suspended', 'ended', 'cancelled'].filter((s) => isCancelable(s as never))).toEqual(['scheduled', 'live']);
  });
});
