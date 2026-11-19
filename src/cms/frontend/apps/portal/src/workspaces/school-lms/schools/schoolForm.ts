// Pure helpers of the school edit form (F052-AC-02, ADMIN_PORTAL §3.16, ADR-0060) — display rules only; the server
// rejects writes to SIS-owned data anyway.
// - A school owned by SIS (`source = sis`) shows its SIS data read-only (edited in SIS); the province stays editable
//   while SIS did not provide one. Manual/imported schools are editable.
// - The PATCH body carries only changed fields (`SchoolUpdateRequest.minProperties = 1`).
import type { School, SchoolUpdate } from './useSchools';

export type SchoolFormField = 'name_vi' | 'name_en' | 'province_code' | 'school_level' | 'grade_min' | 'grade_max' | 'address';

export interface SchoolFormValues {
  name_vi: string;
  name_en?: string;
  province_code?: string;
  school_level: School['school_level'];
  grade_min: number;
  grade_max: number;
  address?: string | null;
}

export function readOnlyFields(school: Pick<School, 'source' | 'province'>): Set<SchoolFormField> {
  if (school.source !== 'sis') return new Set();
  const locked: SchoolFormField[] = ['name_vi', 'name_en', 'school_level', 'grade_min', 'grade_max', 'address'];
  if (school.province) locked.push('province_code');
  return new Set(locked);
}

export function formValues(s: School): SchoolFormValues {
  return {
    name_vi: s.name.vi,
    name_en: s.name.en,
    province_code: s.province?.code,
    school_level: s.school_level,
    grade_min: s.grade_min,
    grade_max: s.grade_max,
    address: s.address ?? null,
  };
}

/** Changed, editable fields only; `null` when nothing changed. */
export function updateBody(school: School, v: SchoolFormValues): SchoolUpdate | null {
  const locked = readOnlyFields(school);
  const before = formValues(school);
  const body: SchoolUpdate = {};
  const changed = (f: SchoolFormField) => !locked.has(f) && (v[f] ?? null) !== (before[f] ?? null);
  if (changed('name_vi') || changed('name_en')) body.name = { vi: v.name_vi.trim(), ...(v.name_en?.trim() ? { en: v.name_en.trim() } : {}) };
  if (changed('province_code') && v.province_code) body.province_code = v.province_code;
  if (changed('school_level')) body.school_level = v.school_level;
  if (changed('grade_min')) body.grade_min = v.grade_min;
  if (changed('grade_max')) body.grade_max = v.grade_max;
  if (changed('address')) body.address = v.address?.trim() ? v.address.trim() : null;
  return Object.keys(body).length ? body : null;
}
