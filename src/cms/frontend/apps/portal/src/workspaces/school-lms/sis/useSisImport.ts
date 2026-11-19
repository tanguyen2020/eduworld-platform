// CSV import of SIS data (F052 T024, F052-AC-08, F052-AC-09, F052-AC-11, SIS_INTEGRATION §6):
//   check file → sha256 in the browser → createSisImport (Idempotency-Key) → PUT the file to `upload_url` before
//   `upload_expires_at` (plain fetch, NO Authorization header: presigned URL) → poll getSisImport until a final status.
// One Idempotency-Key per intent (useIdempotentMutation): clicking twice or retrying after a network error with the
// same file reuses it, so the server creates one import; after a successful create the intent is forgotten, so an
// expired upload starts a new import with a new key. Server-side rules (columns, duplicates) stay on the server.
import { useQuery } from '@tanstack/react-query';
import { useCallback, useState } from 'react';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { qk, useApi, type Schemas } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';
import { checkCsvFile, sha256Hex, type FileProblem } from './fileCheck';

export type SisImport = Schemas['SisImport'];
export type SisImportCreate = Schemas['SisImportCreate'];
export type EntityKind = SisImportCreate['entity_kinds'][number];
export type ImportPhase = 'idle' | 'hashing' | 'creating' | 'uploading' | 'tracking' | 'failed';
export type ImportProblem = FileProblem | 'upload_expired' | 'upload_failed' | 'api';

export const ACTIVE_STATUSES: readonly SisImport['status'][] = ['awaiting_upload', 'queued', 'validating', 'applying'];
export const isFinal = (s: SisImport['status'] | undefined) => !!s && !ACTIVE_STATUSES.includes(s);

/** Poll period: 2 s growing ×1.5 up to 15 s (data-model §2: "chu kỳ tăng dần"). */
export function pollDelay(attempt: number): number {
  return Math.min(15_000, Math.round(2_000 * 1.5 ** Math.max(0, attempt)));
}

export class ImportError extends Error {
  constructor(
    readonly problem: ImportProblem,
    override readonly cause?: unknown,
  ) {
    super(problem);
  }
}

export interface ImportDeps {
  create: (body: SisImportCreate) => Promise<SisImport>;
  upload: (url: string, file: File) => Promise<void>;
  hash: (file: File) => Promise<string>;
  now: () => number;
  onPhase?: (p: ImportPhase) => void;
}

/** The import flow without React (unit-tested): returns the created import once the file is uploaded. */
export async function runImport(deps: ImportDeps, file: File, kind: EntityKind, dryRun: boolean): Promise<SisImport> {
  const problem = checkCsvFile(file);
  if (problem) throw new ImportError(problem);
  deps.onPhase?.('hashing');
  const sha256 = await deps.hash(file);
  deps.onPhase?.('creating');
  let created: SisImport;
  try {
    created = await deps.create({ file_name: file.name, size_bytes: file.size, sha256, entity_kinds: [kind], dry_run: dryRun });
  } catch (e) {
    throw new ImportError('api', e);
  }
  if (!created.upload_url) return created;
  if (created.upload_expires_at && deps.now() >= Date.parse(created.upload_expires_at)) throw new ImportError('upload_expired');
  deps.onPhase?.('uploading');
  try {
    await deps.upload(created.upload_url, file);
  } catch (e) {
    throw new ImportError('upload_failed', e);
  }
  return created;
}

/** PUT to the presigned URL: no Authorization, no tenant header (object storage, not the API). */
export async function uploadToPresignedUrl(url: string, file: File, fetchImpl: typeof fetch = (...a) => globalThis.fetch(...a)): Promise<void> {
  const res = await fetchImpl(url, { method: 'PUT', body: file, headers: { 'Content-Type': 'text/csv' }, credentials: 'omit' });
  if (!res.ok) throw new Error(`upload ${res.status}`);
}

export interface SisImportState {
  phase: ImportPhase;
  importId?: string;
  problem?: ImportProblem;
  error?: unknown;
}

export function useSisImport(schoolId: string | undefined, opts: { upload?: ImportDeps['upload']; now?: () => number } = {}) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const [state, setState] = useState<SisImportState>({ phase: 'idle' });
  const create = useIdempotentMutation<SisImport, { schoolId: string; body: SisImportCreate }>((v, intent) =>
    api.call(api.client.POST('/schools/{school_id}/sis-imports', { params: { path: { school_id: v.schoolId } }, body: v.body, headers: { ...scope.headers, ...intent } })),
  );
  const tracking = useQuery({
    queryKey: qk('sis-import', scope, schoolId, state.importId),
    queryFn: () => api.call(api.client.GET('/schools/{school_id}/sis-imports/{import_id}', { params: { path: { school_id: schoolId!, import_id: state.importId! } }, headers: scope.headers })),
    enabled: scope.ready && !!schoolId && !!state.importId,
    refetchInterval: (q) => (isFinal(q.state.data?.status) || q.state.error ? false : pollDelay(q.state.dataUpdateCount)),
  });
  const { mutateAsync } = create;
  const upload = opts.upload ?? ((url: string, file: File) => uploadToPresignedUrl(url, file));
  const now = opts.now ?? Date.now;
  const start = useCallback(
    async (file: File, kind: EntityKind, dryRun: boolean) => {
      if (!schoolId) return;
      try {
        const created = await runImport(
          { create: (body) => mutateAsync({ schoolId, body }), upload, hash: (f) => sha256Hex(f), now, onPhase: (phase) => setState((s) => ({ ...s, phase, problem: undefined, error: undefined })) },
          file,
          kind,
          dryRun,
        );
        setState({ phase: 'tracking', importId: created.import_id });
      } catch (e) {
        const err = e instanceof ImportError ? e : new ImportError('api', e);
        setState({ phase: 'failed', problem: err.problem, error: err.cause });
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps -- upload/now are stable per screen
    [schoolId, mutateAsync],
  );
  const reset = useCallback(() => setState({ phase: 'idle' }), []);
  const busy = state.phase === 'hashing' || state.phase === 'creating' || state.phase === 'uploading';
  return { state, start, reset, busy, current: tracking.data, trackingError: tracking.error };
}
