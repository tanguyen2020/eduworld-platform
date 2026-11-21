// Read hooks of the "Đối tượng" step of the assignment wizard and of "Học sinh theo khối" (F055 T018, F052; ADR-0095):
// the grades a school teaches (listTenantSchools — `grade_min`…`grade_max`, clamped to the grades 1–9 of the contract),
// the subjects of the tenant (listSubjects) and the students of one grade of one school (listTenantGradeStudents —
// `tenant.student.read` or `lms.student.read`, cursor, optional `q` ≥ 2 characters). Options only come from the API; the
// server checks scope and school (404 outside the scope, 422 for a student of another school on create).
import { useQuery } from '@tanstack/react-query';
import { PAGE_SIZE, qk, useApi, usePagedList, type Schemas } from '../../common/api';
import { useSchoolLmsScope } from '../../common/scope';
import { useSchoolList } from '../../schools/useSchools';
import { LMS_LIMITS } from '../model/fieldSchemas';

export type Subject = Schemas['Subject'];
export type GradeStudent = Schemas['GradeStudent'];

/** Grades a school teaches, within the grade range of AssignmentTarget (1–9). */
export function gradesOf(school: Pick<Schemas['School'], 'grade_min' | 'grade_max'> | undefined): number[] {
  if (!school) return [];
  const { minimum, maximum } = LMS_LIMITS.gradeLevel;
  const from = Math.max(minimum, school.grade_min);
  const to = Math.min(maximum, school.grade_max);
  return from <= to ? Array.from({ length: to - from + 1 }, (_, i) => from + i) : [];
}

/** The school among the schools of the scope (listTenantSchools) and its grades. */
export function useSchoolGrades(schoolId: string | undefined) {
  const schools = useSchoolList();
  const school = schools.items.find((s) => s.school_id === schoolId);
  return { school, grades: gradesOf(school), isFetching: schools.isFetching, error: schools.error };
}

/** Subjects of the tenant (listSubjects). */
export function useTenantSubjects() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useQuery({
    queryKey: qk('subjects', scope),
    queryFn: async () => (await api.call<{ items: Subject[] }>(api.client.GET('/learning/subjects', { headers: scope.headers } as never))).items,
    enabled: scope.ready,
    staleTime: 10 * 60_000,
  });
}

/** Students of one grade of one school (listTenantGradeStudents), filtered by `search` when it has ≥ 2 characters. */
export function useGradeStudents(schoolId: string | undefined, grade: number | undefined, search = '') {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const q = search.trim().length >= 2 ? search.trim() : '';
  return usePagedList<GradeStudent>(
    qk('grade-students', scope, schoolId, grade, q),
    (cursor) =>
      api.call(
        api.client.GET('/tenant/schools/{school_id}/grades/{grade_level}/students', {
          params: { path: { school_id: schoolId!, grade_level: grade! }, query: { limit: PAGE_SIZE, ...(q ? { q } : {}), ...(cursor ? { cursor } : {}) } },
          headers: scope.headers,
        }),
      ),
    scope.ready && !!schoolId && !!grade,
  );
}
