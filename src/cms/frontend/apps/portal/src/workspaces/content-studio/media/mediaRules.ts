// mediaRules (F054-AC-17, CONTENT_MODEL §7, pure): the browser pre-check of a file before presignMediaUpload — kind
// from the extension, accepted extensions and `media_max_bytes_by_kind`. Only a convenience: the server and the media
// pipeline (magic bytes, ClamAV) decide; a renamed `.exe` passes this check and is `rejected` by the pipeline.
import { MEDIA_RULES } from '../shared/config';
import type { MediaKind } from './useMediaUpload';

export type UploadKind = Exclude<MediaKind, 'document'>;

export type MediaCheck = { ok: true; kind: UploadKind } | { ok: false; code: 'MEDIA_TYPE_NOT_ALLOWED' | 'MEDIA_TOO_LARGE'; kind?: UploadKind; maxBytes?: number };

const KINDS: UploadKind[] = ['image', 'audio', 'video', 'model3d'];

export const extOf = (name: string) => (name.includes('.') ? name.slice(name.lastIndexOf('.') + 1).toLowerCase() : '');

export function kindOfFile(name: string): UploadKind | null {
  const ext = extOf(name);
  return KINDS.find((k) => (MEDIA_RULES[k].ext as readonly string[]).includes(ext)) ?? null;
}

export function checkMediaFile(file: Pick<File, 'name' | 'size'>): MediaCheck {
  const kind = kindOfFile(file.name);
  if (!kind) return { ok: false, code: 'MEDIA_TYPE_NOT_ALLOWED' };
  const maxBytes = MEDIA_RULES[kind].maxBytes;
  if (file.size > maxBytes) return { ok: false, code: 'MEDIA_TOO_LARGE', kind, maxBytes };
  if (file.size <= 0) return { ok: false, code: 'MEDIA_TYPE_NOT_ALLOWED', kind };
  return { ok: true, kind };
}

/** `accept` attribute of the drop zone. */
export const MEDIA_ACCEPT = KINDS.flatMap((k) => MEDIA_RULES[k].ext.map((e) => `.${e}`)).join(',');

export const formatMb = (bytes: number) => `${Math.round((bytes / 1024 / 1024) * 10) / 10} MB`;
