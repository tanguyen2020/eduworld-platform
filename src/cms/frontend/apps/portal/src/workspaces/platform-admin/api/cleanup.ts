// Inactive-student cleanup hooks (F057 T037, F057-AC-14/15, ADR-0032): platform settings (GET → ETag, PUT with
// If-Match and a mandatory reason) and the cleanup runs of every tenant (cursor, `tenant_ids[]`), approval of a run
// over the threshold (Idempotency-Key, X-Tenant-Id = tenant of the run). `cancelStudentCleanupRun` does not exist yet
// (delta owner F016, Câu hỏi mở F057-Q2): no Cancel button until it does.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { pageQuery, useCursorList, usePlatformApi, type CursorPage, type Schemas } from './common';

export type StudentCleanupSettings = Schemas['StudentCleanupSettings'];
export type StudentCleanupRun = Schemas['StudentCleanupRun'];

export const CLEANUP_CANCEL_AVAILABLE = false;

export function useCleanupSettings() {
  const api = usePlatformApi();
  return useQuery({ queryKey: ['platform', 'settings', 'student-cleanup'], queryFn: () => api.call(api.client.GET('/admin/settings/student-cleanup', {})) });
}

export function usePutCleanupSettings() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: StudentCleanupSettings) => api.call(api.client.PUT('/admin/settings/student-cleanup', { body })),
    onSuccess: (data) => qc.setQueryData(['platform', 'settings', 'student-cleanup'], data),
  });
}

export function useCleanupRuns() {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const query = scope.listQuery({});
  return useCursorList<StudentCleanupRun>(['platform', 'cleanup-runs', query], (cursor) =>
    api.call<CursorPage<StudentCleanupRun>>(api.client.GET('/admin/student-cleanup/runs', { params: { query: { ...query, ...pageQuery(cursor) } } })),
  );
}

export function useApproveRun() {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (run: Pick<StudentCleanupRun, 'run_id' | 'tenant_id'>, headers) =>
      api.call(
        api.client.POST('/admin/student-cleanup/runs/{run_id}:approve', {
          params: { path: { run_id: run.run_id } },
          headers: { ...headers, ...scope.writeHeaders(run.tenant_id) },
        }),
      ),
    { onSuccess: () => qc.invalidateQueries({ queryKey: ['platform', 'cleanup-runs'] }) },
  );
}
