// useChecksumUpload (F082 T012, F082-AC-03, ADR-0044): a queue of files uploaded through a presigned URL. For each file:
// sha256 computed in the browser (Web Crypto) → `presign` of the caller (the portal calls its presign operation with the
// `intent` of the file as Idempotency-Key — one intent per file, kept on retry) → PUT of the bytes to the presigned URL
// with plain `put` (never an Authorization header) → `onUploaded`. Progress is per file; a refused file keeps the server
// message (413 MEDIA_TOO_LARGE, 415 MEDIA_TYPE_NOT_ALLOWED, 422 VALIDATION_FAILED…) and can be retried or removed while the
// other files go on. No size or type limit here: the server decides and checks the bytes again (AGENTS R1). No API call
// of its own (ADMIN_PORTAL §1.1).
import { useCallback, useRef, useState } from 'react';

export type UploadStatus = 'hashing' | 'presigning' | 'uploading' | 'done' | 'error';

export interface UploadError {
  /** Stable server code (Problem `code`) when known. */
  code?: string;
  message: string;
  /** Problem trace id (shown with the error). */
  traceId?: string;
}

export interface UploadItem {
  id: string;
  name: string;
  file: File;
  status: UploadStatus;
  /** 0–100, by step (hash, presign, PUT). */
  progress: number;
  /** Intent id of this file: the Idempotency-Key of its presign, kept on retry. */
  intent: string;
  sha256?: string;
  error?: UploadError;
}

export interface PresignInput {
  intent: string;
  sha256: string;
  /** File name reduced to `[A-Za-z0-9._-]` (object key media/<sha256>/<file>). */
  fileName: string;
  contentType: string;
  size: number;
  file: File;
}

export interface PresignOutput<T> {
  /** Absent: nothing to upload (the server already holds the same file). */
  uploadUrl?: string;
  /** Headers the PUT must carry (from the presign response). */
  headers?: Record<string, string>;
  /** What the caller keeps for this file (asset key, asset id…). */
  result: T;
}

export interface UseChecksumUploadOptions<T> {
  presign: (input: PresignInput) => Promise<PresignOutput<T>>;
  /** Plain fetch used for the PUT (no token middleware). */
  put: (url: string, init: RequestInit) => Promise<Response>;
  onUploaded: (item: UploadItem, result: T) => void;
  /** Maps a thrown error to its message (the portal maps its Problem errors). */
  describeError?: (error: unknown) => UploadError;
}

const STEP = { hashing: 10, presigning: 35, uploading: 70, done: 100 } as const;

/** sha256 hex of a blob (Web Crypto). */
export async function sha256Hex(blob: Blob): Promise<string> {
  const buf =
    typeof blob.arrayBuffer === 'function'
      ? await blob.arrayBuffer()
      : await new Promise<ArrayBuffer>((resolve, reject) => {
          const r = new FileReader();
          r.onload = () => resolve(r.result as ArrayBuffer);
          r.onerror = () => reject(r.error);
          r.readAsArrayBuffer(blob);
        });
  // copy into this realm's typed array (a test DOM may hand out a foreign ArrayBuffer)
  const digest = await crypto.subtle.digest('SHA-256', Uint8Array.from(new Uint8Array(buf)));
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('');
}

/** File name usable in an object key: characters outside `[A-Za-z0-9._-]` become `_` (last 200 characters kept). */
export function safeFileName(name: string): string {
  const s = name
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[^A-Za-z0-9._-]/g, '_');
  return s.length ? s.slice(-200) : 'file';
}

const newId = () => (typeof crypto.randomUUID === 'function' ? crypto.randomUUID() : `${Date.now()}-${Math.random()}`);

const defaultDescribe = (e: unknown): UploadError => ({ message: e instanceof Error ? e.message : String(e) });

export function useChecksumUpload<T>(opts: UseChecksumUploadOptions<T>) {
  const [items, setItems] = useState<UploadItem[]>([]);
  const ref = useRef(opts);
  ref.current = opts;

  const patch = useCallback((id: string, p: Partial<UploadItem>) => setItems((list) => list.map((x) => (x.id === id ? { ...x, ...p } : x))), []);

  const run = useCallback(
    async (item: UploadItem) => {
      const o = ref.current;
      try {
        patch(item.id, { status: 'hashing', progress: STEP.hashing, error: undefined });
        const sha256 = item.sha256 ?? (await sha256Hex(item.file));
        patch(item.id, { status: 'presigning', progress: STEP.presigning, sha256 });
        const contentType = item.file.type || 'application/octet-stream';
        const res = await o.presign({ intent: item.intent, sha256, fileName: safeFileName(item.name), contentType, size: item.file.size, file: item.file });
        if (res.uploadUrl) {
          patch(item.id, { status: 'uploading', progress: STEP.uploading });
          const put = await o.put(res.uploadUrl, { method: 'PUT', body: item.file, headers: { ...(res.headers ?? {}), 'Content-Type': contentType } });
          if (!put.ok) throw Object.assign(new Error(`HTTP ${put.status}`), { uploadStatus: put.status });
        }
        patch(item.id, { status: 'done', progress: STEP.done });
        o.onUploaded({ ...item, sha256, status: 'done', progress: STEP.done }, res.result);
      } catch (e) {
        const described = (e as { uploadStatus?: number }).uploadStatus ? defaultDescribe(e) : (o.describeError ?? defaultDescribe)(e);
        patch(item.id, { status: 'error', error: described });
      }
    },
    [patch],
  );

  const add = useCallback(
    (files: readonly File[]) => {
      const added = files.map<UploadItem>((file) => ({ id: newId(), name: file.name, file, status: 'hashing', progress: 0, intent: newId() }));
      setItems((list) => [...list, ...added]);
      for (const it of added) void run(it);
      return added;
    },
    [run],
  );

  const retry = useCallback(
    (id: string) => {
      const it = items.find((x) => x.id === id);
      if (it && it.status === 'error') void run(it);
    },
    [items, run],
  );

  const remove = useCallback((id: string) => setItems((list) => list.filter((x) => x.id !== id)), []);
  const clearDone = useCallback(() => setItems((list) => list.filter((x) => x.status !== 'done')), []);

  return { items, add, retry, remove, clearDone, busy: items.some((x) => x.status !== 'done' && x.status !== 'error') };
}
