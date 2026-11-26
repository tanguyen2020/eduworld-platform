// API of "Dịch nội dung game" (F082 T029, F082-AC-12…AC-15): exportGameTranslations / importGameTranslations (one
// Idempotency-Key per user intent, kept on retry — a dry run and the real import are two intents), getGameTranslationJob
// followed while queued/running, and the upload of the translated file as game media `kind = document`
// (presignGameMediaUpload → PUT with the signed headers, never the token → getGameMediaAsset until the server check ends).
// No rule here: the server validates the file, the keys and the texts (AGENTS R1).
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { usePortalRuntime } from '../../../../app/runtime';
import { useSession } from '../../../../session/SessionProvider';
import { INTENT_HEADER } from '../../../../shared/api/middleware/idempotency';
import type { components } from '../../../../shared/api/schema.gen';
import { useIdempotentMutation } from '../../../../shared/api/useIdempotentMutation';
import { sha256OfFile } from '../../../../shared/files/sha256';

export type TranslationJob = components['schemas']['TranslationJob'];
export type TranslationExportRequest = components['schemas']['TranslationExportRequest'];
export type TranslationImportRequest = components['schemas']['TranslationImportRequest'];
type MediaAsset = components['schemas']['MediaAsset'];
type MediaPresignResponse = components['schemas']['MediaPresignResponse'];

/** UI refresh of a running job / file check [Proposed]: a job is manual and short (≤ 2 min for 500 items, plan.md). */
export const TRANSLATION_POLL_MS = 2_000;

/** TranslationExportRequest bounds (OpenAPI maxItems). */
export const EXPORT_LIMITS = { contentIds: 500, contentTypes: 20 } as const;

/** Format of a translated file by its extension (XLIFF 2.1 or CSV). */
export function formatOf(name: string): 'xliff' | 'csv' | null {
  const ext = name.toLowerCase().split('.').pop();
  return ext === 'xlf' || ext === 'xliff' ? 'xliff' : ext === 'csv' ? 'csv' : null;
}

const MIME = { xliff: 'application/xliff+xml', csv: 'text/csv' } as const;

/** License of an internal translation file (MediaPresignRequest.license is required). */
const DOCUMENT_LICENSE = 'internal';

const pending = (status?: string) => status === 'queued' || status === 'running';

export function useTranslationJob(jobId: string | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: ['game-ops', 'translation-job', scopeKey, jobId],
    queryFn: () => api.call<TranslationJob>(api.client.GET('/game-admin/translations/jobs/{job_id}', { params: { path: { job_id: jobId! } } })),
    enabled: !!jobId,
    refetchInterval: (q) => (!q.state.data || pending((q.state.data as TranslationJob).status) ? TRANSLATION_POLL_MS : false),
  });
}

export function useTranslationWrites() {
  const { api } = usePortalRuntime();
  const exportJob = useIdempotentMutation((body: TranslationExportRequest, intent) =>
    api.call<TranslationJob>(api.client.POST('/game-admin/translations:export', { body, headers: intent as never })),
  );
  const importJob = useIdempotentMutation((body: TranslationImportRequest, intent) =>
    api.call<TranslationJob>(api.client.POST('/game-admin/translations:import', { body, headers: intent as never })),
  );
  return { exportJob, importJob };
}

/** Upload of a translated file: presign (document) → PUT → server check followed until ready / rejected. */
export function useDocumentUpload() {
  const { api, upload } = usePortalRuntime();
  const { scopeKey } = useSession();
  const [assetId, setAssetId] = useState<string>();
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const asset = useQuery({
    queryKey: ['game-ops', 'translation-file', scopeKey, assetId],
    queryFn: () => api.call<MediaAsset>(api.client.GET('/game-media/{asset_id}', { params: { path: { asset_id: assetId! } } })),
    enabled: !!assetId,
    refetchInterval: (q) => (!q.state.data || ((q.state.data as MediaAsset).status !== 'ready' && (q.state.data as MediaAsset).status !== 'rejected') ? TRANSLATION_POLL_MS : false),
  });

  const start = async (file: File) => {
    const format = formatOf(file.name);
    setAssetId(undefined);
    setError(null);
    if (!format) return;
    setBusy(true);
    try {
      const sha256 = await sha256OfFile(file);
      const r = await api.call<MediaPresignResponse>(
        api.client.POST('/game-media:presign-upload', {
          body: { kind: 'document', filename: file.name, mime_type: MIME[format], size_bytes: file.size, sha256, license: DOCUMENT_LICENSE, is_exam_material: false, owner_scope: 'platform' },
          // one intent per file: a retry of the same file reuses its Idempotency-Key
          headers: { [INTENT_HEADER]: `${file.name}:${file.size}:${sha256}` } as never,
        }),
      );
      if (r.upload_url && !r.duplicate_of) {
        const put = await (upload ?? fetch)(r.upload_url, { method: 'PUT', body: file, headers: { ...(r.upload_headers ?? {}), 'Content-Type': MIME[format] } });
        if (!put.ok) throw new Error(`HTTP ${put.status}`);
      }
      setAssetId(r.duplicate_of ?? r.asset_id);
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  const status = asset.data?.status;
  const state: 'idle' | 'uploading' | 'ready' | 'error' = error || status === 'rejected' || asset.error ? 'error' : status === 'ready' ? 'ready' : busy || assetId ? 'uploading' : 'idle';
  const reset = () => (setAssetId(undefined), setError(null));
  return { start, reset, assetId, state, error: error ?? asset.error, rejectReason: asset.data?.reject_reason };
}
