// Classes and their students (F052 T017, T018, T059; F052-AC-04, F052-AC-05, F052-AC-31): read-only lists from SIS/CSV
// (listTenantSchoolClasses, listTenantClassStudents, listUnassignedStudents) and the one write of the class screen,
// resetStudentPassword (Idempotency-Key per confirmation, reused on retry — useIdempotentMutation).
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { PAGE_SIZE, qk, useApi, usePagedList, type Schemas } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';

export type SchoolClass = Schemas['Class'];
export type ClassStudent = Schemas['ClassStudent'];
export type UnassignedStudent = Schemas['UnassignedStudent'];
export type PasswordResetResult = Schemas['StudentPasswordResetResult'];

export interface ClassFilters {
  academicYear?: string;
  grade?: number;
}

export function useSchoolClasses(schoolId: string | undefined, f: ClassFilters = {}) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return usePagedList<SchoolClass>(
    qk('classes', scope, schoolId, f.academicYear ?? '', f.grade ?? ''),
    (cursor) =>
      api.call(
        api.client.GET('/tenant/schools/{school_id}/classes', {
          params: {
            path: { school_id: schoolId! },
            query: { limit: PAGE_SIZE, ...(cursor ? { cursor } : {}), ...(f.academicYear ? { academic_year: f.academicYear } : {}), ...(f.grade ? { grade: f.grade } : {}) },
          },
          headers: scope.headers,
        }),
      ),
    scope.ready && !!schoolId,
  );
}

export function useClassStudents(schoolId: string | undefined, classId: string | undefined, search = '') {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const q = search.trim().length >= 2 ? search.trim() : '';
  return usePagedList<ClassStudent>(
    qk('class-students', scope, schoolId, classId, q),
    (cursor) =>
      api.call(
        api.client.GET('/tenant/schools/{school_id}/classes/{class_id}/students', {
          params: { path: { school_id: schoolId!, class_id: classId! }, query: { limit: PAGE_SIZE, ...(cursor ? { cursor } : {}), ...(q ? { q } : {}) } },
          headers: scope.headers,
        }),
      ),
    scope.ready && !!schoolId && !!classId,
  );
}

export function useUnassignedStudents(schoolId: string | undefined) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return usePagedList<UnassignedStudent>(
    qk('unassigned', scope, schoolId ?? 'all'),
    (cursor) =>
      api.call(
        api.client.GET('/tenant/students/unassigned', {
          params: { query: { limit: PAGE_SIZE, ...(cursor ? { cursor } : {}), ...(schoolId ? { school_id: schoolId } : {}) } },
          headers: scope.headers,
        }),
      ),
    scope.ready,
  );
}

/** One class of a school by id, from the (cached) class list — header of the class page. */
export function useClassName(schoolId: string | undefined, classId: string | undefined) {
  const classes = useSchoolClasses(schoolId);
  return classes.items.find((c) => c.class_id === classId)?.name ?? '';
}

export function useResetStudentPassword() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useIdempotentMutation<PasswordResetResult, { studentId: string; reason: string }>((v, intent) =>
    api.call(
      api.client.POST('/tenant/students/{student_id}/password:reset', {
        params: { path: { student_id: v.studentId } },
        body: { reason: v.reason },
        headers: { ...scope.headers, ...intent },
      }),
    ),
  );
}
