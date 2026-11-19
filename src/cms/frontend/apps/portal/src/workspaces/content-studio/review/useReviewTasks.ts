// useReviewTasks / useReviewDecision (F054-AC-12…AC-15, ADMIN_PORTAL §4): the review queue (listContentReviewTasks
// by lane / content_type / "giao cho tôi"; `tenant_ids` only for the System Admin filter — a tenant reviewer never
// sends it and only gets what the API returns) and the decision on one version (transitionContentVersion approve /
// reject with If-Match + Idempotency-Key). Bulk approval sends one intent per item (its own Idempotency-Key).
import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { ApiClient } from '../../../shared/api/client';
import { INTENT_HEADER } from '../../../shared/api/middleware/idempotency';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { versionEtag, type ItemRef } from '../release/useRelease';
import type { ContentType } from '../shared/contentTypes';
import type { ReviewTask } from './reviewRules';

export interface ReviewFilters {
  lane?: 'tenant' | 'platform_learning';
  contentType?: ContentType;
  mine?: boolean;
  status?: 'open' | 'approved' | 'rejected' | 'cancelled';
}

type Page = { data: ReviewTask[]; page?: { has_more?: boolean; next_cursor?: string | null } };

export function useReviewTasks(f: ReviewFilters) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  const scope = useCrossTenantScope();
  const query = scope.listQuery({
    ...(f.lane ? { lane: f.lane } : {}),
    ...(f.contentType ? { content_type: f.contentType } : {}),
    ...(f.mine ? { assigned_to_me: true } : {}),
    status: f.status ?? 'open',
    limit: 50,
  });
  return useInfiniteQuery({
    queryKey: ['content-studio', 'review-tasks', scopeKey, query],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => api.call<Page>(api.client.GET('/cms/learning/review-tasks', { params: { query: { ...query, ...(pageParam ? { cursor: pageParam } : {}) } as never } })),
    getNextPageParam: (p) => (p.page?.has_more ? (p.page.next_cursor ?? undefined) : undefined),
  });
}

export interface DecisionVars extends ItemRef {
  version: number;
  action: 'approve' | 'reject';
  note?: string;
  audioExemptionAcknowledged?: boolean;
}

/** One approve / reject (If-Match of a fresh read of the version). */
export function decide(api: ApiClient, v: DecisionVars, intent: Record<string, string>) {
  return versionEtag(api, v, v.version).then((etag) =>
    api.call(
      api.client.POST('/cms/learning/content/{content_type}/{content_id}/versions/{version}:transition', {
        params: { path: { content_type: v.contentType, content_id: v.contentId, version: v.version } },
        body: { action: v.action, ...(v.note ? { note: v.note } : {}), ...(v.audioExemptionAcknowledged ? { audio_exemption_acknowledged: true } : {}) },
        headers: { ...intent, ...(etag ? { 'If-Match': etag } : {}) } as never,
      }),
    ),
  );
}

export function useReviewDecision() {
  const { api } = usePortalRuntime();
  const qc = useQueryClient();
  return useIdempotentMutation((v: DecisionVars, intent) => decide(api, v, intent), { onSuccess: () => qc.invalidateQueries({ queryKey: ['content-studio'] }) });
}

/** Intent header of one item of a bulk run: retrying the run reuses each item's Idempotency-Key (F054-AC-14). */
export const bulkIntent = (runId: string, t: Pick<ReviewTask, 'content_type' | 'content_id' | 'version'>) => ({ [INTENT_HEADER]: `bulk-approve:${runId}:${t.content_type}:${t.content_id}:${t.version}` });
