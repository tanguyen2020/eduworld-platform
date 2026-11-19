// usePreview (F054-AC-10, AC-22, ADR-0031, ADMIN_PORTAL §5): a short-lived preview session of any version
// (createLearningPreviewSession: render payload WITHOUT answers, no attempt / learning event / XP) per display
// language, created only when the user asks for it, and the try-out scoring (submitPreviewResponse: scored by the
// server, nothing stored). The session lives `preview_token_ttl` (30 min, CONFIG_DEFAULTS); it is not refetched.
import { useMutation, useQuery } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { components } from '../../../shared/api/schema.gen';
import type { ItemRef } from '../release/useRelease';
import type { QuestionResponse } from './tryOutResponse';

export type PreviewSession = components['schemas']['LearningPreviewSession'];
export type PreviewResult = components['schemas']['PreviewResponseResult'];
export type QuestionRender = components['schemas']['QuestionRender'];

export type PreviewLang = 'vi' | 'en';
export const LOCALE_OF: Record<PreviewLang, 'vi-VN' | 'en-US'> = { vi: 'vi-VN', en: 'en-US' };

export const previewKey = (scope: string, r: ItemRef, version: number, lang: PreviewLang) => ['content-studio', 'preview', scope, r.contentType, r.contentId, version, lang] as const;

export function usePreviewSession(r: ItemRef, version: number, lang: PreviewLang, enabled: boolean) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: previewKey(scopeKey, r, version, lang),
    queryFn: () =>
      api.call<PreviewSession>(
        api.client.POST('/cms/learning/preview/sessions', { body: { content_type: r.contentType, content_id: r.contentId, version, display_language: LOCALE_OF[lang] } }),
      ),
    enabled,
    staleTime: Infinity,
    gcTime: 30 * 60_000,
    retry: false,
  });
}

export function useTryOut(sessionId: string | undefined) {
  const { api } = usePortalRuntime();
  return useMutation({
    mutationFn: (v: { position: number; response: QuestionResponse }) =>
      api.call<PreviewResult>(
        api.client.POST('/cms/learning/preview/sessions/{preview_session_id}/responses', { params: { path: { preview_session_id: sessionId! } }, body: { position: v.position, response: v.response } }),
      ),
  });
}
