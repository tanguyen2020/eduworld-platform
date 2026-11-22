// useActivity (F058 T023, F058-AC-02, AC-05, AC-07, AC-28): the CMS activity API — listActivityTypes,
// listCmsActivities, getCmsActivity (+ ETag), createCmsActivity (Idempotency-Key, owner chosen by a System Admin),
// updateCmsActivityDraft (full ActivityDefinitionInput + If-Match), createCmsActivityVersion via useRelease. Nothing
// is kept in browser storage.
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { components } from '../../../shared/api/schema.gen';
import { useCursorPages } from '../../../shared/api/useCursorPages';
import type { IntentHeaders } from '../../../shared/api/useIdempotentMutation';
import { withEtag, type Loaded } from '../questions/useQuestion';
import type { ActivityDefinition, ActivityDefinitionInput, ActivityType, ActivityTypeInfo } from './activityModel';

export const activityKey = (scope: string, id: string) => ['content-studio', 'activity', scope, id] as const;

export function useActivityTypes() {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: ['content-studio', 'activity-types', scopeKey],
    queryFn: () => api.call<{ data: ActivityTypeInfo[] }>(api.client.GET('/cms/learning/activity-types', {})),
    staleTime: 5 * 60_000,
  });
}

export interface ActivityListFilter {
  activity_type?: ActivityType;
  status?: components['schemas']['ContentStatus'];
}

export function useActivityList(f: ActivityListFilter) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  // A table: cursor pages, "Tải thêm" (owner rule 2026-11-22).
  return useCursorPages<ActivityDefinition>(['content-studio', 'list', scopeKey, 'activities', f], (cursor) =>
    api.call(api.client.GET('/cms/learning/activities', { params: { query: { ...f, ...(cursor ? { cursor } : {}) } } })),
  );
}

export function useActivityQuery(id: string | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: activityKey(scopeKey, id ?? 'new'),
    queryFn: () => withEtag<ActivityDefinition>(api, api.client.GET('/cms/learning/activities/{activity_id}', { params: { path: { activity_id: id! } } })),
    enabled: !!id,
    staleTime: Infinity,
  });
}

export function useActivityApi() {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  const qc = useQueryClient();
  return {
    create: (body: ActivityDefinitionInput & Record<string, unknown>, headers: IntentHeaders & Record<string, string>) =>
      withEtag<ActivityDefinition>(api, api.client.POST('/cms/learning/activities', { body, headers } as never)),
    saveDraft: (id: string, body: ActivityDefinitionInput, etag: string | null) =>
      withEtag<ActivityDefinition>(
        api,
        api.client.PATCH('/cms/learning/activities/{activity_id}', {
          params: { path: { activity_id: id } },
          body,
          headers: (etag ? { 'If-Match': etag } : {}) as never,
        } as never),
      ),
    remember: (id: string, loaded: Loaded<ActivityDefinition>) => qc.setQueryData(activityKey(scopeKey, id), loaded),
  };
}
