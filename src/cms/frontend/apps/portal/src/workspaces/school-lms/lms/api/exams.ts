// Exam session hooks of an assignment with `exam_session_id` (F055 T037, T048; F055-AC-18, AC-23): getExamSession
// (accommodations, status; the apiClient remembers its ETag), listExamSessionPapers (question versions of the locked
// papers — the questions a regrade can target), putExamAccommodations (delta F055-D2: Idempotency-Key + If-Match +
// reason; the server recomputes each student's deadline).
import { useQuery } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../../shared/api/useIdempotentMutation';
import { lmsKey, useLmsContext, withEtag, type S } from './lmsCommon';

export type ExamSession = S['ExamSession'];
export type ExamAccommodation = S['ExamAccommodation'];
export type ExamPaper = S['ExamPaper'];

/** The session with its ETag (If-Match of putExamAccommodations, a sub-resource the apiClient cache does not map). */
export function useExamSession(id: string | undefined) {
  const { api, scope } = useLmsContext();
  return useQuery({
    queryKey: lmsKey('exam-session', scope, id),
    queryFn: () => withEtag<ExamSession>(api, api.client.GET('/exams/sessions/{exam_session_id}', { params: { path: { exam_session_id: id! } }, headers: scope.headers } as never) as never),
    enabled: scope.ready && !!id,
  });
}

export function useExamPapers(id: string | undefined) {
  const { api, scope } = useLmsContext();
  return useQuery({
    queryKey: lmsKey('exam-papers', scope, id),
    queryFn: () => api.call<{ data: ExamPaper[] }>(api.client.GET('/exams/sessions/{exam_session_id}/papers', { params: { path: { exam_session_id: id! } }, headers: scope.headers } as never)),
    enabled: scope.ready && !!id,
  });
}

export function usePutAccommodations(id: string) {
  const { api, scope, qc } = useLmsContext();
  return useIdempotentMutation<{ data: ExamSession; etag: string | null }, { accommodations: ExamAccommodation[]; reason: string; etag: string | null }>(
    ({ etag, ...body }, intent) =>
      withEtag<ExamSession>(
        api,
        api.client.PUT('/exams/sessions/{exam_session_id}/accommodations', {
          params: { path: { exam_session_id: id } },
          body,
          headers: { ...scope.headers, ...(etag ? { 'If-Match': etag } : {}), ...intent },
        } as never) as never,
      ),
    { onSuccess: (r) => qc.setQueryData(lmsKey('exam-session', scope, id), r) },
  );
}
