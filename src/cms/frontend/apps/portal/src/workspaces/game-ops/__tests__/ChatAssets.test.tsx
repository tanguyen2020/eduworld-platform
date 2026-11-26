// F082 T009, T010, T011, T014, T015, T033 — "Tài nguyên chat" of workspace A (sticker packs, internal GIF library of module
// chat). Backend = MOCK (test/game-ops-mock.ts, test only, AGENTS R17).
// AC-01: list of sticker packs (code, name, sticker count, cover, unlock kind, status, version); menu by permission.
// AC-02: bilingual pack form, unlock rule only from PUBLISHED reward rules / items, "Gửi duyệt" locked with the missing `en`.
// AC-03: presign with sha256 + Idempotency-Key (kept on retry), PUT without Authorization, per-file server errors.
// AC-04: rollback to a version published before, unpublish warns, "Gán tenant" only with platform.tenant_data.assign.
// AC-05/06: GIF grid (static preview, click to play), filters to the server, Approve hidden for the uploader, server reason,
//           no external URL / third-party GIF search. AC-07: preview as a student sees it. AC-19: tag cap + counter.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { GAME_ACCOUNTS, renderGameOps } from '../../../test/game-ops-mock';
import { problem, stopPortals } from '../../../test/portal-harness';
import { setGifFrameExtractor } from '../chat-assets/gifFrame';

vi.setConfig({ testTimeout: 600_000, hookTimeout: 120_000 });
configure({ asyncUtilTimeout: 60_000 });
afterEach(() => stopPortals());

const DESIGNER = GAME_ACCOUNTS['gd.thietke']!.sub;
const SHA = 'a'.repeat(64);
const RULE_ID = '0193a4d2-9000-7000-8000-00000000aaaa';

const PACK = {
  code: 'toan_vui',
  name_vi: 'Bạn đồng hành Toán học',
  name_en: 'Math buddies',
  unlock_rule: { reward_rule_id: RULE_ID },
  stickers: [
    { sticker_id: '0193a4d2-9000-7000-8000-0000000000s1', asset_key: `media/${SHA}/cat.webp`, content_type: 'image/webp', size_bytes: 900, alt_text_vi: 'Mèo vỗ tay', alt_text_en: 'Cat clapping', sort_order: 0, preview_url: 'https://chat-assets.test/signed/cat.webp?sig=1' },
    { sticker_id: '0193a4d2-9000-7000-8000-0000000000s2', asset_key: `media/${SHA}/dog.webp`, content_type: 'image/webp', size_bytes: 800, alt_text_vi: 'Chó cười', alt_text_en: 'Dog laughing', sort_order: 1, preview_url: 'https://chat-assets.test/signed/dog.webp?sig=1' },
  ],
};

const packUrl = (id: string, v = 1) => `/game-ops/chat-assets/sticker-packs/${id}/v/${v}`;

describe('Sticker pack (F082-AC-01, AC-02)', () => {
  it('F082-AC-01 lists packs with code, name, sticker count, cover, unlock kind, status and version; the menu follows the permission', async () => {
    await renderGameOps('/game-ops/chat-assets/sticker-packs', { setup: (m) => m.add('sticker-packs', [{ status: 'published', author_id: DESIGNER, payload: PACK }]) });
    const row = (await screen.findByText('toan_vui')).closest('tr')!;
    expect(within(row).getByText('Bạn đồng hành Toán học')).toBeInTheDocument();
    expect(within(row).getByText('2')).toBeInTheDocument();
    expect(within(row).getByText('Phần thưởng')).toBeInTheDocument();
    expect(within(row).getByText('v1')).toBeInTheDocument();
    expect(within(row).getByRole('img', { name: 'Mèo vỗ tay' })).toHaveAttribute('src', PACK.stickers[0]!.preview_url);
    const nav = screen.getByRole('navigation', { name: 'Menu điều hướng' });
    expect(within(nav).getByText('Sticker, GIF')).toBeInTheDocument();
    // no translation status column / filter on module chat types
    expect(screen.queryByRole('combobox', { name: 'Trạng thái bản dịch' })).toBeNull();
  });

  it('F082-AC-01 a translator (no game.chat_assets.write) has no "Sticker, GIF" menu', async () => {
    await renderGameOps('/game-ops/translations', { account: 'tr.dich' });
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    expect(await within(nav).findByText('Dịch nội dung game')).toBeInTheDocument();
    expect(within(nav).queryByText('Sticker, GIF')).toBeNull();
  });

  it('F082-AC-02 bilingual form; unlock rule from published reward rules only; "Gửi duyệt" locked while an `en` is missing', async () => {
    let id = '';
    const r = await renderGameOps('/', {
      setup: (m) => {
        m.add('reward-rules', [{ status: 'published', payload: { reward_rule_id: RULE_ID, code: 'quest.math_done' } }]);
        m.add('reward-rules', [{ status: 'draft', payload: { code: 'draft.only' } }]);
        id = m.add('sticker-packs', [{ status: 'draft', author_id: DESIGNER, payload: { ...PACK, name_en: undefined, unlock_rule: undefined, stickers: [{ ...PACK.stickers[0]!, alt_text_en: undefined }] } }]).config_id;
      },
    });
    await r.router.navigate(packUrl(id));
    const nameVi = await screen.findByLabelText('Tên pack (Tiếng Việt)');
    expect(nameVi).toHaveValue('Bạn đồng hành Toán học');
    expect(nameVi).toHaveAttribute('maxlength', '128');
    expect(screen.getAllByText('Gốc').length).toBeGreaterThan(0);
    // the missing `en` texts lock "Gửi duyệt" and are listed
    const blockers = screen.getByRole('list', { name: /Chưa thể gửi duyệt/ });
    expect(within(blockers).getByText(/Tên pack/)).toBeInTheDocument();
    expect(within(blockers).getByText(/Alt text sticker 1/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Gửi duyệt' })).toBeDisabled();
    // unlock rule: reward rules of the published list only (every page, status=published), no free id input
    await r.user.click(screen.getByRole('radio', { name: 'Phần thưởng (reward rule)' }));
    await r.user.click(await screen.findByRole('combobox', { name: 'Reward rule đã xuất bản' }));
    expect((await screen.findAllByTitle('quest.math_done')).length).toBeGreaterThan(0);
    expect(screen.queryByTitle('draft.only')).toBeNull();
    expect(r.backend.calls('GET', '/game-admin/reward-rules').some((c) => new URLSearchParams(c.search).get('status') === 'published')).toBe(true);
    // filling the English texts unlocks the button
    fireEvent.change(screen.getByLabelText('Tên pack (Tiếng Anh)'), { target: { value: 'Math buddies' } });
    fireEvent.change(screen.getByLabelText('Alt text sticker 1 (Tiếng Anh)'), { target: { value: 'Cat clapping' } });
    await waitFor(() => expect(screen.getByRole('button', { name: 'Gửi duyệt' })).toBeEnabled());
  });
});

describe('Tải tệp sticker (F082-AC-03)', () => {
  it('F082-AC-03 sha256 + Idempotency-Key per file (same key on retry), PUT without Authorization, error on its file only', async () => {
    let id = '';
    let refuse = true;
    const r = await renderGameOps('/', {
      setup: (m, b) => {
        id = m.add('sticker-packs', [{ status: 'draft', author_id: DESIGNER, payload: { ...PACK, stickers: [] } }]).config_id;
        b.overrides.unshift((req) =>
          req.method === 'POST' && req.path.endsWith('/assets:presign') && (req.body as { file_name: string }).file_name === 'big.webp' && refuse
            ? problem(422, 'VALIDATION_FAILED', { detail: 'size_bytes: over the file limit of the type', errors: [{ field: 'size_bytes', code: 'invalid', message: 'over the file limit of the type' }], trace_id: 'trace-big' })
            : undefined,
        );
      },
    });
    await r.router.navigate(packUrl(id));
    const input = await screen.findByLabelText('Tải sticker lên');
    fireEvent.change(input, { target: { files: [new File(['abc'], 'big.webp', { type: 'image/webp' }), new File(['abc'], 'meo vui.webp', { type: 'image/webp' })] } });
    await waitFor(() => expect(r.backend.requests.filter((x) => x.url?.startsWith('https://upload.test/chat/'))).toHaveLength(1));
    const presigns = r.backend.calls('POST', `/game-admin/sticker-packs/${id}/assets:presign`);
    const ok = presigns.find((c) => (c.body as { file_name: string }).file_name === 'meo_vui.webp')!;
    expect(ok.body).toEqual({ file_name: 'meo_vui.webp', content_type: 'image/webp', size_bytes: 3, sha256: 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad' });
    expect(ok.headers['idempotency-key']).toBeTruthy();
    const put = r.backend.requests.find((x) => x.url?.startsWith('https://upload.test/chat/'))!;
    expect(put.method).toBe('PUT');
    expect(put.headers.authorization).toBeUndefined();
    // the refused file shows the server message + trace id; the other one became a sticker
    expect(await screen.findByText(/Mã truy vết: trace-big/)).toBeInTheDocument();
    expect(await screen.findByLabelText('Alt text sticker 1 (Tiếng Việt)')).toBeInTheDocument();
    refuse = false;
    await r.user.click(screen.getByRole('button', { name: 'Thử lại big.webp' }));
    await waitFor(() => expect(r.backend.requests.filter((x) => x.url?.startsWith('https://upload.test/chat/'))).toHaveLength(2));
    const bigKeys = r.backend.calls('POST', `/game-admin/sticker-packs/${id}/assets:presign`).filter((c) => (c.body as { file_name: string }).file_name === 'big.webp').map((c) => c.headers['idempotency-key']);
    expect(bigKeys).toHaveLength(2);
    expect(new Set(bigKeys).size).toBe(1);
    expect(bigKeys[0]).not.toBe(ok.headers['idempotency-key']);
  });
});

describe('Phát hành, gán tenant (F082-AC-04, AC-07)', () => {
  it('F082-AC-04 rollback to a version published before (archived) and unpublish with the old-messages warning', async () => {
    let id = '';
    const r = await renderGameOps('/', {
      account: 'sa.admin',
      setup: (m) => {
        const c = m.add('sticker-packs', [
          { status: 'archived', author_id: DESIGNER, payload: PACK, published_at: '2026-11-01T01:00:00Z' },
          { status: 'published', author_id: DESIGNER, payload: { ...PACK, name_vi: 'Bạn Toán' }, published_at: '2026-11-10T01:00:00Z' },
        ]);
        c.current = 2;
        id = c.config_id;
      },
    });
    await r.router.navigate(packUrl(id, 2));
    await r.user.click(await screen.findByRole('tab', { name: 'Phiên bản' }));
    const panel = await screen.findByTestId('versions-panel');
    await r.user.click(within(panel).getByRole('button', { name: 'Rollback về v1' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByRole('textbox'), { target: { value: 'Ảnh lỗi' } });
    await r.user.click(within(dialog).getByRole('button', { name: /OK|Xác nhận/ }));
    await waitFor(() => expect(r.backend.calls('POST', `/game-admin/sticker-packs/${id}:rollback`)).toHaveLength(1));
    const call = r.backend.calls('POST', `/game-admin/sticker-packs/${id}:rollback`)[0]!;
    expect(call.body).toEqual({ to_version: 1, reason: 'Ảnh lỗi' });
    expect(call.headers['idempotency-key']).toBeTruthy();
    await r.user.click(within(panel).getByRole('button', { name: 'Gỡ xuất bản' }));
    expect(await screen.findByText(/Tin nhắn cũ vẫn hiển thị sticker/)).toBeInTheDocument();
  });

  it('F082-AC-04 "Gán tenant" opens the shared assignment dialog only with platform.tenant_data.assign', async () => {
    let id = '';
    const r = await renderGameOps('/', { account: 'sa.admin', setup: (m) => (id = m.add('sticker-packs', [{ status: 'published', author_id: DESIGNER, payload: PACK }], { current: 1 }).config_id) });
    await r.router.navigate(packUrl(id));
    await r.user.click(await screen.findByRole('button', { name: 'Gán tenant' }));
    expect(await screen.findByRole('dialog', { name: 'Gán dữ liệu platform cho tenant' })).toBeInTheDocument();
  });

  it('F082-AC-04 the designer (no platform.tenant_data.assign) sees no "Gán tenant"; F082-AC-07 preview as a student sees it', async () => {
    let id = '';
    const r = await renderGameOps('/', { setup: (m) => (id = m.add('sticker-packs', [{ status: 'published', author_id: DESIGNER, payload: PACK }], { current: 1 }).config_id) });
    await r.router.navigate(packUrl(id));
    const preview = await screen.findByTestId('chat-asset-preview');
    expect(screen.queryByRole('button', { name: 'Gán tenant' })).toBeNull();
    expect(within(preview).getByText('Đang khóa — mở bằng phần thưởng')).toBeInTheDocument();
    const img = within(preview).getByRole('img', { name: 'Mèo vỗ tay' });
    expect(img).toHaveAttribute('src', PACK.stickers[0]!.preview_url);
    await r.user.click(within(preview).getByText('EN'));
    expect(within(preview).getByRole('img', { name: 'Cat clapping' })).toBeInTheDocument();
    // images only from the signed URLs of module chat
    for (const el of within(preview).getAllByRole('img')) expect(el.getAttribute('src')).toMatch(/^https:\/\/chat-assets\.test\/signed\//);
  });
});

describe('Thư viện GIF (F082-AC-05, AC-06, AC-19)', () => {
  const GIF = { asset_key: `media/${SHA}/fox.gif`, preview_key: `media/${SHA}/fox.png`, size_bytes: 1000, width: 200, height: 200, tags_vi: ['vui'], tags_en: ['happy'], preview_url: 'https://chat-assets.test/signed/fox.png', url: 'https://chat-assets.test/signed/fox.gif' };

  it('F082-AC-05 grid of static previews (click to play), filters by tag / status / uploader on the server, cursor pages; no external URL or GIF search', async () => {
    const r = await renderGameOps('/game-ops/chat-assets/gifs', {
      setup: (m) => {
        m.add('gifs', [{ status: 'published', author_id: DESIGNER, payload: GIF }]);
        for (let i = 0; i < 22; i++) m.add('gifs', [{ status: 'draft', author_id: GAME_ACCOUNTS['cm.duyet']!.sub, payload: { ...GIF, tags_vi: [`tag${i}`], tags_en: [] } }]);
      },
    });
    const img = await screen.findByRole('img', { name: 'GIF: vui, happy' });
    expect(img).toHaveAttribute('src', GIF.preview_url);
    await r.user.click(screen.getAllByRole('button', { name: 'Phát GIF' })[0]!);
    expect(screen.getByRole('img', { name: 'GIF: vui, happy' })).toHaveAttribute('src', GIF.url);
    const tag = screen.getByLabelText('Lọc theo tag');
    expect(tag).toHaveAttribute('maxlength', '64');
    fireEvent.change(tag, { target: { value: 'vui' } });
    await waitFor(() => expect(new URLSearchParams(r.backend.calls('GET', '/game-admin/gifs').at(-1)!.search).get('tag')).toBe('vui'));
    fireEvent.change(tag, { target: { value: '' } });
    fireEvent.click(screen.getByRole('radio', { name: 'Của tôi' }));
    await waitFor(() => expect(new URLSearchParams(r.backend.calls('GET', '/game-admin/gifs').at(-1)!.search).get('author_id')).toBe(DESIGNER));
    fireEvent.click(screen.getByRole('radio', { name: 'Tất cả' }));
    await r.user.click(screen.getByRole('combobox', { name: 'Trạng thái' }));
    await r.user.click((await screen.findAllByTitle('Bản nháp')).at(-1)!);
    await waitFor(() => expect(new URLSearchParams(r.backend.calls('GET', '/game-admin/gifs').at(-1)!.search).get('status')).toBe('draft'));
    expect(await screen.findByRole('button', { name: 'Tải thêm' })).toBeInTheDocument();
    expect(document.querySelector('input[type="url"]')).toBeNull();
    expect(screen.queryByText(/giphy|tenor|tìm GIF trên mạng/i)).toBeNull();
  });

  it('F082-AC-05 upload GIFs: create the draft, presign + PUT the GIF and its static preview, then save keys and tags', async () => {
    setGifFrameExtractor(async (file) => ({ preview: new File(['png'], `${file.name}.png`, { type: 'image/png' }), width: 120, height: 90 }));
    const r = await renderGameOps('/game-ops/chat-assets/gifs');
    await r.user.click(await screen.findByRole('button', { name: 'Tải GIF lên' }));
    const dialog = await screen.findByRole('dialog', { name: 'Tải GIF lên' });
    fireEvent.change(within(dialog).getByLabelText('Tag tiếng Việt'), { target: { value: 'hoc tap' } });
    await r.user.click(within(dialog).getByRole('button', { name: 'Thêm tag tiếng Việt' }));
    fireEvent.change(within(dialog).getByLabelText('Chọn tệp GIF'), { target: { files: [new File(['gif'], 'fox.gif', { type: 'image/gif' })] } });
    await waitFor(() => expect(r.backend.requests.filter((c) => c.method === 'PATCH' && c.path.startsWith('/game-admin/gifs/'))).toHaveLength(1));
    expect(r.backend.calls('POST', '/game-admin/gifs')[0]!.headers['idempotency-key']).toBeTruthy();
    const presigns = r.backend.requests.filter((x) => x.method === 'POST' && x.path.endsWith('/assets:presign'));
    expect(presigns.map((p) => (p.body as { content_type: string }).content_type)).toEqual(['image/gif', 'image/png']);
    const patch = r.backend.requests.find((c) => c.method === 'PATCH' && c.path.startsWith('/game-admin/gifs/'))!;
    expect(patch.headers['if-match']).toBe('"1"');
    expect((patch.body as { payload: Record<string, unknown> }).payload).toMatchObject({ asset_key: expect.stringMatching(/^media\/[0-9a-f]{64}\/fox\.gif$/), preview_key: expect.stringMatching(/fox\.gif\.png$/), size_bytes: 3, width: 120, height: 90, tags_vi: ['hoc tap'] });
  });

  it('F082-AC-06 the uploader has no "Duyệt"; a server refusal of a review shows its reason by code', async () => {
    let id = '';
    const r = await renderGameOps('/', { setup: (m) => (id = m.add('gifs', [{ status: 'in_review', author_id: DESIGNER, payload: GIF }]).config_id) });
    await r.router.navigate(`/game-ops/chat-assets/gifs/${id}/v/1`);
    await screen.findByTestId('chat-asset-preview');
    expect(screen.queryByRole('button', { name: 'Duyệt' })).toBeNull();
    stopPortals();
    const rev = await renderGameOps('/', {
      account: 'cm.duyet',
      setup: (m, b) => {
        id = m.add('gifs', [{ status: 'in_review', author_id: DESIGNER, payload: GIF }]).config_id;
        b.overrides.unshift((req) => (req.path.endsWith(':transition') ? problem(403, 'PERMISSION_DENIED', { trace_id: 'trace-review' }) : undefined));
      },
    });
    await rev.router.navigate(`/game-ops/chat-assets/gifs/${id}/v/1`);
    await rev.user.click(await screen.findByRole('button', { name: 'Duyệt' }));
    expect(await screen.findByText(/trace-review/)).toBeInTheDocument();
  });

  it('F082-AC-19 tag inputs are capped by the schema with a counter; the pattern is explained', async () => {
    let id = '';
    const r = await renderGameOps('/', { setup: (m) => (id = m.add('gifs', [{ status: 'draft', author_id: DESIGNER, payload: GIF }]).config_id) });
    await r.router.navigate(`/game-ops/chat-assets/gifs/${id}/v/1`);
    const input = await screen.findByLabelText('Tag tiếng Anh');
    expect(input).toHaveAttribute('maxlength', '64');
    fireEvent.change(input, { target: { value: 'Big Smile!' } });
    expect(screen.getByText(/chữ thường không dấu/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Thêm tag tiếng Anh' })).toBeDisabled();
    expect(within(input.closest('.ant-input-affix-wrapper') ?? document.body).getByText(/\/ ?64/)).toBeInTheDocument();
  });
});
