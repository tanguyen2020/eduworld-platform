// Course and learning path hooks (F055 T030; F055-AC-13…AC-15): listCourses (cursor), updateCourse (status
// active/hidden, If-Match = the row's `version` — contract delta F055-D1), listLearningPaths, create/updateLearningPath
// (If-Match = the path's `version`), unlockPathNodeForStudent (reason + Idempotency-Key).
import { useMutation, useQuery } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../../shared/api/useIdempotentMutation';
import { LMS_PAGE_SIZE, lmsKey, MERGE_PATCH, useDataList, useLmsContext, type DataPage, type S } from './lmsCommon';

export type Course = S['Course'];
export type LearningPath = S['LearningPath'];
export type LearningPathInput = S['LearningPathInput'];

/** If-Match of a resource whose list row carries its `version` (backend ETag format W/"n"). */
export const ifMatchOf = (version: number | undefined) => (version ? { 'If-Match': `W/"${version}"` } : {});

export interface CourseFilters {
  schoolId?: string;
  classId?: string;
  subjectId?: string;
  status?: Course['status'];
}

export function useCourses(f: CourseFilters = {}, enabled = true) {
  const { api, scope } = useLmsContext();
  return useDataList<Course>(
    lmsKey('courses', scope, f.schoolId ?? '', f.classId ?? '', f.subjectId ?? '', f.status ?? ''),
    (cursor) =>
      api.call<DataPage<Course>>(
        api.client.GET('/lms/courses', {
          params: {
            query: {
              limit: LMS_PAGE_SIZE,
              ...(cursor ? { cursor } : {}),
              ...(f.schoolId ? { school_id: f.schoolId } : {}),
              ...(f.classId ? { class_id: f.classId } : {}),
              ...(f.subjectId ? { subject_id: f.subjectId } : {}),
              ...(f.status ? { status: f.status } : {}),
            },
          },
          headers: scope.headers,
        } as never),
      ),
    scope.ready && enabled,
  );
}

/** Hide / show a course (school_admin, lms.course.write); If-Match from the row version. */
export function useUpdateCourseStatus() {
  const { api, scope, invalidate } = useLmsContext();
  return useMutation({
    mutationFn: (v: { course: Course; status: 'active' | 'hidden' }) =>
      api.call<Course>(
        api.client.PATCH('/lms/courses/{course_id}', {
          params: { path: { course_id: v.course.course_id } },
          body: { status: v.status },
          headers: { ...scope.headers, ...MERGE_PATCH, ...ifMatchOf((v.course as { version?: number }).version) },
        } as never),
      ),
    onSettled: () => void invalidate('courses'),
  });
}

export function useLearningPaths(courseId: string | undefined) {
  const { api, scope } = useLmsContext();
  return useQuery({
    queryKey: lmsKey('paths', scope, courseId),
    queryFn: () => api.call<{ data: LearningPath[] }>(api.client.GET('/lms/courses/{course_id}/paths', { params: { path: { course_id: courseId! } }, headers: scope.headers } as never)),
    enabled: scope.ready && !!courseId,
  });
}

/** Create (Idempotency-Key per intent) or update (If-Match = the path's version) a learning path. */
export function useSaveLearningPath(courseId: string) {
  const { api, scope, invalidate } = useLmsContext();
  return useIdempotentMutation<LearningPath, { pathId?: string; version?: number; body: LearningPathInput }>(
    (v, intent) =>
      v.pathId
        ? api.call<LearningPath>(
            api.client.PATCH('/lms/courses/{course_id}/paths/{path_id}', {
              params: { path: { course_id: courseId, path_id: v.pathId } },
              body: v.body,
              headers: { ...scope.headers, ...MERGE_PATCH, ...ifMatchOf(v.version) },
            } as never),
          )
        : api.call<LearningPath>(api.client.POST('/lms/courses/{course_id}/paths', { params: { path: { course_id: courseId } }, body: v.body, headers: { ...scope.headers, ...intent } } as never)),
    { onSuccess: () => void invalidate('paths') },
  );
}

export function useUnlockNode(courseId: string, pathId: string) {
  const { api, scope } = useLmsContext();
  return useIdempotentMutation<void, { nodeId: string; studentId: string; reason: string }>((v, intent) =>
    api.call<void>(
      api.client.POST('/lms/courses/{course_id}/paths/{path_id}/nodes/{node_id}/students/{student_id}:unlock', {
        params: { path: { course_id: courseId, path_id: pathId, node_id: v.nodeId, student_id: v.studentId } },
        body: { reason: v.reason },
        headers: { ...scope.headers, ...intent },
      } as never),
    ),
  );
}
