// Analysis hook (F056 T031; F056-AC-13): getExamSessionReport (score distribution, median, pass rate, p-value per item
// without `rapid_guess`, misconceptions, reliability) of the exam session behind a graded assignment
// (`Assignment.exam_session_id`). Numbers are shown as returned (AGENTS R1). The course results table (getCourseResults)
// is gone with the courses (ADR-0095): the results screen lists assignments (listAssignments) instead.
import { useQuery } from '@tanstack/react-query';
import type { Schemas } from '../../common/api';
import { dashKey, useDashboardContext } from './dashboardApi';

export type ExamSessionReport = Schemas['ExamSessionReport'];
export type ExamItemQuality = Schemas['ExamItemQuality'];

export function useExamSessionReport(examSessionId: string | undefined) {
  const { api, scope, queue } = useDashboardContext();
  return useQuery({
    queryKey: dashKey('exam-report', scope, examSessionId),
    queryFn: () =>
      queue.run(() => api.call<ExamSessionReport>(api.client.GET('/exams/sessions/{exam_session_id}/report', { params: { path: { exam_session_id: examSessionId! } }, headers: scope.headers } as never))),
    enabled: scope.ready && !!examSessionId,
  });
}
