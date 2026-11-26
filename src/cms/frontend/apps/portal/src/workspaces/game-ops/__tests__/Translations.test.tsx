// F082 T019, T026, T032, T038 — translation of game content in workspace A. Backend = MOCK (test/game-ops-mock.ts, test
// only, AGENTS R17).
// AC-09: Đủ / Thiếu / Lỗi thời badges and the "Cần dịch" / "Bản dịch lỗi thời" filters sent to the server (game types only).
// AC-10: game texts as two columns (vi "Gốc" first); an outdated field opens the diff of its original (diffGameConfigVersions
//        between translated_in_version and the current version); no machine translation.
// AC-11: missing_fields lock "Gửi duyệt" and are listed; outdated only → a warning in the confirmation, still allowed.
// AC-15: export dialog → job followed → link with its expiry; import: file → server check → dry run → report → confirm
//        (one Idempotency-Key per intent). AC-18: 403 page without the permission. AC-20: turned-off menu / action and
//        403 FEATURE_DISABLED.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { GAME_ACCOUNTS, renderGameOps } from '../../../test/game-ops-mock';
import { stopPortals } from '../../../test/portal-harness';

vi.setConfig({ testTimeout: 600_000, hookTimeout: 120_000 });
configure({ asyncUtilTimeout: 60_000 });
afterEach(() => stopPortals());

const DESIGNER = GAME_ACCOUNTS['gd.thietke']!.sub;
const ACH = {
  code: 'explorer',
  category: 'exploration',
  name: { vi: 'Nhà thám hiểm' },
  description: { vi: 'Khám phá rừng số mới', en: 'Explore the number forest' },
  trigger: { event: 'area.entered' },
  progress_metric: 'count',
  visibility: 'visible',
  tiers: [{ tier: 1, threshold: 1 }],
};

describe('Trạng thái bản dịch (F082-AC-09)', () => {
  it('F082-AC-09 badges per item and the "Cần dịch" / "Bản dịch lỗi thời" filters go to the server', async () => {
    const r = await renderGameOps('/game-ops/achievements/achievements', {
      setup: (m) => {
        m.add('achievements', [{ status: 'draft', payload: { ...ACH, code: 'a_partial' }, translation_status: 'partial', missing_fields: ['/name'] }]);
        m.add('achievements', [{ status: 'draft', payload: { ...ACH, code: 'a_outdated' }, translation_status: 'outdated' }]);
        m.add('achievements', [{ status: 'published', payload: { ...ACH, code: 'a_done' }, translation_status: 'complete' }]);
      },
    });
    expect(within((await screen.findByText('a_partial')).closest('tr')!).getByText('Thiếu')).toBeInTheDocument();
    expect(within(screen.getByText('a_outdated').closest('tr')!).getByText('Lỗi thời')).toBeInTheDocument();
    expect(within(screen.getByText('a_done').closest('tr')!).getByText('Đủ')).toBeInTheDocument();
    await r.user.click(screen.getByRole('combobox', { name: 'Trạng thái bản dịch' }));
    await r.user.click((await screen.findAllByTitle('Cần dịch')).at(-1)!);
    await waitFor(() => expect(new URLSearchParams(r.backend.calls('GET', '/game-admin/achievements').at(-1)!.search).get('translation_status')).toBe('partial'));
    expect(await screen.findByText('a_partial')).toBeInTheDocument();
    expect(screen.queryByText('a_done')).toBeNull();
    await r.user.click(screen.getByRole('combobox', { name: 'Trạng thái bản dịch' }));
    await r.user.click((await screen.findAllByTitle('Bản dịch lỗi thời')).at(-1)!);
    await waitFor(() => expect(new URLSearchParams(r.backend.calls('GET', '/game-admin/achievements').at(-1)!.search).get('translation_status')).toBe('outdated'));
  });
});

describe('Editor song ngữ (F082-AC-10, AC-11)', () => {
  it('F082-AC-11 missing_fields lock "Gửi duyệt" and list the fields; F082-AC-10 two columns with "Gốc", no machine translation', async () => {
    let id = '';
    const r = await renderGameOps('/', { setup: (m) => (id = m.add('achievements', [{ status: 'draft', author_id: DESIGNER, payload: ACH, translation_status: 'partial', missing_fields: ['/name'] }]).config_id) });
    await r.router.navigate(`/game-ops/achievements/achievements/${id}/v/1`);
    const blockers = await screen.findByRole('list', { name: /Chưa thể gửi duyệt/ });
    expect(within(blockers).getByText(/Thiếu bản dịch: name/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Gửi duyệt' })).toBeDisabled();
    expect(screen.getByTestId('editor-translation-status')).toHaveTextContent('Thiếu');
    const name = screen.getByTestId('field-name');
    expect(within(name).getByText('Gốc')).toBeInTheDocument();
    expect(screen.queryByText(/dịch máy|tự động dịch/i)).toBeNull();
  });

  it('F082-AC-11 outdated only: warning in the confirmation, still sent; F082-AC-10 the diff of the original', async () => {
    let id = '';
    const r = await renderGameOps('/', {
      setup: (m) =>
        (id = m.add('achievements', [
          { status: 'published', author_id: DESIGNER, payload: ACH, published_at: '2026-11-01T01:00:00Z' },
          { status: 'draft', author_id: DESIGNER, payload: { ...ACH, name: { vi: 'Nhà thám hiểm', en: 'Explorer' } }, translation_status: 'outdated', outdated_fields: [{ pointer: '/description', translated_in_version: 1 }] },
        ]).config_id),
    });
    await r.router.navigate(`/game-ops/achievements/achievements/${id}/v/2`);
    const panel = await screen.findByTestId('outdated-fields');
    await r.user.click(within(panel).getByRole('button', { name: 'Xem thay đổi bản gốc: description' }));
    const drawer = await screen.findByRole('dialog', { name: 'Bản gốc đã đổi: description' });
    await waitFor(() => expect(r.backend.calls('GET', `/game-admin/achievements/${id}/versions/2:diff`)).toHaveLength(1));
    expect(new URLSearchParams(r.backend.calls('GET', `/game-admin/achievements/${id}/versions/2:diff`)[0]!.search).get('with')).toBe('1');
    expect(within(drawer).getByText('v1 → v2')).toBeInTheDocument();
    fireEvent.keyDown(drawer, { key: 'Escape' });
    await r.user.click(screen.getByRole('button', { name: 'Gửi duyệt' }));
    const confirm = (await screen.findByRole('button', { name: 'Vẫn gửi duyệt' })).closest('.ant-modal') as HTMLElement;
    expect(within(confirm).getAllByText('Bản dịch lỗi thời').length).toBeGreaterThan(0);
    expect(within(confirm).getByText(/description/)).toBeInTheDocument();
    expect(r.backend.calls('POST', `/game-admin/achievements/${id}/versions/2:transition`)).toHaveLength(0);
    await r.user.click(within(confirm).getByRole('button', { name: 'Vẫn gửi duyệt' }));
    await waitFor(() => expect(r.backend.calls('POST', `/game-admin/achievements/${id}/versions/2:transition`)).toHaveLength(1));
  });
});

describe('Xuất / nhập file dịch (F082-AC-15)', () => {
  it('F082-AC-15 export: dialog → exportGameTranslations (Idempotency-Key) → job followed → link with its expiry (ICT)', async () => {
    const r = await renderGameOps('/game-ops/translations', { account: 'tr.dich', setup: (m) => m.add('quests', [{ status: 'draft', payload: { code: 'q_intro' } }]) });
    await r.user.click(await screen.findByRole('button', { name: 'Xuất file dịch' }));
    const dialog = await screen.findByRole('dialog', { name: 'Xuất file dịch' });
    await r.user.click(within(dialog).getByRole('combobox', { name: 'Loại nội dung' }));
    await r.user.click((await screen.findAllByTitle('Quest')).at(-1)!);
    await r.user.click(within(dialog).getByRole('combobox', { name: 'Nội dung (tối đa 500)' }));
    await r.user.click((await screen.findAllByTitle('q_intro')).at(-1)!);
    await r.user.click(within(dialog).getByRole('button', { name: 'Xuất file' }));
    await waitFor(() => expect(r.backend.calls('POST', '/game-admin/translations:export')).toHaveLength(1));
    const call = r.backend.calls('POST', '/game-admin/translations:export')[0]!;
    expect(call.headers['idempotency-key']).toBeTruthy();
    expect(call.gateway).toBe('internal');
    expect(call.body).toMatchObject({ content_types: ['quests'], target_language: 'en', format: 'xliff', only_missing: true });
    expect((call.body as { content_ids: string[] }).content_ids).toHaveLength(1);
    const link = await screen.findByRole('link', { name: 'Tải file' });
    expect(link).toHaveAttribute('href', 'https://exports.test/translations/job.xlf?sig=MOCK');
    expect(screen.getByText(/Link hết hạn lúc 26\/11\/2026 10:15/)).toBeInTheDocument();
  });

  it('F082-AC-15 import: upload (document) → server check → dry run → report → confirm with a new intent', async () => {
    const r = await renderGameOps('/game-ops/translations', { account: 'tr.dich' });
    await r.user.click(await screen.findByRole('button', { name: 'Nhập file dịch' }));
    const dialog = await screen.findByRole('dialog', { name: 'Nhập file dịch' });
    fireEvent.change(within(dialog).getByLabelText('File dịch (XLIFF 2.1 hoặc CSV)'), { target: { files: [new File(['<xliff/>'], 'quests.xlf', { type: '' })] } });
    await waitFor(() => expect(r.backend.calls('POST', '/game-media:presign-upload')).toHaveLength(1));
    const presign = r.backend.calls('POST', '/game-media:presign-upload')[0]!;
    expect(presign.body).toMatchObject({ kind: 'document', filename: 'quests.xlf', mime_type: 'application/xliff+xml', size_bytes: 8, license: 'internal' });
    expect(presign.headers['idempotency-key']).toBeTruthy();
    await waitFor(() => expect(r.backend.requests.filter((x) => x.url?.startsWith('https://upload.test/media/'))).toHaveLength(1));
    const put = r.backend.requests.find((x) => x.url?.startsWith('https://upload.test/media/'))!;
    expect(put.headers.authorization).toBeUndefined();
    expect(put.headers['x-amz-checksum-sha256']).toBe('MOCK');
    expect(await within(dialog).findByText('File đã sẵn sàng')).toBeInTheDocument();
    await r.user.click(within(dialog).getByRole('button', { name: 'Chạy thử' }));
    await waitFor(() => expect(r.backend.calls('POST', '/game-admin/translations:import')).toHaveLength(1));
    const dry = r.backend.calls('POST', '/game-admin/translations:import')[0]!;
    expect(dry.body).toMatchObject({ format: 'xliff', target_language: 'en', dry_run: true });
    expect(await within(dialog).findByText('Tổng 2 · cập nhật 0 · bỏ qua 0')).toBeInTheDocument();
    await r.user.click(within(dialog).getByRole('button', { name: 'Xác nhận nhập' }));
    await waitFor(() => expect(r.backend.calls('POST', '/game-admin/translations:import')).toHaveLength(2));
    const real = r.backend.calls('POST', '/game-admin/translations:import')[1]!;
    expect(real.body).toMatchObject({ dry_run: false });
    expect((real.body as { asset_id: string }).asset_id).toBe((dry.body as { asset_id: string }).asset_id);
    expect(real.headers['idempotency-key']).toBeTruthy();
    expect(real.headers['idempotency-key']).not.toBe(dry.headers['idempotency-key']);
    expect(await within(dialog).findByText('Tổng 2 · cập nhật 2 · bỏ qua 0')).toBeInTheDocument();
  });
});

describe('Bảo vệ (F082-AC-18, AC-20)', () => {
  it('F082-AC-18 without game.content.translate → 403 on /game-ops/translations; without game.chat_assets.write → 403 on chat assets', async () => {
    await renderGameOps('/game-ops/translations', { account: 'gd.sticker' });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    stopPortals();
    const r = await renderGameOps('/game-ops/chat-assets', { account: 'tr.dich' });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(r.backend.requests.filter((x) => x.path.startsWith('/game-admin/sticker-packs'))).toHaveLength(0);
  });

  it('F082-AC-20 a turned-off translations menu is hidden; a turned-off action hides the buttons', async () => {
    await renderGameOps('/game-ops/translations', { disabledMenus: ['menu.game_ops.translations'] });
    expect(await screen.findByTestId('feature-disabled')).toBeInTheDocument();
    expect(within(screen.getByRole('navigation', { name: 'Menu điều hướng' })).queryByText('Dịch nội dung game')).toBeNull();
    stopPortals();
    await renderGameOps('/game-ops/translations', { account: 'tr.dich', disabledActions: ['action.game_ops.translations.translate'] });
    await screen.findByTestId('translations-page');
    expect(screen.queryByRole('button', { name: 'Xuất file dịch' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Nhập file dịch' })).toBeNull();
  });

  it('F082-AC-20 403 FEATURE_DISABLED on export → "tạm tắt" notice and getMyPermissions reloaded', async () => {
    const r = await renderGameOps('/game-ops/translations', {
      account: 'tr.dich',
      setup: (_m, b) =>
        b.overrides.unshift((req) =>
          req.path === '/game-admin/translations:export'
            ? new Response(JSON.stringify({ status: 403, code: 'FEATURE_DISABLED', feature_key: 'action.game_ops.translations.translate', trace_id: 'trace-fd-82' }), { status: 403, headers: { 'Content-Type': 'application/problem+json' } })
            : undefined,
        ),
    });
    await r.user.click(await screen.findByRole('button', { name: 'Xuất file dịch' }));
    const dialog = await screen.findByRole('dialog', { name: 'Xuất file dịch' });
    const before = r.backend.calls('GET', '/me/permissions').length;
    await r.user.click(within(dialog).getByRole('button', { name: 'Xuất file' }));
    expect(await screen.findByTestId('game-ops-feature-disabled')).toBeInTheDocument();
    await waitFor(() => expect(r.backend.calls('GET', '/me/permissions').length).toBeGreaterThan(before));
  });
});
