// useTranslationJobs (F053-AC-25, ADMIN_PORTAL §1.5): export of the fields to translate (XLIFF 2.1 / CSV,
// Idempotency-Key), import of a translated file (uploaded through the media presign flow, then
// importLearningTranslations — a new Draft is created, mismatching keys are reported), and polling of the job with
// getLearningTranslationJob until succeeded / failed. Requires `learning_content.translate`.
import { useQuery } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { components } from '../../../shared/api/schema.gen';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { TRANSLATION_JOB_POLL_MS } from '../shared/config';

export type TranslationJob = components['schemas']['TranslationJob'];
export type TranslationExportRequest = components['schemas']['TranslationExportRequest'];
export type TranslationImportRequest = components['schemas']['TranslationImportRequest'];

const running = (s: TranslationJob['status'] | undefined) => s === 'queued' || s === 'running';

export function useTranslationJob(jobId: string | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: ['content-studio', 'translation-job', scopeKey, jobId],
    queryFn: () => api.call<TranslationJob>(api.client.GET('/cms/learning/translations/jobs/{job_id}', { params: { path: { job_id: jobId! } } })),
    enabled: !!jobId,
    refetchInterval: (q) => (running(q.state.data?.status) ? TRANSLATION_JOB_POLL_MS : false),
  });
}

export function useTranslationActions() {
  const { api } = usePortalRuntime();
  const exportJob = useIdempotentMutation((body: TranslationExportRequest, intent) => api.call<TranslationJob>(api.client.POST('/cms/learning/translations:export', { body, headers: intent as never })));
  const importJob = useIdempotentMutation((body: TranslationImportRequest, intent) => api.call<TranslationJob>(api.client.POST('/cms/learning/translations:import', { body, headers: intent as never })));
  return { exportJob, importJob };
}
