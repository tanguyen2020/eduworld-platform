// F054 T036–T040 (whole portal + MOCK backend; real pipeline / Playwright pending — OQ F051-9): media library.
//   AC-16 list filtered by kind / status / q, rejection reason, variants · AC-17 drag & drop several files, pre-check of
//   type and size before presign, license + alt text required, status to ready / rejected, server refusal shown ·
//   AC-18 detail: used in versions, no overwrite action.
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { P, renderPublishing, stopPortals, type MockPublishing } from '../../../test/publishing-harness';

afterEach(() => {
  stopPortals();
  vi.unstubAllGlobals();
});

function seed(m: MockPublishing) {
  m.media.push(
    { asset_id: P(201), kind: 'image', status: 'ready', original_filename: 'phan-so.png', sha256: 'a'.repeat(64), license: 'CC-BY', alt_text: 'Hình phân số', variants: [{ variant: 'w256', url: 'https://cdn.mock.test/a.webp', mime_type: 'image/webp', size_bytes: 1000 }], used_in: [{ item_type: 'question', item_id: P(1), version: 2 }] },
    { asset_id: P(202), kind: 'image', status: 'rejected', reject_reason: 'type_mismatch', original_filename: 'virus.png', sha256: 'b'.repeat(64) },
  );
}

const file = (name: string, size = 100, type = 'image/png') => new File([new Uint8Array(size)], name, { type });

describe('Thư viện media (F054 US3)', () => {
  it('AC-16/AC-18: filters, rejection reason, variants; detail shows the versions using it and no overwrite', async () => {
    const { backend } = await renderPublishing('/content-studio/media', { setup: seed });
    expect(await screen.findByText('phan-so.png')).toBeInTheDocument();
    expect(screen.getByText('type_mismatch')).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Trạng thái' }));
    fireEvent.click(await screen.findByTitle('Bị từ chối'));
    await waitFor(() => expect(backend.calls('GET', '/cms/media').some((r) => r.search.includes('status=rejected'))).toBe(true));
    await waitFor(() => expect(screen.queryByText('phan-so.png')).toBeNull());
    fireEvent.change(screen.getByLabelText('Tìm theo tên tệp'), { target: { value: 'virus' } });
    fireEvent.keyDown(screen.getByLabelText('Tìm theo tên tệp'), { key: 'Enter', code: 'Enter' });
    await waitFor(() => expect(backend.calls('GET', '/cms/media').some((r) => r.search.includes('q=virus'))).toBe(true));
    fireEvent.click(await screen.findByRole('button', { name: 'virus.png' }));
    const d = await screen.findByTestId('media-detail');
    expect(within(d).getByText('Tệp bị từ chối: type_mismatch')).toBeInTheDocument();
    stopPortals();
    document.body.innerHTML = '';
    await renderPublishing('/content-studio/media', { setup: seed });
    fireEvent.click(await screen.findByRole('button', { name: 'phan-so.png' }));
    const detail = await screen.findByTestId('media-detail');
    expect(within(detail).getByTestId('media-used-in')).toHaveTextContent(`Câu hỏi ${P(1)} · v2`);
    expect(within(detail).getByText(/Không ghi đè tệp đã tải lên/)).toBeInTheDocument();
    expect(within(detail).queryByRole('button', { name: /Thay|Ghi đè|Tải lên/ })).toBeNull();
  });

  it('AC-17: several files — pre-check refuses type/size before presign, license + alt required, statuses to ready / rejected', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 200 }))); // MOCK object storage PUT
    const { backend } = await renderPublishing('/content-studio/media', { setup: seed });
    fireEvent.click(await screen.findByRole('button', { name: 'Tải lên' }));
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [file('ok.png'), file('renamed-exe.png'), file('tool.exe', 10, 'application/octet-stream'), file('huge.png', 11 * 1024 * 1024)] } });
    const rows = await screen.findByTestId('upload-rows');
    await waitFor(() => expect(rows.querySelectorAll('li')).toHaveLength(4));
    expect(within(rows.querySelector('[data-file="tool.exe"]') as HTMLElement).getByText('Loại tệp không được phép.')).toBeInTheDocument();
    expect(within(rows.querySelector('[data-file="huge.png"]') as HTMLElement).getByText('Tệp vượt giới hạn 10 MB.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Tải lên 2 tệp' }));
    expect(await screen.findByText('Bắt buộc ghi giấy phép hoặc nguồn.')).toBeInTheDocument();
    expect(await screen.findByText('Ảnh phải có mô tả.')).toBeInTheDocument();
    expect(backend.calls('POST', '/cms/media:presign-upload')).toHaveLength(0);
    fireEvent.change(screen.getByLabelText('Giấy phép / nguồn'), { target: { value: 'Tự biên soạn' } });
    fireEvent.change(screen.getByLabelText('Mô tả ảnh (alt)'), { target: { value: 'Hình minh họa' } });
    fireEvent.click(screen.getByRole('button', { name: 'Tải lên 2 tệp' }));
    await waitFor(() => expect(backend.calls('POST', '/cms/media:presign-upload')).toHaveLength(2));
    expect(backend.calls('POST', '/cms/media:presign-upload').map((c) => (c.body as { filename: string }).filename)).toEqual(['ok.png', 'renamed-exe.png']);
    expect((backend.calls('POST', '/cms/media:presign-upload')[0]!.body as Record<string, unknown>)).toMatchObject({ kind: 'image', license: 'Tự biên soạn', alt_text: 'Hình minh họa' });
    await waitFor(() => expect(rows.querySelector('[data-file="ok.png"] [data-media-status="ready"]')).not.toBeNull());
    await waitFor(() => expect(rows.querySelector('[data-file="renamed-exe.png"] [data-media-status="rejected"]')).not.toBeNull());
  });

  it('AC-17: server refusal MEDIA_TYPE_NOT_ALLOWED is shown on the file', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 200 })));
    await renderPublishing('/content-studio/media', { setup: seed });
    fireEvent.click(await screen.findByRole('button', { name: 'Tải lên' }));
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [new File([new Uint8Array(10)], 'clip.mp4.bat.mp3', { type: 'audio/mpeg' })] } });
    await screen.findByTestId('upload-rows');
    fireEvent.change(screen.getByLabelText('Giấy phép / nguồn'), { target: { value: 'CC0' } });
    fireEvent.click(screen.getByRole('button', { name: 'Tải lên 1 tệp' }));
    // the MOCK refuses names containing ".bat" like a server-side type check
    await waitFor(() => expect(screen.queryByText('Loại tệp không được phép.')).not.toBeNull());
  });
});
