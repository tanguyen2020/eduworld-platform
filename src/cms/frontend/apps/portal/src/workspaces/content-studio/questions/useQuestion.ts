// useQuestion (F053 T032, plan "application"): the CMS question API of the editor — getCmsQuestion (with its ETag),
// createQuestion (Idempotency-Key), updateQuestionDraft (merge patch + If-Match = last ETag), createQuestionVersion
// (Idempotency-Key). Query key ['content-studio','question', scopeKey, id] (data-model §1). The authoring body holds
// the answer key: it lives only in memory / the TanStack cache of this session, never in browser storage.
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { QuestionDraft } from '@eduworld/ops-ui/content';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { ApiClient } from '../../../shared/api/client';
import type { components } from '../../../shared/api/schema.gen';
import type { IntentHeaders } from '../../../shared/api/useIdempotentMutation';

export type QuestionAuthoring = components['schemas']['QuestionAuthoring'];
export type QuestionAuthoringInput = components['schemas']['QuestionAuthoringInput'];

export interface Loaded<T> {
  data: T;
  etag: string | null;
}

/** Unwraps an openapi-fetch result (ApiError on failure) and keeps the ETag header. */
export async function withEtag<T>(api: ApiClient, p: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<Loaded<T>> {
  const r = await p;
  const data = await api.call(Promise.resolve(r));
  return { data, etag: r.response.headers.get('ETag') };
}

export const questionKey = (scope: string, id: string) => ['content-studio', 'question', scope, id] as const;

/** Question metadata edited next to the QuestionEditor (QuestionAuthoringInput fields outside the draft). */
export interface QuestionMeta {
  grade_levels?: number[];
  subject_id?: string;
  topic_id?: string;
  objective_ids?: string[];
  difficulty?: number;
  content_language: 'vi' | 'en';
  tags?: string[];
  audio_exempt_reason?: string;
  worked_example_ref?: { content_item_id: string; block_id: string };
  change_note?: string;
  /** F035: efficacy instrument (platform scope only); undefined = never touched, not sent. */
  efficacy_instrument?: boolean;
}

/** Server copy → editor state. */
export function fromAuthoring(q: QuestionAuthoring): { draft: QuestionDraft; meta: QuestionMeta } {
  const x = q as QuestionAuthoring & Record<string, unknown>;
  return {
    draft: {
      type: q.type ?? 'single_choice',
      stem: (x.stem as unknown as QuestionDraft['stem']) ?? { blocks: [{ type: 'paragraph' }] },
      render_payload: (x.render_payload as Record<string, unknown>) ?? {},
      answer_key: (x.answer_key as Record<string, unknown>) ?? {},
      scoring_rule: (x.scoring_rule as unknown as QuestionDraft['scoring_rule']) ?? { mode: 'all_or_nothing' },
      explanation: x.explanation as unknown as QuestionDraft['explanation'],
      hints: (x.hints as unknown as QuestionDraft['hints']) ?? [],
    },
    meta: {
      grade_levels: q.grade_levels,
      subject_id: q.subject_id,
      topic_id: q.topic_id,
      objective_ids: q.objective_ids,
      difficulty: q.difficulty,
      content_language: q.content_language ?? 'vi',
      tags: q.tags,
      audio_exempt_reason: q.audio_exempt_reason,
      worked_example_ref: q.worked_example_ref,
      change_note: q.change_note,
      ...(q.efficacy_instrument ? { efficacy_instrument: true } : {}),
    },
  };
}

/**
 * Editor state → authoring body. Optional fields the author cleared are sent as `null` (JSON merge patch removes
 * them); `math_assets` are the formulas rendered in the browser (ADR-0055).
 */
export function toAuthoringBody(draft: QuestionDraft, meta: QuestionMeta, mathAssets: QuestionAuthoringInput['math_assets'], mediaRefs: string[]): Record<string, unknown> {
  return {
    type: draft.type,
    stem: draft.stem,
    render_payload: draft.render_payload,
    answer_key: draft.answer_key,
    scoring_rule: draft.scoring_rule,
    explanation: draft.explanation ?? null,
    hints: draft.hints ?? [],
    math_assets: mathAssets ?? [],
    media_refs: mediaRefs,
    grade_levels: meta.grade_levels,
    subject_id: meta.subject_id,
    topic_id: meta.topic_id,
    objective_ids: meta.objective_ids ?? [],
    difficulty: meta.difficulty,
    content_language: meta.content_language,
    tags: meta.tags ?? [],
    audio_exempt_reason: meta.audio_exempt_reason?.trim() ? meta.audio_exempt_reason.trim() : null,
    worked_example_ref: meta.worked_example_ref ?? null,
    change_note: meta.change_note?.trim() ? meta.change_note.trim() : null,
    // F035-AC-01: an efficacy question is exam-only; the flag is sent only once the author touched it.
    ...(meta.efficacy_instrument === undefined ? {} : { efficacy_instrument: meta.efficacy_instrument, ...(meta.efficacy_instrument ? { exam_only: true } : {}) }),
  };
}

export function useCmsQuestion(id: string | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: questionKey(scopeKey, id ?? 'new'),
    // the question resource gives the latest version; the ETag for draft PATCH / workflow is the VERSION's
    // (getCmsQuestionVersion), the question's own ETag only guards deleteQuestion
    queryFn: async () => {
      const q = await api.call<QuestionAuthoring>(api.client.GET('/cms/learning/questions/{question_id}', { params: { path: { question_id: id! } } }));
      return withEtag<QuestionAuthoring>(api, api.client.GET('/cms/learning/questions/{question_id}/versions/{version}', { params: { path: { question_id: id!, version: q.version } } }));
    },
    enabled: !!id,
    staleTime: Infinity,
  });
}

export function useQuestionApi() {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  const qc = useQueryClient();
  return {
    create: (body: Record<string, unknown>, headers: IntentHeaders & Record<string, string>) =>
      withEtag<QuestionAuthoring>(api, api.client.POST('/cms/learning/questions', { body: body as QuestionAuthoringInput, headers: headers as never })),
    saveDraft: (id: string, version: number, body: Record<string, unknown>, etag: string | null) =>
      withEtag<QuestionAuthoring>(
        api,
        api.client.PATCH('/cms/learning/questions/{question_id}/versions/{version}', {
          params: { path: { question_id: id, version } },
          body: body as QuestionAuthoringInput,
          headers: { 'Content-Type': 'application/merge-patch+json', ...(etag ? { 'If-Match': etag } : {}) } as never,
        }),
      ),
    newVersion: (id: string, fromVersion: number, headers: IntentHeaders) =>
      withEtag<QuestionAuthoring>(api, api.client.POST('/cms/learning/questions/{question_id}/versions', { params: { path: { question_id: id } }, body: { from_version: fromVersion }, headers: headers as never })),
    /** Puts a server copy in the cache (after save / create / workflow). */
    remember: (id: string, loaded: Loaded<QuestionAuthoring>) => qc.setQueryData(questionKey(scopeKey, id), loaded),
    invalidate: (id: string) => qc.invalidateQueries({ queryKey: questionKey(scopeKey, id) }),
  };
}
