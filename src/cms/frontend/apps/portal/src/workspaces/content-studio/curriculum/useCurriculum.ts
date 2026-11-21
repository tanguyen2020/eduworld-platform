// useCurriculum (F053 T050, F053-AC-26…AC-30): curriculum API of the tree — listCmsSubjects, listCmsCurricula,
// listCurriculumVersions, listCmsTopics, listCmsObjectives (reads, cache keys with the session scope); updateTopic and
// update/createObjective with If-Match (`W/"<row_version>"` of the list item, contract delta of F053); createTopic,
// createObjective, createCurriculum (Idempotency-Key); curriculum version workflow (transitionCurriculumVersion,
// publishCurriculumVersion — If-Match + Idempotency-Key).
import { useQuery } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { components } from '../../../shared/api/schema.gen';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { withEtag } from '../questions/useQuestion';

export type Subject = components['schemas']['Subject'];
export type Curriculum = components['schemas']['Curriculum'];
export type CurriculumVersion = components['schemas']['CurriculumVersion'];
export type Topic = components['schemas']['Topic'];
export type LearningObjective = components['schemas']['LearningObjective'];
export type LearningObjectiveInput = components['schemas']['LearningObjectiveInput'];
export type TopicPatch = components['schemas']['TopicPatch'];

type Page<T> = { data: T[] };

/** If-Match of a list item (row_version, F053 contract delta); none when the server does not send it yet. */
export const ifMatchOf = (rowVersion: number | undefined): Record<string, string> => (rowVersion ? { 'If-Match': `W/"${rowVersion}"` } : {});

const key = (scope: string, ...rest: unknown[]) => ['content-studio', ...rest.slice(0, 1), scope, ...rest.slice(1)] as const;

export function useSubjects() {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({ queryKey: key(scopeKey, 'subjects'), queryFn: () => api.call<Page<Subject>>(api.client.GET('/cms/learning/subjects', { params: { query: { limit: 100 } as never } })) });
}

export function useCurricula() {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({ queryKey: key(scopeKey, 'curricula'), queryFn: () => api.call<Page<Curriculum>>(api.client.GET('/cms/learning/curricula', { params: { query: { limit: 100 } as never } })) });
}

export function useCurriculumVersions(curriculumId: string | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: key(scopeKey, 'curriculum-versions', curriculumId),
    queryFn: () => api.call<Page<CurriculumVersion>>(api.client.GET('/cms/learning/curricula/{curriculum_id}/versions', { params: { path: { curriculum_id: curriculumId! }, query: { limit: 50 } as never } })),
    enabled: !!curriculumId,
  });
}

/** components/parameters/Limit of the contract: 1–100 (a larger limit is 422 VALIDATION_FAILED). */
export const LIST_LIMIT_MAX = 100;

/**
 * listCmsTopics: curriculum_version_id is required by the contract, so nothing is requested without it (a picker that
 * only knows the subject gets no topics — OQ F053-9, F053 Clarifications 2026-11-21).
 */
export function useTopics(filters: { curriculum_version_id?: string; subject_id?: string; grade_level?: number }, enabled = true) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  const version = filters.curriculum_version_id;
  return useQuery({
    queryKey: key(scopeKey, 'topics', filters),
    queryFn: () =>
      api.call<Page<Topic>>(api.client.GET('/cms/learning/topics', { params: { query: { ...filters, curriculum_version_id: version!, limit: LIST_LIMIT_MAX } } })),
    enabled: enabled && !!version,
  });
}

export function useObjectives(filters: { curriculum_version_id?: string; topic_id?: string; subject_id?: string; grade_level?: number }, enabled = true) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: key(scopeKey, 'objectives', filters),
    queryFn: () => api.call<Page<LearningObjective>>(api.client.GET('/cms/learning/objectives', { params: { query: { ...filters, limit: LIST_LIMIT_MAX } } })),
    enabled: enabled && !!(filters.curriculum_version_id || filters.topic_id || filters.subject_id),
  });
}

export function useCurriculumApi() {
  const { api } = usePortalRuntime();
  const updateTopic = (topic: Topic, patch: TopicPatch) =>
    api.call<Topic>(api.client.PATCH('/cms/learning/topics/{topic_id}', { params: { path: { topic_id: topic.topic_id } }, body: patch, headers: { 'Content-Type': 'application/merge-patch+json', ...ifMatchOf((topic as Topic & { row_version?: number }).row_version) } as never }));
  const createTopic = useIdempotentMutation((body: components['schemas']['TopicInput'], intent) => api.call<Topic>(api.client.POST('/cms/learning/topics', { body, headers: intent as never })));
  const createObjective = useIdempotentMutation((body: LearningObjectiveInput, intent) => api.call<LearningObjective>(api.client.POST('/cms/learning/objectives', { body, headers: intent as never })));
  const updateObjective = (lo: LearningObjective, patch: components['schemas']['LearningObjectivePatch']) =>
    api.call<LearningObjective>(api.client.PATCH('/cms/learning/objectives/{objective_id}', { params: { path: { objective_id: lo.objective_id } }, body: patch, headers: { 'Content-Type': 'application/merge-patch+json', ...ifMatchOf((lo as LearningObjective & { row_version?: number }).row_version) } as never }));
  const createCurriculum = useIdempotentMutation((v: { body: components['schemas']['CurriculumInput']; headers: Record<string, string> }, intent) =>
    api.call<Curriculum>(api.client.POST('/cms/learning/curricula', { body: v.body, headers: { ...intent, ...v.headers } as never })),
  );
  const loadVersion = (curriculumId: string, version: number) =>
    withEtag<CurriculumVersion>(api, api.client.GET('/cms/learning/curricula/{curriculum_id}/versions/{version}', { params: { path: { curriculum_id: curriculumId, version } } }));
  const transitionVersion = useIdempotentMutation((v: { curriculumId: string; version: number; action: 'submit_review' | 'approve' | 'reject'; note?: string; etag: string | null }, intent) =>
    api.call<CurriculumVersion>(
      api.client.POST('/cms/learning/curricula/{curriculum_id}/versions/{version}:transition', {
        params: { path: { curriculum_id: v.curriculumId, version: v.version } },
        body: { action: v.action, ...(v.note ? { note: v.note } : {}) },
        headers: { ...intent, ...(v.etag ? { 'If-Match': v.etag } : {}) } as never,
      }),
    ),
  );
  const publishVersion = useIdempotentMutation((v: { curriculumId: string; version: number; etag: string | null }, intent) =>
    api.call<CurriculumVersion>(
      api.client.POST('/cms/learning/curricula/{curriculum_id}/versions/{version}:publish', {
        params: { path: { curriculum_id: v.curriculumId, version: v.version } },
        headers: { ...intent, ...(v.etag ? { 'If-Match': v.etag } : {}) } as never,
      }),
    ),
  );
  return { updateTopic, createTopic, createObjective, updateObjective, createCurriculum, loadVersion, transitionVersion, publishVersion };
}
