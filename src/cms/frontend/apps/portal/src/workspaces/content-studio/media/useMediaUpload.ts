// useMediaUpload (F053-AC-05, F027, ADR-0044): upload of a new asset — sha256 of the file in the browser →
// presignMediaUpload (Idempotency-Key; license required, alt_text required for images) → PUT of the file to the
// presigned quarantine URL (returned by the server, never configured here) → completeMediaUpload → the asset is then
// polled with getMediaAsset until `ready` or `rejected`. A file already in the scope (same sha256) is reused
// (`duplicate_of`, no upload).
import { useQuery } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import { useTenantSource } from '../../../shared/auth/TenantContext';
import type { components } from '../../../shared/api/schema.gen';
import { INTENT_HEADER } from '../../../shared/api/middleware/idempotency';
import { useCursorPages } from '../../../shared/api/useCursorPages';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { MEDIA_POLL_MS } from '../shared/config';

export type MediaAsset = components['schemas']['MediaAsset'];
export type MediaKind = components['schemas']['MediaKind'];
export type MediaStatus = components['schemas']['MediaStatus'];

export interface UploadInput {
  file: File;
  kind: MediaKind;
  license: string;
  altText?: string;
  isExamMaterial?: boolean;
  /** System Admin target (owner + X-Tenant-Id), from OwnerScopeField. */
  owner?: { body: Record<string, unknown>; headers: Record<string, string> };
}

/** Bytes of a file (FileReader fallback for engines without Blob.arrayBuffer). */
function bytesOf(file: Blob): Promise<ArrayBuffer> {
  if (typeof file.arrayBuffer === 'function') return file.arrayBuffer();
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => resolve(r.result as ArrayBuffer);
    r.onerror = () => reject(r.error);
    r.readAsArrayBuffer(file);
  });
}

export async function sha256OfFile(file: Blob): Promise<string> {
  // copy into this realm's typed array (a test DOM may hand out a foreign ArrayBuffer)
  const bytes = Uint8Array.from(new Uint8Array(await bytesOf(file)));
  const digest = await crypto.subtle.digest('SHA-256', bytes);
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('');
}

export const mediaAssetKey = (scope: string, id: string) => ['content-studio', 'media-asset', scope, id] as const;
export const mediaListKey = (scope: string, filters: Record<string, unknown>) => ['content-studio', 'media', scope, filters] as const;

const pending = (s: MediaStatus | undefined) => s === 'quarantined' || s === 'processing';

/** One asset, polled while it is in the pipeline. */
export function useMediaAsset(id: string | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: mediaAssetKey(scopeKey, id ?? ''),
    queryFn: () => api.call<MediaAsset>(api.client.GET('/cms/media/{asset_id}', { params: { path: { asset_id: id! } } })),
    enabled: !!id,
    refetchInterval: (q) => (pending(q.state.data?.status) ? MEDIA_POLL_MS : false),
  });
}

export function useMediaList(kind: MediaKind, q: string, enabled: boolean) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  // A list of rows: cursor pages, "Tải thêm" (owner rule 2026-11-22).
  return useCursorPages<MediaAsset>(
    mediaListKey(scopeKey, { kind, q }),
    (cursor) => api.call(api.client.GET('/cms/media', { params: { query: { kind, ...(q ? { q } : {}), ...(cursor ? { cursor } : {}) } } })),
    { enabled },
  );
}

export function useMediaUpload() {
  const { api } = usePortalRuntime();
  const tenant = useTenantSource();
  return useIdempotentMutation(async (input: UploadInput & { sha256: string }, intent) => {
    const presigned = await api.call(
      api.client.POST('/cms/media:presign-upload', {
        body: {
          kind: input.kind,
          filename: input.file.name.slice(0, 255),
          mime_type: (input.file.type || 'application/octet-stream').slice(0, 127),
          size_bytes: input.file.size,
          sha256: input.sha256,
          license: input.license,
          ...(input.altText ? { alt_text: input.altText } : {}),
          is_exam_material: !!input.isExamMaterial,
          // owner: the System Admin's choice, else the session scope (tenant staff → tenant, platform staff → platform)
          owner_scope: (input.owner?.body.owner_scope as 'platform' | 'tenant' | undefined) ?? (tenant.getTenantId() ? 'tenant' : 'platform'),
          ...(input.owner?.body.tenant_id ? { tenant_id: input.owner.body.tenant_id as string } : {}),
        },
        headers: { ...intent, ...(input.owner?.headers ?? {}) } as never,
      }),
    );
    if (presigned.duplicate_of || !presigned.upload_url) return presigned.asset_id;
    const put = await fetch(presigned.upload_url, { method: 'PUT', body: input.file, headers: presigned.upload_headers ?? {} });
    if (!put.ok) throw new Error(`upload failed: ${put.status}`);
    await api.call(api.client.POST('/cms/media/{asset_id}:complete-upload', { params: { path: { asset_id: presigned.asset_id } }, headers: { [INTENT_HEADER]: `${intent[INTENT_HEADER]}:complete`, ...(input.owner?.headers ?? {}) } as never }));
    return presigned.asset_id;
  });
}
