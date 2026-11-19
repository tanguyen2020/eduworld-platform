// CSV checks before upload (F052 T024, F052-AC-08): `.csv` extension (SisImportCreate.file_name pattern, ≤ 200 chars) and
// size ≤ `csv_import_max` (SisImportCreate.size_bytes.maximum); sha256 of the file computed in the browser (Web Crypto).
// Web Crypto has no incremental digest, so the file (≤ 10 MB) is read once into memory and never stored anywhere.
// Pre-checks only warn early — the server validates the file again (AGENTS R1).
import { FIELDS, LIMITS } from '../common/fieldSchemas';

export type FileProblem = 'not_csv' | 'empty' | 'too_large' | 'name_too_long';

export function checkCsvFile(file: Pick<File, 'name' | 'size'>, maxBytes: number = LIMITS.csvImportMaxBytes.maximum): FileProblem | null {
  if (!new RegExp(FIELDS.sisFileName.pattern).test(file.name)) return 'not_csv';
  if (file.name.length > FIELDS.sisFileName.maxLength) return 'name_too_long';
  if (file.size <= 0) return 'empty';
  if (file.size > maxBytes) return 'too_large';
  return null;
}

const hex = (buf: ArrayBuffer) => Array.from(new Uint8Array(buf), (b) => b.toString(16).padStart(2, '0')).join('');

async function readBytes(file: Blob): Promise<ArrayBuffer> {
  if (typeof file.arrayBuffer === 'function') return file.arrayBuffer();
  return new Response(file).arrayBuffer();
}

export async function sha256Hex(file: Blob, subtle: SubtleCrypto = globalThis.crypto.subtle): Promise<string> {
  // copy into a Uint8Array of this realm (jsdom/foreign ArrayBuffers are rejected by some SubtleCrypto implementations)
  return hex(await subtle.digest('SHA-256', new Uint8Array(await readBytes(file))));
}
