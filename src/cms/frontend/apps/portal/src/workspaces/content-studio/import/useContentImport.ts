// useContentImport (F054-AC-27, FR-QB-011, ADR-0044): upload of the normalized CSV as a `document` media asset
// (presign → PUT → complete, Idempotency-Key) waited until `ready`, then createContentImport (`dry_run` first,
// Idempotency-Key per run) and getContentImport polled until `succeeded` / `failed`. Nothing is created before the
// user starts the real run (SC-003, SC-005: one intent = one import).
import { usePortalRuntime } from '../../../app/runtime';
import type { components } from '../../../shared/api/schema.gen';
import { INTENT_HEADER } from '../../../shared/api/middleware/idempotency';
import { sha256OfFile, useMediaUpload, type MediaAsset } from '../media/useMediaUpload';
import { MEDIA_POLL_MS, TRANSLATION_JOB_POLL_MS } from '../shared/config';

export type ContentImportJob = components['schemas']['ContentImportJob'];

/** Server cap of `ContentImportJob.errors` (maxItems): a full list means it was cut. */
export const IMPORT_ERRORS_MAX = 500;
/** Polls before giving up waiting (asset pipeline, import job) [Proposed]. */
export const IMPORT_MAX_POLLS = 150;

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

export function useContentImport(opts: { pollMs?: number } = {}) {
  const { api } = usePortalRuntime();
  const upload = useMediaUpload();
  const poll = opts.pollMs;

  /** Uploads the CSV and waits for the pipeline; returns the asset id. */
  const uploadFile = async (file: File, license: string): Promise<string> => {
    const sha256 = await sha256OfFile(file);
    const assetId = await upload.mutateAsync({ file, kind: 'document', license, sha256 });
    for (let i = 0; i < IMPORT_MAX_POLLS; i++) {
      const a = await api.call<MediaAsset>(api.client.GET('/cms/media/{asset_id}', { params: { path: { asset_id: assetId } } }));
      if (a.status === 'ready') return assetId;
      if (a.status === 'rejected') throw Object.assign(new Error('MEDIA_REJECTED'), { reason: a.reject_reason });
      await sleep(poll ?? MEDIA_POLL_MS);
    }
    throw new Error('MEDIA_TIMEOUT');
  };

  /** Starts an import run (dry run or real) and waits for its report. */
  const run = async (assetId: string, dryRun: boolean): Promise<ContentImportJob> => {
    const job = await api.call<ContentImportJob>(
      api.client.POST('/cms/learning/imports', { body: { asset_id: assetId, kind: 'questions_csv', dry_run: dryRun }, headers: { [INTENT_HEADER]: `import:${assetId}:${dryRun ? 'dry' : 'run'}` } as never }),
    );
    let current = job;
    for (let i = 0; i < IMPORT_MAX_POLLS && (current.status === 'queued' || current.status === 'running'); i++) {
      await sleep(poll ?? TRANSLATION_JOB_POLL_MS);
      current = await api.call<ContentImportJob>(api.client.GET('/cms/learning/imports/{import_id}', { params: { path: { import_id: job.import_id } } }));
    }
    return current;
  };

  return { uploadFile, run };
}
