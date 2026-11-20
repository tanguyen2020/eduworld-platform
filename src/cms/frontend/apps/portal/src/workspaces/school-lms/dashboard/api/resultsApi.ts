// Results and analysis hooks (F056 T031; F056-AC-12, F056-AC-13): getCourseResults (graded assignments of a course,
// self-scored `score_10`, cursor pages ≤ dashboard_page_size_max) and getExamSessionReport (score distribution, median,
// pass rate, p-value per item without `rapid_guess`, misconceptions, reliability) of the exam session behind a graded
// assignment (`Assignment.exam_session_id`). Numbers are shown as returned (AGENTS R1).
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import type { Schemas } from '../../common/api';
import { DASHBOARD_CONFIG } from '../model/limits';
import { dashKey, useDashboardContext } from './dashboardApi';

export type CourseResults = Schemas['CourseResults'];
export type ExamSessionReport = Schemas['ExamSessionReport'];
export type ExamItemQuality = Schemas['ExamItemQuality'];

export function useCourseResults(courseId: string | undefined) {
  const { api, scope, queue } = useDashboardContext();
  const q = useInfiniteQuery({
    queryKey: dashKey('course-results', scope, courseId),
    queryFn: ({ pageParam }) =>
      queue.run(() =>
        api.call<CourseResults>(
          api.client.GET('/lms/courses/{course_id}/results', {
            params: { path: { course_id: courseId! }, query: { limit: DASHBOARD_CONFIG.dashboard_page_size_max, ...(pageParam ? { cursor: pageParam } : {}) } },
            headers: scope.headers,
          } as never),
        ),
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last: CourseResults) => (last.page?.has_more && last.page.next_cursor ? last.page.next_cursor : undefined),
    enabled: scope.ready && !!courseId,
  });
  return {
    columns: q.data?.pages[0]?.columns ?? [],
    rows: q.data?.pages.flatMap((p) => p.rows) ?? [],
    hasMore: !!q.hasNextPage,
    loadMore: () => void q.fetchNextPage(),
    isLoading: q.isPending && !!courseId,
    isFetching: q.isFetching,
    error: q.error,
  };
}

export function useExamSessionReport(examSessionId: string | undefined) {
  const { api, scope, queue } = useDashboardContext();
  return useQuery({
    queryKey: dashKey('exam-report', scope, examSessionId),
    queryFn: () =>
      queue.run(() => api.call<ExamSessionReport>(api.client.GET('/exams/sessions/{exam_session_id}/report', { params: { path: { exam_session_id: examSessionId! } }, headers: scope.headers } as never))),
    enabled: scope.ready && !!examSessionId,
  });
}
