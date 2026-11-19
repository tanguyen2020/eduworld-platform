// Sync conflicts of a school (F052 T026, F052-AC-10, F052-AC-11): listSchoolReconciliationIssues and
// resolveSchoolReconciliationIssue (Idempotency-Key per intent, reused on retry; reason required).
import { useQueryClient } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { PAGE_SIZE, qk, useApi, usePagedList, type Schemas } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';

export type ReconciliationIssue = Schemas['SisReconciliationIssue'];
export type IssueType = Schemas['SisReconciliationIssueType'];
export type IssueStatus = ReconciliationIssue['status'];
export type ResolveBody = Schemas['SchoolReconciliationResolve'];

export function useReconciliationIssues(schoolId: string | undefined, filters: { issueType?: IssueType; status?: IssueStatus }) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return usePagedList<ReconciliationIssue>(
    qk('recon-issues', scope, schoolId, filters.issueType ?? '', filters.status ?? ''),
    async (cursor) => {
      const res = await api.call(
        api.client.GET('/schools/{school_id}/reconciliation-issues', {
          params: {
            path: { school_id: schoolId! },
            query: { limit: PAGE_SIZE, ...(cursor ? { cursor } : {}), ...(filters.issueType ? { issue_type: filters.issueType } : {}), ...(filters.status ? { status: filters.status } : {}) },
          },
          headers: scope.headers,
        }),
      );
      return { items: res.data, page: res.page };
    },
    scope.ready && !!schoolId,
  );
}

export function useResolveIssue(schoolId: string) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const qc = useQueryClient();
  return useIdempotentMutation<ReconciliationIssue, { issueId: string; body: ResolveBody }>(
    (v, intent) =>
      api.call(
        api.client.POST('/schools/{school_id}/reconciliation-issues/{issue_id}:resolve', {
          params: { path: { school_id: schoolId, issue_id: v.issueId } },
          body: v.body,
          headers: { ...scope.headers, ...intent },
        }),
      ),
    { onSuccess: () => void qc.invalidateQueries({ queryKey: ['school-lms', 'recon-issues', scope.key] }) },
  );
}
