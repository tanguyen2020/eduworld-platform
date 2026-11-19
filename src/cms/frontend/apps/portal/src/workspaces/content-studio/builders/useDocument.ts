// useDocument (F054 T048–T050, F054-AC-20…AC-23): the authoring API of the versioned learning documents edited by
// the builders — question set, lesson, assessment blueprint: get (latest version + ETag of that version), create
// (Idempotency-Key, owner chosen by a System Admin), draft PATCH (merge patch + If-Match), pool stats per rule
// (getQuestionSetPoolStats / getAssessmentBlueprintPoolStats). The body may hold answer-related settings only as the
// contract serves them to the authoring view; nothing is kept in browser storage.
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { components } from '../../../shared/api/schema.gen';
import type { IntentHeaders } from '../../../shared/api/useIdempotentMutation';
import { withEtag, type Loaded } from '../questions/useQuestion';
import { idParamOf } from '../release/useRelease';
import { CONTENT_TYPES } from '../shared/contentTypes';

export type DocType = 'question_set' | 'lesson' | 'assessment_blueprint';
export type QuestionSetAuthoring = components['schemas']['QuestionSetAuthoring'];
export type LessonAuthoring = components['schemas']['LessonAuthoring'];
export type BlueprintAuthoring = components['schemas']['AssessmentBlueprintAuthoring'];
export type PoolStats = components['schemas']['PoolStats'];
export type DocAuthoring = QuestionSetAuthoring | LessonAuthoring | BlueprintAuthoring;

export const docKey = (scope: string, type: DocType, id: string) => ['content-studio', type, scope, id] as const;

const base = (type: DocType) => CONTENT_TYPES[type].base!;
const idPath = (type: DocType, id: string) => ({ [idParamOf(type)]: id });

export function useDocumentQuery<T extends DocAuthoring>(type: DocType, id: string | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: docKey(scopeKey, type, id ?? 'new'),
    queryFn: () => withEtag<T>(api, api.client.GET(`${base(type)}/{${idParamOf(type)}}` as never, { params: { path: idPath(type, id!) } } as never)),
    enabled: !!id,
    staleTime: Infinity,
  });
}

export function usePoolStats(type: 'question_set' | 'assessment_blueprint', id: string | undefined, version: number | undefined, enabled = true) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: ['content-studio', 'pool-stats', scopeKey, type, id, version],
    queryFn: () => api.call<PoolStats>(api.client.GET(`${base(type)}/{${idParamOf(type)}}/versions/{version}:pool-stats` as never, { params: { path: { ...idPath(type, id!), version } } } as never)),
    enabled: enabled && !!id && !!version,
  });
}

export function useDocumentApi<T extends DocAuthoring>(type: DocType) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  const qc = useQueryClient();
  return {
    create: (body: Record<string, unknown>, headers: IntentHeaders & Record<string, string>) => withEtag<T>(api, api.client.POST(base(type) as never, { body, headers } as never)),
    saveDraft: (id: string, version: number, body: Record<string, unknown>, etag: string | null) =>
      withEtag<T>(
        api,
        api.client.PATCH(`${base(type)}/{${idParamOf(type)}}/versions/{version}` as never, {
          params: { path: { ...idPath(type, id), version } },
          body,
          headers: { 'Content-Type': 'application/merge-patch+json', ...(etag ? { 'If-Match': etag } : {}) },
        } as never),
      ),
    remember: (id: string, loaded: Loaded<T>) => qc.setQueryData(docKey(scopeKey, type, id), loaded),
  };
}
