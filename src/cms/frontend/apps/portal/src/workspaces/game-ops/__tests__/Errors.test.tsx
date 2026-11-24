// F079 T033, T036, T037 — preview, errors and limits of workspace A. Backend = MOCK (test/game-ops-mock.ts, test only, R17).
// AC-17: "Xem trước" → createGamePreviewSession (Idempotency-Key) → snapshot read with X-Preview-Token → GamePreviewFrame
//        (iframe of the runtime `gamePreviewUrl`); the token never appears in the iframe URL.
// AC-19: 403 PERMISSION_DENIED → 403 page with the trace_id (no MFA step-up, ADR-0094); internal gateway unreachable →
//        "Cần kết nối mạng quản trị (VPN)".
// AC-20: the same Idempotency-Key on a retry of the same intent; maxLength + counter; errors by code + trace_id.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TEST_CONFIG } from '../../../shared/api/test-utils';
import { GAME_ACCOUNTS, renderGameOps } from '../../../test/game-ops-mock';
import { problem, stopPortals } from '../../../test/portal-harness';

vi.setConfig({ testTimeout: 600_000, hookTimeout: 120_000 });
configure({ asyncUtilTimeout: 60_000 });
afterEach(() => stopPortals());

const DESIGNER = GAME_ACCOUNTS['gd.thietke']!.sub;

describe('Preview Godot Web (F079-AC-17)', () => {
  it('creates a preview session, reads the snapshot with the token and embeds the frame without the token in its URL', async () => {
    let id = '';
    const r = await renderGameOps('/', { setup: (m) => (id = m.add('quests', [{ status: 'draft', author_id: DESIGNER, payload: { code: 'q_preview' } }]).config_id) });
    await r.router.navigate(`/game-ops/quests/quests/${id}/v/1`);
    await r.user.click(await screen.findByRole('button', { name: 'Xem trước v1' }));
    await waitFor(() => expect(r.backend.calls('GET', '/game-admin/preview/snapshot')).toHaveLength(1));
    const session = r.backend.calls('POST', '/game-admin/preview/sessions')[0]!;
    expect(session.headers['idempotency-key']).toBeTruthy();
    expect(session.body).toEqual({ config_type: 'quests', config_id: id, version: 1 });
    expect(r.backend.calls('GET', '/game-admin/preview/snapshot')[0]!.headers['x-preview-token']).toBe('preview-token-MOCK-0123456789');
    const frame = (await screen.findByTitle('Preview Godot Web v1')) as HTMLIFrameElement;
    expect(frame.src).toBe(TEST_CONFIG.gamePreviewUrl);
    expect(frame.src).not.toContain('preview-token');
    expect(frame.getAttribute('sandbox')).toBe('allow-scripts allow-same-origin');
  });
});

describe('Lỗi quyền và mạng quản trị (F079-AC-19)', () => {
  it('403 PERMISSION_DENIED → 403 page with the trace id', async () => {
    await renderGameOps('/game-ops/items/items', {
      setup: (_m, b) => b.overrides.unshift((r) => (r.path === '/game-admin/items' ? problem(403, 'PERMISSION_DENIED', { trace_id: 'trace-denied-79' }) : undefined)),
    });
    const page = await screen.findByTestId('game-ops-forbidden');
    expect(page).toHaveTextContent('Không có quyền truy cập');
    expect(page).toHaveTextContent('trace-denied-79');
    expect(screen.queryByText(/MFA|xác thực hai lớp/i)).toBeNull();
  });

  it('internal gateway unreachable → "Cần kết nối mạng quản trị (VPN)"', async () => {
    await renderGameOps('/game-ops/items/items', {
      setup: (_m, b) =>
        b.overrides.unshift((r) => {
          if (r.path === '/game-admin/items') throw new TypeError('Failed to fetch');
          return undefined;
        }),
    });
    expect((await screen.findAllByText('Cần kết nối mạng quản trị (VPN)')).length).toBeGreaterThan(0);
  });
});

describe('Idempotency, giới hạn độ dài, mã lỗi (F079-AC-20)', () => {
  it('a retry of the same create reuses the Idempotency-Key; the title is capped with a counter; the error shows code text + trace_id', async () => {
    let calls = 0;
    const { backend, user } = await renderGameOps('/game-ops/rewards/reward-rules/new', {
      setup: (_m, b) =>
        b.overrides.unshift((r) => (r.method === 'POST' && r.path === '/game-admin/reward-rules' && ++calls === 1 ? problem(500, 'INTERNAL', { trace_id: 'trace-500-79' }) : undefined)),
    });
    const title = await screen.findByLabelText('Tiêu đề hiển thị trong Console');
    expect(title).toHaveAttribute('maxlength', '200');
    fireEvent.change(title, { target: { value: 'x'.repeat(250) } });
    expect(within(title.closest('.ant-input-affix-wrapper') ?? document.body).getByText(/\/ 200|\/200/)).toBeInTheDocument();
    fireEvent.change(title, { target: { value: 'Quy tắc XP quiz' } });
    await user.click(screen.getByRole('button', { name: 'Tạo bản nháp' }));
    expect(await screen.findByText('Mã truy vết: trace-500-79')).toBeInTheDocument();
    expect(screen.getByText('Lỗi hệ thống. Vui lòng thử lại sau.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Tạo bản nháp' }));
    await waitFor(() => expect(backend.calls('POST', '/game-admin/reward-rules').length).toBeGreaterThanOrEqual(2));
    const keys = backend.calls('POST', '/game-admin/reward-rules').map((c) => c.headers['idempotency-key']);
    expect(keys[0]).toBeTruthy();
    expect(new Set(keys).size).toBe(1);
  });
});
