// useRegrade (F058 T049, F058-AC-22, AC-23, F031 US4): system re-evaluation of a faulty version — impact preview
// (previewRegradeRequest, nothing written), create (createRegradeRequest, 202, one Idempotency-Key per intent kept on
// retry), status polled until completed / failed (getRegradeRequest), history of the version (listRegradeRequests).
// There is no manual grading anywhere (ADR-0072).
import { useQuery } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { components } from '../../../shared/api/schema.gen';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';

export type RegradeRequest = components['schemas']['RegradeRequest'];
export type RegradeCreate = components['schemas']['RegradeRequestCreate'];
export type RegradeScope = 'question_version' | 'activity_version';

/** Poll period of a queued / running request [Proposed], like the media pipeline (MEDIA_POLL_MS). */
export const REGRADE_POLL_MS = 2_000;

const versionQuery = (scope: RegradeScope, versionId: string) => (scope === 'question_version' ? { question_version_id: versionId } : { activity_version_id: versionId });

export function useRegradePreview(body: RegradeCreate | null) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: ['content-studio', 'regrade-preview', scopeKey, body],
    queryFn: () => api.call<{ affected_attempts: number }>(api.client.POST('/cms/learning/regrade-requests:preview', { body: body! })),
    enabled: !!body,
  });
}

export function useRegradeHistory(scope: RegradeScope, versionId: string | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: ['content-studio', 'regrades', scopeKey, scope, versionId],
    queryFn: () => api.call<{ data: RegradeRequest[] }>(api.client.GET('/cms/learning/regrade-requests', { params: { query: { ...versionQuery(scope, versionId!), limit: 50 } as never } })),
    enabled: !!versionId,
  });
}

export function useRegradeStatus(id: string | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: ['content-studio', 'regrade', scopeKey, id],
    queryFn: () => api.call<RegradeRequest>(api.client.GET('/cms/learning/regrade-requests/{regrade_request_id}', { params: { path: { regrade_request_id: id! } } })),
    enabled: !!id,
    refetchInterval: (q) => (q.state.data && (q.state.data.status === 'completed' || q.state.data.status === 'failed') ? false : REGRADE_POLL_MS),
  });
}

/** Published versions of the question of a version (listContentVersions): the corrected key of use_new_key. */
export function useCorrectedVersions(item: { contentType: string; contentId: string } | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: ['content-studio', 'regrade-versions', scopeKey, item],
    queryFn: () =>
      api.call<{ data: { version: number; status: string; content_version_id?: string }[] }>(
        api.client.GET('/cms/learning/content/{content_type}/{content_id}/versions', { params: { path: { content_type: item!.contentType, content_id: item!.contentId }, query: { status: 'published', limit: 100 } } } as never),
      ),
    enabled: !!item,
  });
}

export function useCreateRegrade() {
  const { api } = usePortalRuntime();
  return useIdempotentMutation((body: RegradeCreate, intent) => api.call<RegradeRequest>(api.client.POST('/cms/learning/regrade-requests', { body, headers: intent } as never)));
}
