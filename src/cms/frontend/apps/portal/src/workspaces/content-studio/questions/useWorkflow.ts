// useWorkflow (F053-AC-09, AC-10, CONTENT_MODEL §6, ADR-0043): workflow of ONE content version — transition
// (submit_review / approve / reject, If-Match, Idempotency-Key, note required to reject) and publish now
// (Idempotency-Key, change_note). One intent per user action: clicking "Xuất bản ngay" twice reuses the key, so the
// server creates one release only (F051 useIdempotentMutation).
import { usePortalRuntime } from '../../../app/runtime';
import type { components } from '../../../shared/api/schema.gen';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { withEtag } from './useQuestion';

export type ContentType = components['schemas']['LearningContentType'];
export type ContentVersion = components['schemas']['ContentVersion'];
export type ContentRelease = components['schemas']['ContentRelease'];

export interface TransitionVars {
  contentType: ContentType;
  contentId: string;
  version: number;
  action: 'submit_review' | 'approve' | 'reject';
  note?: string;
  etag: string | null;
}

export interface PublishVars {
  contentType: ContentType;
  contentId: string;
  version: number;
  changeNote?: string;
  etag: string | null;
}

export function useWorkflow() {
  const { api } = usePortalRuntime();
  const transition = useIdempotentMutation((v: TransitionVars, intent) =>
    withEtag<ContentVersion>(
      api,
      api.client.POST('/cms/learning/content/{content_type}/{content_id}/versions/{version}:transition', {
        params: { path: { content_type: v.contentType, content_id: v.contentId, version: v.version } },
        body: { action: v.action, ...(v.note ? { note: v.note } : {}) },
        headers: { ...intent, ...(v.etag ? { 'If-Match': v.etag } : {}) } as never,
      }),
    ),
  );
  const publish = useIdempotentMutation((v: PublishVars, intent) =>
    withEtag<ContentRelease>(
      api,
      api.client.POST('/cms/learning/content/{content_type}/{content_id}/versions/{version}:publish', {
        params: { path: { content_type: v.contentType, content_id: v.contentId, version: v.version } },
        body: v.changeNote ? { change_note: v.changeNote } : {},
        headers: { ...intent, ...(v.etag ? { 'If-Match': v.etag } : {}) } as never,
      }),
    ),
  );
  return { transition, publish };
}
