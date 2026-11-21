// F052 T020, T021 — F052-AC-08, F052-AC-09, F052-AC-11: file checks (.csv, csv_import_max), sha256 in the browser,
// the import flow (create → PUT presigned URL without Authorization → track), expired upload link, poll back-off.
import { describe, expect, it, vi } from 'vitest';
import { LIMITS } from '../common/fieldSchemas';
import { checkCsvFile, sha256Hex } from './fileCheck';
import { ImportError, isFinal, pollDelay, runImport, uploadToPresignedUrl, type ImportDeps, type SisImport } from './useSisImport';

const csv = (name = 'hoc_sinh.csv', body = 'student_external_id,username\nHS001,hs.an.3a\n') => new File([body], name, { type: 'text/csv' });

describe('file checks (F052-AC-08)', () => {
  it('.csv only, size ≤ csv_import_max, not empty', () => {
    expect(checkCsvFile({ name: 'a.csv', size: 10 })).toBeNull();
    expect(checkCsvFile({ name: 'a.xlsx', size: 10 })).toBe('not_csv');
    expect(checkCsvFile({ name: 'a.csv', size: 0 })).toBe('empty');
    expect(checkCsvFile({ name: 'a.csv', size: LIMITS.csvImportMaxBytes.maximum + 1 })).toBe('too_large');
    expect(checkCsvFile({ name: `${'x'.repeat(200)}.csv`, size: 10 })).toBe('name_too_long');
  });

  it('sha256 of the file computed with Web Crypto', async () => {
    // MOCK Blob: bytes of this realm (jsdom Blobs live in another realm than Node's Web Crypto)
    const abc = Buffer.from('abc');
    const blob = { arrayBuffer: async () => abc.buffer.slice(abc.byteOffset, abc.byteOffset + abc.length) } as unknown as Blob;
    expect(await sha256Hex(blob)).toBe('ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad');
  });
});

describe('import flow (F052-AC-08, F052-AC-09, F052-AC-11)', () => {
  const created: SisImport = { import_id: 'i1', school_id: 's1', status: 'awaiting_upload', upload_url: 'https://storage.test/u?sig=1', upload_expires_at: '2099-01-01T00:00:00Z', created_at: '2026-11-19T00:00:00Z' };
  const deps = (o: Partial<ImportDeps> = {}): ImportDeps => ({ create: vi.fn(async () => created), upload: vi.fn(async () => undefined), hash: vi.fn(async () => 'h'.repeat(64)), now: () => Date.parse('2026-11-19T00:00:00Z'), ...o });

  it('creates with file name, size, sha256, one entity kind, dry run; then uploads to upload_url', async () => {
    const d = deps();
    const f = csv();
    await expect(runImport(d, f, 'students', false)).resolves.toEqual(created);
    expect(d.create).toHaveBeenCalledWith({ file_name: 'hoc_sinh.csv', size_bytes: f.size, sha256: 'h'.repeat(64), entity_kinds: ['students'], dry_run: false });
    expect(d.upload).toHaveBeenCalledWith('https://storage.test/u?sig=1', f);
  });

  it('a wrong file never reaches the API', async () => {
    const d = deps();
    await expect(runImport(d, csv('a.txt'), 'students', true)).rejects.toMatchObject({ problem: 'not_csv' });
    expect(d.create).not.toHaveBeenCalled();
  });

  it('upload link already expired → upload_expired (a new import gets a new key)', async () => {
    const d = deps({ now: () => Date.parse('2099-01-02T00:00:00Z') });
    await expect(runImport(d, csv(), 'students', true)).rejects.toMatchObject({ problem: 'upload_expired' });
    expect(d.upload).not.toHaveBeenCalled();
  });

  it('upload failure and API failure are told apart', async () => {
    await expect(runImport(deps({ upload: async () => Promise.reject(new Error('net')) }), csv(), 'students', true)).rejects.toMatchObject({ problem: 'upload_failed' });
    await expect(runImport(deps({ create: async () => Promise.reject(new Error('500')) }), csv(), 'students', true)).rejects.toBeInstanceOf(ImportError);
  });

  it('PUT to the presigned URL carries no Authorization nor tenant header', async () => {
    const fetchImpl = vi.fn(async () => new Response(null, { status: 200 }));
    await uploadToPresignedUrl('https://storage.test/u?sig=1', csv(), fetchImpl as unknown as typeof fetch);
    const init = (fetchImpl.mock.calls[0] as unknown as [string, RequestInit])[1];
    expect(init.method).toBe('PUT');
    expect(Object.keys(init.headers as Record<string, string>).map((k) => k.toLowerCase())).toEqual(['content-type']);
    expect(init.credentials).toBe('omit');
  });

  it('poll back-off grows to 15 s; final statuses stop polling', () => {
    expect([0, 1, 2, 10].map(pollDelay)).toEqual([2000, 3000, 4500, 15000]);
    expect(isFinal('applying')).toBe(false);
    expect(['completed', 'completed_with_issues', 'failed', 'expired'].every((s) => isFinal(s as SisImport['status']))).toBe(true);
  });
});
