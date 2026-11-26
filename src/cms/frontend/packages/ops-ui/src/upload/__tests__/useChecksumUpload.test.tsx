// F082 T010 — checksum upload queue (F082-AC-03, ADR-0044): sha256 computed in the browser, presign through the caller
// (one intent per file, kept on retry → same Idempotency-Key), PUT to the presigned URL without Authorization, per-file
// progress and the server error shown on THAT file; another file keeps going. No size/type limit in the code (AGENTS R1).
import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it, vi } from 'vitest';
import { renderOps } from '../../test/render';
import { UploadQueue } from '../UploadQueue';
import { sha256Hex, useChecksumUpload, type PresignInput, type UseChecksumUploadOptions } from '../useChecksumUpload';

const ABC_SHA = 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad';

function Harness(props: UseChecksumUploadOptions<{ key: string }>) {
  const q = useChecksumUpload(props);
  return (
    <>
      <input aria-label="files" type="file" multiple onChange={(e) => q.add(Array.from(e.target.files ?? []))} />
      <UploadQueue items={q.items} onRetry={q.retry} onRemove={q.remove} />
    </>
  );
}

const file = (name: string, text = 'abc', type = 'image/webp') => new File([text], name, { type });

describe('useChecksumUpload (F082-AC-03)', () => {
  it('F082-AC-03 sha256 of the bytes in the browser', async () => {
    expect(await sha256Hex(new Blob(['abc']))).toBe(ABC_SHA);
  });

  it('F082-AC-03 presign with sha256 + sanitized name, PUT without Authorization, done per file', async () => {
    const presign = vi.fn(async (i: PresignInput) => ({ uploadUrl: `https://upload.test/${i.sha256}`, headers: { 'x-amz-meta': '1' }, result: { key: `media/${i.sha256}/${i.fileName}` } }));
    const put = vi.fn(async () => new Response(null, { status: 200 }));
    const onUploaded = vi.fn();
    await renderOps(<Harness presign={presign} put={put} onUploaded={onUploaded} />);
    fireEvent.change(screen.getByLabelText('files'), { target: { files: [file('sticker vui (1).webp')] } });
    await waitFor(() => expect(onUploaded).toHaveBeenCalledTimes(1));
    const input = presign.mock.calls[0]![0];
    expect(input).toMatchObject({ sha256: ABC_SHA, fileName: 'sticker_vui__1_.webp', contentType: 'image/webp', size: 3 });
    expect(input.intent).toBeTruthy();
    const [url, init] = put.mock.calls[0]! as unknown as [string, RequestInit];
    expect(url).toBe(`https://upload.test/${ABC_SHA}`);
    expect(init.method).toBe('PUT');
    const headers = new Headers(init.headers);
    expect(headers.get('authorization')).toBeNull();
    expect(headers.get('content-type')).toBe('image/webp');
    expect(headers.get('x-amz-meta')).toBe('1');
    expect(onUploaded.mock.calls[0]![1]).toEqual({ key: `media/${ABC_SHA}/sticker_vui__1_.webp` });
    expect(await screen.findByText('Đã tải lên')).toBeInTheDocument();
  });

  it('F082-AC-03 a server error stays on its file, other files go on; retry reuses the same intent', async () => {
    let fail = true;
    const presign = vi.fn(async (i: PresignInput) => {
      if (i.fileName === 'big.webp' && fail) throw Object.assign(new Error('x'), { code: 'MEDIA_TOO_LARGE', message: 'Tệp vượt giới hạn của loại' });
      return { uploadUrl: 'https://upload.test/ok', result: { key: i.fileName } };
    });
    const put = vi.fn(async () => new Response(null, { status: 200 }));
    const onUploaded = vi.fn();
    await renderOps(<Harness presign={presign} put={put} onUploaded={onUploaded} describeError={(e) => ({ code: (e as { code: string }).code, message: (e as Error).message })} />);
    fireEvent.change(screen.getByLabelText('files'), { target: { files: [file('big.webp'), file('small.webp', 'xyz')] } });
    expect(await screen.findByText(/Tệp vượt giới hạn của loại/)).toBeInTheDocument();
    expect(screen.getByText(/MEDIA_TOO_LARGE/)).toBeInTheDocument();
    await waitFor(() => expect(onUploaded).toHaveBeenCalledTimes(1));
    expect(onUploaded.mock.calls[0]![1]).toEqual({ key: 'small.webp' });
    const firstIntent = presign.mock.calls.find((c) => c[0].fileName === 'big.webp')![0].intent;
    fail = false;
    await act(async () => fireEvent.click(screen.getByRole('button', { name: 'Thử lại big.webp' })));
    await waitFor(() => expect(onUploaded).toHaveBeenCalledTimes(2));
    const intents = presign.mock.calls.filter((c) => c[0].fileName === 'big.webp').map((c) => c[0].intent);
    expect(intents).toEqual([firstIntent, firstIntent]);
    expect(presign.mock.calls.find((c) => c[0].fileName === 'small.webp')![0].intent).not.toBe(firstIntent);
  });

  it('F082-AC-03 a failed PUT is an error of that file', async () => {
    const presign = vi.fn(async () => ({ uploadUrl: 'https://upload.test/x', result: { key: 'k' } }));
    const put = vi.fn(async () => new Response(null, { status: 403 }));
    await renderOps(<Harness presign={presign} put={put} onUploaded={vi.fn()} />);
    fireEvent.change(screen.getByLabelText('files'), { target: { files: [file('a.png', 'abc', 'image/png')] } });
    expect(await screen.findByText(/HTTP 403/)).toBeInTheDocument();
  });

  it('F082-AC-03 no numeric file limit in the upload code (the server decides)', () => {
    const src = fs.readFileSync(path.join(__dirname, '..', 'useChecksumUpload.ts'), 'utf8') + fs.readFileSync(path.join(__dirname, '..', 'UploadQueue.tsx'), 'utf8');
    expect(src).not.toMatch(/\b(512|1024|2048|2097152|524288|MAX_BYTES|maxBytes)\b/);
  });
});
