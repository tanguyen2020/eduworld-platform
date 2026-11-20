// Assignment hooks (F055 T016, T025; F055-AC-05, AC-08…AC-12): listAssignments, getAssignment, createAssignment +
// publishAssignment (one user intent: each call keeps its own Idempotency-Key across retries and publish sends the
// If-Match of the created draft), updateAssignment (If-Match), cancel / release-results / archive / remind, per-student
// extend / excuse, and the submission read model (cursor). Server decides everything; these hooks only call the API.
import { useMutation, useQuery } from '@tanstack/react-query';
import { useRef } from 'react';
import { INTENT_HEADER } from '../../../../shared/api/middleware/idempotency';
import { useIdempotentMutation } from '../../../../shared/api/useIdempotentMutation';
import { LMS_PAGE_SIZE, lmsKey, MERGE_PATCH, useDataList, useLmsContext, withEtag, type DataPage, type S } from './lmsCommon';

export type Assignment = S['Assignment'];
export type AssignmentSubmission = S['AssignmentSubmission'];
export type AssignmentStatus = S['AssignmentStatus'];

export interface AssignmentFilters {
  status?: AssignmentStatus;
  courseId?: string;
  classId?: string;
}

export function useAssignments(f: AssignmentFilters = {}) {
  const { api, scope } = useLmsContext();
  return useDataList<Assignment>(
    lmsKey('assignments', scope, f.status ?? '', f.courseId ?? '', f.classId ?? ''),
    (cursor) =>
      api.call<DataPage<Assignment>>(
        api.client.GET('/lms/assignments', {
          params: {
            query: { limit: LMS_PAGE_SIZE, ...(cursor ? { cursor } : {}), ...(f.status ? { status: f.status } : {}), ...(f.courseId ? { course_id: f.courseId } : {}), ...(f.classId ? { class_id: f.classId } : {}) },
          },
          headers: scope.headers,
        } as never),
      ),
    scope.ready,
  );
}

export function useAssignment(id: string | undefined) {
  const { api, scope } = useLmsContext();
  return useQuery({
    queryKey: lmsKey('assignment', scope, id),
    queryFn: () => api.call<Assignment>(api.client.GET('/lms/assignments/{assignment_id}', { params: { path: { assignment_id: id! } }, headers: scope.headers } as never)),
    enabled: scope.ready && !!id,
  });
}

export function useSubmissions(id: string | undefined, status?: S['AssignmentSubmissionStatus']) {
  const { api, scope } = useLmsContext();
  return useDataList<AssignmentSubmission>(
    lmsKey('submissions', scope, id, status ?? ''),
    (cursor) =>
      api.call<DataPage<AssignmentSubmission>>(
        api.client.GET('/lms/assignments/{assignment_id}/submissions', {
          params: { path: { assignment_id: id! }, query: { limit: LMS_PAGE_SIZE, ...(cursor ? { cursor } : {}), ...(status ? { status } : {}) } },
          headers: scope.headers,
        } as never),
      ),
    scope.ready && !!id,
  );
}

/**
 * "Giao bài" = createAssignment then publishAssignment (F055-AC-05). One intent per form content: a retry after a
 * network error reuses both Idempotency-Keys and skips the create that already succeeded, so a double click or a
 * retry never creates two assignments.
 */
export function useCreateAndPublishAssignment() {
  const { api, scope, invalidate } = useLmsContext();
  const created = useRef<{ intent: string; id: string; etag: string | null } | null>(null);
  return useIdempotentMutation<Assignment, S['AssignmentCreate']>(
    async (body, intent) => {
      const id = intent[INTENT_HEADER];
      if (created.current?.intent !== id) {
        const r = await withEtag(api, api.client.POST('/lms/assignments', { body, headers: { ...scope.headers, [INTENT_HEADER]: `${id}:create` } } as never) as never);
        created.current = { intent: id, id: (r.data as Assignment).assignment_id, etag: r.etag };
      }
      const draft = created.current;
      return api.call<Assignment>(
        api.client.POST('/lms/assignments/{assignment_id}:publish', {
          params: { path: { assignment_id: draft.id } },
          headers: { ...scope.headers, ...(draft.etag ? { 'If-Match': draft.etag } : {}), [INTENT_HEADER]: `${id}:publish` },
        } as never),
      );
    },
    {
      onSuccess: () => {
        created.current = null;
        void invalidate('assignments');
      },
    },
  );
}

export function useUpdateAssignment(id: string) {
  const { api, scope, invalidate } = useLmsContext();
  return useMutation({
    mutationFn: (body: S['AssignmentUpdate']) =>
      api.call<Assignment>(api.client.PATCH('/lms/assignments/{assignment_id}', { params: { path: { assignment_id: id } }, body, headers: { ...scope.headers, ...MERGE_PATCH } } as never)),
    onSettled: () => {
      void invalidate('assignment');
      void invalidate('assignments');
    },
  });
}

type ActionOp = 'cancel' | 'release-results' | 'archive';

/** cancel (reason) / release-results / archive of one assignment — Idempotency-Key per intent. */
export function useAssignmentAction(id: string, op: ActionOp) {
  const { api, scope, invalidate } = useLmsContext();
  return useIdempotentMutation<Assignment, { reason?: string }>(
    (v, intent) =>
      api.call<Assignment>(
        api.client.POST(`/lms/assignments/{assignment_id}:${op}` as '/lms/assignments/{assignment_id}:cancel', {
          params: { path: { assignment_id: id } },
          ...(op === 'cancel' ? { body: { reason: v.reason ?? '' } } : {}),
          headers: { ...scope.headers, ...intent },
        } as never),
      ),
    {
      onSuccess: () => {
        void invalidate('assignment');
        void invalidate('assignments');
      },
    },
  );
}

export function useRemindAssignment(id: string) {
  const { api, scope } = useLmsContext();
  return useIdempotentMutation<{ recipients: number }, { at: string }>((_v, intent) =>
    api.call<{ recipients: number }>(api.client.POST('/lms/assignments/{assignment_id}:remind', { params: { path: { assignment_id: id } }, headers: { ...scope.headers, ...intent } } as never)),
  );
}

export interface StudentActionVars {
  studentId: string;
  reason: string;
  dueAt?: string;
}

/** extendAssignmentForStudent / excuseAssignmentForStudent → the updated AssignmentSubmission (F055-AC-09). */
export function useStudentAction(id: string, op: 'extend' | 'excuse') {
  const { api, scope, qc } = useLmsContext();
  return useIdempotentMutation<AssignmentSubmission, StudentActionVars>(
    (v, intent) =>
      api.call<AssignmentSubmission>(
        api.client.POST(`/lms/assignments/{assignment_id}/students/{student_id}:${op}` as '/lms/assignments/{assignment_id}/students/{student_id}:extend', {
          params: { path: { assignment_id: id, student_id: v.studentId } },
          body: op === 'extend' ? { due_at: v.dueAt, reason: v.reason } : { reason: v.reason },
          headers: { ...scope.headers, ...intent },
        } as never),
      ),
    {
      onSuccess: (row) => {
        // update the row in place from the response (no reload of the whole table)
        qc.setQueriesData<{ pages: DataPage<AssignmentSubmission>[] }>({ queryKey: ['school-lms', 'lms', 'submissions', scope.key, id] }, (old) =>
          old ? { ...old, pages: old.pages.map((p) => ({ ...p, data: p.data.map((s) => (s.student_id === row.student_id ? { ...s, ...row } : s)) })) } : old,
        );
      },
    },
  );
}
