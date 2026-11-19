// Inactive-student cleanup hooks (F057 T037, F057-AC-14/15, ADR-0032): platform settings (GET → ETag, PUT with
// If-Match and a mandatory reason) and the cleanup runs of every tenant (cursor, `tenant_ids[]`); a run over the
// threshold is approved (approveStudentCleanupRun) or cancelled (cancelStudentCleanupRun, reason 3..500 → `cancelled`,
// nobody cleaned — OQ F057-2 resolved 2026-11-20), both with Idempotency-Key and X-Tenant-Id = tenant of the run.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { pageQuery, useCursorList, usePlatformApi, type CursorPage, type Schemas } from './common';

export type StudentCleanupSettings = Schemas['StudentCleanupSettings'];
export type StudentCleanupRun = Schemas['StudentCleanupRun'];

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

/** cancelStudentCleanupRun: a run `awaiting_approval` → `cancelled` (204; 409 when it is no longer awaiting approval). */
export function useCancelRun() {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ run, reason }: { run: Pick<StudentCleanupRun, 'run_id' | 'tenant_id'>; reason: string }, headers) =>
      api.call(
        api.client.POST('/admin/student-cleanup/runs/{run_id}:cancel', {
          params: { path: { run_id: run.run_id } },
          body: { reason },
          headers: { ...headers, ...scope.writeHeaders(run.tenant_id) },
        }),
      ),
    { onSuccess: () => qc.invalidateQueries({ queryKey: ['platform', 'cleanup-runs'] }) },
  );
}
