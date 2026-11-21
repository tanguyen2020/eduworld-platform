// Clone hooks (F057 T024, F057-AC-07/08, ADR-0029, AGENTS R22): createTenantClone (Idempotency-Key, 202 job) and
// getTenantClone polled until a final status. `previewTenantClone` does not exist yet (delta owner F045, Câu hỏi mở
// F057-Q1): the wizard skips the dependency preview step and says so.
import { useQuery } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { JOB_POLL_INTERVAL_MS } from '../limits';
import { isFinalJob, usePlatformApi, type ResourceType, type Schemas } from './common';

export type TenantCloneRequest = Schemas['TenantCloneRequest'];
export type TenantCloneJob = Schemas['TenantCloneJob'];

/**
 * Types the wizard offers: configuration and content only (ADMIN_PORTAL §3.7 step 1) — never student data (`user`,
 * `student`, enrollments, attempts, results, chat, mastery have no entry here). The backend allowlist
 * (platformadmin.Clonable) stays the authority and answers 422 for anything else.
 */
export const CLONABLE_TYPES: readonly ResourceType[] = [
  'curriculum', 'lesson', 'question', 'question_set', 'assessment_blueprint', 'media', 'assignment',
  'challenge_template', 'school_settings', 'tenant_settings', // no class_template in Release 1.0 (OQ F045-4)
];

/** previewTenantClone is not in the contract yet (F057 T005 pending with owner F045). */
export const CLONE_PREVIEW_AVAILABLE = false;

export function useCreateClone() {
  const api = usePlatformApi();
  return useIdempotentMutation((body: TenantCloneRequest, headers) => api.call(api.client.POST('/admin/tenant-clones', { body, headers })));
}

export function useCloneJob(jobId: string | undefined) {
  const api = usePlatformApi();
  return useQuery({
    queryKey: ['platform', 'clone-job', jobId],
    queryFn: () => api.call(api.client.GET('/admin/tenant-clones/{job_id}', { params: { path: { job_id: jobId! } } })),
    enabled: !!jobId,
    refetchInterval: (q) => (q.state.status === 'error' || isFinalJob(q.state.data?.status) ? false : JOB_POLL_INTERVAL_MS),
  });
}
