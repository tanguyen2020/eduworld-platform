// Inactive-student cleanup (F052 T043, F052-AC-21, ADR-0032): previewTenantStudentCleanup and exclusions
// (createStudentCleanupExclusions with an Idempotency-Key per intent; deleteStudentCleanupExclusions). Cleanup itself
// is a server job (soft delete, R24); nothing is deleted from the portal.
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { PAGE_SIZE, qk, useApi, usePagedList, type Schemas } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';

export type CleanupCandidate = Schemas['StudentCleanupCandidate'];
export type ExclusionRequest = Schemas['CleanupExclusionRequest'];

export function useCleanupPreview() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return usePagedList<CleanupCandidate>(
    qk('cleanup-preview', scope),
    async (cursor) => {
      const res = await api.call(api.client.GET('/tenant/student-cleanup/preview', { params: { query: { limit: PAGE_SIZE, ...(cursor ? { cursor } : {}) } }, headers: scope.headers }));
      return { items: res.data, page: res.page };
    },
    scope.ready,
  );
}

function useInvalidate() {
  const scope = useSchoolLmsScope();
  const qc = useQueryClient();
  return () => void qc.invalidateQueries({ queryKey: ['school-lms', 'cleanup-preview', scope.key] });
}

export function useExcludeStudents() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const onSuccess = useInvalidate();
  return useIdempotentMutation<unknown, ExclusionRequest>((body, intent) => api.call(api.client.POST('/tenant/student-cleanup/exclusions', { body, headers: { ...scope.headers, ...intent } })), { onSuccess });
}

export function useRemoveExclusions() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const onSuccess = useInvalidate();
  return useMutation({
    mutationFn: (body: ExclusionRequest) => api.call(api.client.DELETE('/tenant/student-cleanup/exclusions', { body, headers: scope.headers })),
    onSuccess,
  });
}
