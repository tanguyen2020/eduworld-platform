// Read hooks of the "Đối tượng" step of the assignment wizard (F055 T018; F055-AC-02, ADR-0095): the grades a school
// teaches (listTenantSchools — `grade_min`…`grade_max`, clamped to the grades 1–9 of the contract), the subjects of the
// tenant (listSubjects) and a search of the students of one school.
//
// Student search — interim (ADR-0095 gap, WP-I1): the contract has no "students of a school by grade" operation yet
// (`listTenantGradeStudents` comes with WP-I1), so the search uses listTenantUsers (`user_type=student`, `school_id`,
// `q` ≥ 2 characters). That operation needs `tenant.user.read`; a user without it (e.g. an academic manager) gets 403 and
// the wizard says that students cannot be looked up here — grade targets still work. Options only come from the API;
// whether a student really belongs to the school is checked by the server (422 for another school).
import { useQuery } from '@tanstack/react-query';
import { PAGE_SIZE, qk, useApi, usePagedList, type Schemas } from '../../common/api';
import { useSchoolLmsScope } from '../../common/scope';
import { useSchoolList } from '../../schools/useSchools';
import { LMS_LIMITS } from '../model/fieldSchemas';

export type Subject = Schemas['Subject'];
export type StudentUser = Schemas['User'];

/** Grades a school teaches, within the grade range of AssignmentTarget (1–9). */
export function gradesOf(school: Pick<Schemas['School'], 'grade_min' | 'grade_max'> | undefined): number[] {
  if (!school) return [];
  const { minimum, maximum } = LMS_LIMITS.gradeLevel;
  const from = Math.max(minimum, school.grade_min);
  const to = Math.min(maximum, school.grade_max);
  return from <= to ? Array.from({ length: to - from + 1 }, (_, i) => from + i) : [];
}

/** The school of the wizard among the schools of the scope (listTenantSchools) and its grades. */
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

/** Students of one school matching `search` (≥ 2 characters) — interim listTenantUsers, see the header. */
export function useSchoolStudentSearch(schoolId: string | undefined, search: string) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const q = search.trim().length >= 2 ? search.trim() : '';
  return usePagedList<StudentUser>(
    qk('school-students', scope, schoolId, q),
    (cursor) =>
      api.call(
        api.client.GET('/tenant/users', {
          params: { query: { limit: PAGE_SIZE, user_type: 'student', school_id: schoolId!, q, ...(cursor ? { cursor } : {}) } },
          headers: scope.headers,
        }),
      ),
    scope.ready && !!schoolId && !!q,
  );
}
