// parseImportFile (F054-AC-27, FR-QB-011, specs/schemas/import/questions-row.schema.json — pure, in the browser):
// reads a question import file — CSV (RFC 4180, UTF-8 with or without BOM) or the first sheet of an XLSX (zip of
// SpreadsheetML: shared strings, inline strings, numbers) — into header + rows numbered like the server report (row 1 =
// header), and writes the normalized CSV that is uploaded. [Proposed] no third-party parser (SheetJS on npm has
// unfixed advisories): the zip is read with the browser DecompressionStream ('deflate-raw').
import { IMPORT_MAX_ROWS } from '../shared/config';

export interface ImportTable {
  header: string[];
  /** Data rows; `row` = line number in the file (header = 1), empty rows skipped. */
  rows: { row: number; cells: Record<string, string> }[];
}

export type ParseErrorCode = 'INVALID_FORMAT' | 'MISSING_COLUMN' | 'FILE_TOO_LARGE' | 'NO_DATA';

export class ImportParseError extends Error {
  constructor(
    readonly code: ParseErrorCode,
    readonly detail = '',
  ) {
    super(`${code} ${detail}`);
  }
}

/** Required columns of questions-row.schema.json. */
export const REQUIRED_COLUMNS = ['type', 'subject_code', 'grade_levels', 'curriculum_code', 'topic_code', 'objective_codes', 'difficulty', 'stem', 'render_payload', 'answer_key', 'scoring_rule'] as const;

/** RFC 4180 CSV → records (quotes, doubled quotes, CR/LF inside quoted fields). */
export function parseCsv(text: string): string[][] {
  const s = text.charCodeAt(0) === 0xfeff ? text.slice(1) : text;
  const out: string[][] = [];
  let rec: string[] = [];
  let field = '';
  let quoted = false;
  for (let i = 0; i < s.length; i++) {
    const c = s[i]!;
    if (quoted) {
      if (c === '"') {
        if (s[i + 1] === '"') {
          field += '"';
          i++;
        } else quoted = false;
      } else field += c;
      continue;
    }
    if (c === '"' && field === '') quoted = true;
    else if (c === ',') {
      rec.push(field);
      field = '';
    } else if (c === '\n' || c === '\r') {
      if (c === '\r' && s[i + 1] === '\n') i++;
      rec.push(field);
      out.push(rec);
      rec = [];
      field = '';
    } else field += c;
  }
  if (quoted) throw new ImportParseError('INVALID_FORMAT', 'unterminated quote');
  if (field !== '' || rec.length) {
    rec.push(field);
    out.push(rec);
  }
  return out;
}

const csvCell = (v: string) => (/[",\r\n]/.test(v) ? `"${v.replace(/"/g, '""')}"` : v);

/** Records → CSV text (UTF-8, CRLF). */
export function toCsv(records: readonly (readonly string[])[]): string {
  return records.map((r) => r.map(csvCell).join(',')).join('\r\n') + '\r\n';
}

/** Records → table: header lower-cased, required columns checked, row numbers kept, row cap. */
export function toTable(records: readonly string[][]): ImportTable {
  if (records.length < 2) throw new ImportParseError('NO_DATA');
  const header = records[0]!.map((h) => h.trim().toLowerCase());
  const missing = REQUIRED_COLUMNS.filter((c) => !header.includes(c));
  if (missing.length) throw new ImportParseError('MISSING_COLUMN', missing.join(', '));
  const rows: ImportTable['rows'] = [];
  records.slice(1).forEach((rec, i) => {
    if (rec.join('').trim() === '') return;
    rows.push({ row: i + 2, cells: Object.fromEntries(header.map((h, k) => [h, (rec[k] ?? '').trim()])) });
  });
  if (rows.length === 0) throw new ImportParseError('NO_DATA');
  if (rows.length > IMPORT_MAX_ROWS) throw new ImportParseError('FILE_TOO_LARGE', String(IMPORT_MAX_ROWS));
  return { header, rows };
}

// ---- XLSX (first sheet) ----

interface ZipEntry {
  name: string;
  method: number;
  compressedSize: number;
  offset: number;
}

function zipEntries(buf: Uint8Array): ZipEntry[] {
  const dv = new DataView(buf.buffer, buf.byteOffset, buf.byteLength);
  let eocd = -1;
  for (let i = buf.length - 22; i >= Math.max(0, buf.length - 65_557); i--) {
    if (dv.getUint32(i, true) === 0x06054b50) {
      eocd = i;
      break;
    }
  }
  if (eocd < 0) throw new ImportParseError('INVALID_FORMAT', 'not a zip');
  const count = dv.getUint16(eocd + 10, true);
  let p = dv.getUint32(eocd + 16, true);
  const out: ZipEntry[] = [];
  const dec = new TextDecoder();
  for (let n = 0; n < count; n++) {
    if (dv.getUint32(p, true) !== 0x02014b50) throw new ImportParseError('INVALID_FORMAT', 'bad central directory');
    const method = dv.getUint16(p + 10, true);
    const compressedSize = dv.getUint32(p + 20, true);
    const nameLen = dv.getUint16(p + 28, true);
    const extraLen = dv.getUint16(p + 30, true);
    const commentLen = dv.getUint16(p + 32, true);
    const offset = dv.getUint32(p + 42, true);
    out.push({ name: dec.decode(buf.subarray(p + 46, p + 46 + nameLen)), method, compressedSize, offset });
    p += 46 + nameLen + extraLen + commentLen;
  }
  return out;
}

async function inflateRaw(data: Uint8Array): Promise<Uint8Array> {
  const stream = new Blob([Uint8Array.from(data)]).stream().pipeThrough(new DecompressionStream('deflate-raw'));
  return new Uint8Array(await new Response(stream).arrayBuffer());
}

async function readEntry(buf: Uint8Array, e: ZipEntry): Promise<string> {
  const dv = new DataView(buf.buffer, buf.byteOffset, buf.byteLength);
  if (dv.getUint32(e.offset, true) !== 0x04034b50) throw new ImportParseError('INVALID_FORMAT', 'bad local header');
  const start = e.offset + 30 + dv.getUint16(e.offset + 26, true) + dv.getUint16(e.offset + 28, true);
  const raw = buf.subarray(start, start + e.compressedSize);
  const bytes = e.method === 0 ? raw : e.method === 8 ? await inflateRaw(raw) : null;
  if (!bytes) throw new ImportParseError('INVALID_FORMAT', `zip method ${e.method}`);
  return new TextDecoder().decode(bytes);
}

const xml = (s: string) => new DOMParser().parseFromString(s, 'application/xml');
const byTag = (d: Document | Element, tag: string) => Array.from(d.getElementsByTagNameNS('*', tag));
const textOf = (el: Element) => byTag(el, 't').map((t) => t.textContent ?? '').join('');

/** Column letters of a cell reference (`AB12` → 27, 0-based). */
export function columnIndex(ref: string): number {
  const letters = /^[A-Z]+/.exec(ref.toUpperCase())?.[0] ?? 'A';
  let n = 0;
  for (const ch of letters) n = n * 26 + (ch.charCodeAt(0) - 64);
  return n - 1;
}

/** First worksheet of an XLSX file → records (strings). */
export async function readXlsx(bytes: Uint8Array): Promise<string[][]> {
  const entries = zipEntries(bytes);
  const find = (name: string) => entries.find((e) => e.name === name);
  const wbRels = find('xl/_rels/workbook.xml.rels');
  const wb = find('xl/workbook.xml');
  let sheetPath = 'xl/worksheets/sheet1.xml';
  if (wb && wbRels) {
    const firstSheet = byTag(xml(await readEntry(bytes, wb)), 'sheet')[0];
    // relationship id = the namespaced `id` attribute (prefix `r:` in practice)
    const rid = Array.from(firstSheet?.attributes ?? []).find((a) => a.localName === 'id' && !!a.prefix)?.value;
    const rel = byTag(xml(await readEntry(bytes, wbRels)), 'Relationship').find((r) => r.getAttribute('Id') === rid);
    const target = rel?.getAttribute('Target');
    if (target) sheetPath = target.startsWith('/') ? target.slice(1) : `xl/${target.replace(/^\.\//, '')}`;
  }
  const sheet = find(sheetPath);
  if (!sheet) throw new ImportParseError('INVALID_FORMAT', 'no worksheet');
  const sst = find('xl/sharedStrings.xml');
  const shared = sst ? byTag(xml(await readEntry(bytes, sst)), 'si').map(textOf) : [];
  const out: string[][] = [];
  for (const row of byTag(xml(await readEntry(bytes, sheet)), 'row')) {
    const r = Number(row.getAttribute('r') ?? out.length + 1);
    while (out.length < r - 1) out.push([]);
    const rec: string[] = [];
    for (const c of byTag(row, 'c')) {
      const i = columnIndex(c.getAttribute('r') ?? 'A');
      const type = c.getAttribute('t');
      const v = byTag(c, 'v')[0]?.textContent ?? '';
      const value = type === 's' ? (shared[Number(v)] ?? '') : type === 'inlineStr' ? textOf(c) : type === 'b' ? (v === '1' ? 'true' : 'false') : v;
      while (rec.length < i) rec.push('');
      rec[i] = value;
    }
    out.push(rec);
  }
  return out;
}

export type ImportKind = 'questions_csv' | 'questions_xlsx';

export function importKindOf(name: string): ImportKind | null {
  const n = name.toLowerCase();
  return n.endsWith('.csv') ? 'questions_csv' : n.endsWith('.xlsx') ? 'questions_xlsx' : null;
}

async function bytesOf(file: Blob): Promise<Uint8Array> {
  if (typeof file.arrayBuffer === 'function') return new Uint8Array(await file.arrayBuffer());
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => resolve(new Uint8Array(r.result as ArrayBuffer));
    r.onerror = () => reject(r.error);
    r.readAsArrayBuffer(file);
  });
}

/** Reads a CSV / XLSX file chosen by the user into a table. */
export async function parseImportFile(file: File): Promise<ImportTable> {
  const kind = importKindOf(file.name);
  if (!kind) throw new ImportParseError('INVALID_FORMAT', 'extension');
  const bytes = await bytesOf(file);
  const records = kind === 'questions_csv' ? parseCsv(new TextDecoder().decode(bytes)) : await readXlsx(bytes);
  return toTable(records);
}
