// useCurriculum (F053 T050, F053-AC-26…AC-30): curriculum API of the tree — listCmsSubjects, listCmsCurricula,
// listCurriculumVersions, listCmsTopics, listCmsObjectives (reads, cache keys with the session scope); updateTopic and
// update/createObjective with If-Match (`W/"<row_version>"` of the list item, contract delta of F053); createTopic,
// createObjective, createCurriculum (Idempotency-Key); curriculum version workflow (transitionCurriculumVersion,
// publishCurriculumVersion — If-Match + Idempotency-Key). Lists feed dropdowns and the tree: every cursor page is read
// (fetchAllPages, owner rule 2026-11-22), never a single page.
import { useQueries, useQuery } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { components } from '../../../shared/api/schema.gen';
import { fetchAllAsPage } from '../../../shared/api/fetchAllPages';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { withEtag } from '../questions/useQuestion';

export type Subject = components['schemas']['Subject'];
export type Curriculum = components['schemas']['Curriculum'];
export type CurriculumVersion = components['schemas']['CurriculumVersion'];
export type Topic = components['schemas']['Topic'];
export type LearningObjective = components['schemas']['LearningObjective'];
export type LearningObjectiveInput = components['schemas']['LearningObjectiveInput'];
export type TopicPatch = components['schemas']['TopicPatch'];


/** If-Match of a list item (row_version, F053 contract delta); none when the server does not send it yet. */
export const ifMatchOf = (rowVersion: number | undefined): Record<string, string> => (rowVersion ? { 'If-Match': `W/"${rowVersion}"` } : {});

const key = (scope: string, ...rest: unknown[]) => ['content-studio', ...rest.slice(0, 1), scope, ...rest.slice(1)] as const;

export function useSubjects() {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({ queryKey: key(scopeKey, 'subjects'), queryFn: () => fetchAllAsPage<Subject>((query) => api.call(api.client.GET('/cms/learning/subjects', { params: { query } }))) });
}

export function useCurricula(enabled = true) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({ queryKey: key(scopeKey, 'curricula'), queryFn: () => fetchAllAsPage<Curriculum>((query) => api.call(api.client.GET('/cms/learning/curricula', { params: { query } }))), enabled });
}

/** Current curriculum version picked for a subject × grade (khối). */
export interface CurrentVersionPick {
  curriculum: Curriculum;
  version: CurriculumVersion;
}

/**
 * F053 T066 (OQ F053-9): the current version (`Curriculum.current_version`, the published pointer) whose
 * `subject_grades` covers the subject and, when one grade is known, that grade. The tenant's own curriculum wins over
 * a platform one assigned to the tenant; otherwise the first in the listCmsCurricula order. Pure (tested alone).
 */
export function pickCurrentVersion(curricula: readonly Curriculum[], versions: readonly CurriculumVersion[], subjectId: string, grade?: number): CurrentVersionPick | undefined {
  const matches: CurrentVersionPick[] = [];
  for (const c of curricula) {
    const currentId = c.current_version?.curriculum_version_id;
    const v = currentId ? versions.find((x) => x.curriculum_version_id === currentId) : undefined;
    if (v?.subject_grades?.some((sg) => sg.subject_id === subjectId && (grade === undefined || sg.grade_level === grade))) matches.push({ curriculum: c, version: v });
  }
  return matches.find((m) => m.curriculum.owner_scope === 'tenant') ?? matches[0];
}

/**
 * Topics of the current curriculum version for subject × grade (F053 T066): listCmsCurricula, then
 * getCurriculumVersion of each current version (its subject_grades), then listCmsTopics with the inferred
 * curriculum_version_id (required by the contract). `none` = settled without a current version: the picker explains it.
 */
export function useCurrentTopics(subjectId: string | undefined, grade: number | undefined, enabled = true) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  const on = enabled && !!subjectId;
  const curricula = useCurricula(on);
  const current = (curricula.data?.data ?? []).filter((c) => !!c.current_version);
  const versions = useQueries({
    queries: current.map((c) => ({
      queryKey: key(scopeKey, 'curriculum-version', c.curriculum_id, c.current_version!.version),
      queryFn: () =>
        api.call<CurriculumVersion>(api.client.GET('/cms/learning/curricula/{curriculum_id}/versions/{version}', { params: { path: { curriculum_id: c.curriculum_id, version: c.current_version!.version } } })),
      enabled: on,
    })),
  });
  const loaded = versions.flatMap((q) => (q.data ? [q.data] : []));
  const settled = curricula.isFetched && versions.every((q) => q.isFetched || q.isError);
  const pick = on ? pickCurrentVersion(current, loaded, subjectId!, grade) : undefined;
  const vid = pick?.version.curriculum_version_id;
  const topics = useTopics({ curriculum_version_id: vid, subject_id: subjectId, ...(grade ? { grade_level: grade } : {}) }, on && !!vid);
  return { topics, pick, none: on && settled && !pick, loading: on && (!settled || topics.isFetching) };
}

/** Help line of a topic picker (T066): the curriculum version the topics come from, or why there are none. */
export function topicPickerHelp(t: (k: string, o?: Record<string, unknown>) => string, current: ReturnType<typeof useCurrentTopics>, lang: string): string | undefined {
  if (current.none) return t('content_studio.topicPicker.none');
  if (!current.pick) return undefined;
  const n = current.pick.curriculum.name as { vi?: string; en?: string };
  const name = (lang.startsWith('en') ? (n.en ?? n.vi) : (n.vi ?? n.en)) ?? current.pick.curriculum.code;
  return t('content_studio.topicPicker.from', { name, version: current.pick.version.version_label });
}

export function useCurriculumVersions(curriculumId: string | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: key(scopeKey, 'curriculum-versions', curriculumId),
    queryFn: () =>
      fetchAllAsPage<CurriculumVersion>((query) => api.call(api.client.GET('/cms/learning/curricula/{curriculum_id}/versions', { params: { path: { curriculum_id: curriculumId! }, query } }))),
    enabled: !!curriculumId,
  });
}

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
      fetchAllAsPage<Topic>((page) => api.call(api.client.GET('/cms/learning/topics', { params: { query: { ...filters, curriculum_version_id: version!, ...page } } }))),
    enabled: enabled && !!version,
  });
}

export function useObjectives(filters: { curriculum_version_id?: string; topic_id?: string; subject_id?: string; grade_level?: number }, enabled = true) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: key(scopeKey, 'objectives', filters),
    queryFn: () => fetchAllAsPage<LearningObjective>((page) => api.call(api.client.GET('/cms/learning/objectives', { params: { query: { ...filters, ...page } } }))),
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
