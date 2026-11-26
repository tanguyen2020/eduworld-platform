// F108 T018, T019, T032 — "Thi đấu" of workspace A (module competition through the console screens). Backend = MOCK
// (test/game-ops-mock.ts, test only, AGENTS R17).
// AC-09: menu only with competition.config.*; a tenant user gets the 403 page and the game_ops chunk is never loaded.
// AC-10: 5 groups; list per type; draft from a published version (createGameConfig config_id + based_on_version_id);
//        publish with effective_from; rollback with a mandatory reason; template assignments read only.
// AC-11: server field errors (422 VALIDATION_FAILED) on the form; VPN notice when the internal gateway is unreachable.
// AC-17: a turned-off menu is hidden.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { GAME_ACCOUNTS, renderGameOps } from '../../../test/game-ops-mock';
import { MockPortalBackend, problem, renderPortal, stopPortals, TENANTS } from '../../../test/portal-harness';

vi.setConfig({ testTimeout: 600_000, hookTimeout: 120_000 });
configure({ asyncUtilTimeout: 60_000 });
afterEach(() => stopPortals());

const AUTHOR = GAME_ACCOUNTS['tq.thidau']!.sub;
const RULES = { schema_version: 1, tiers: { hysteresis: 50, levels: [{ code: 'T1', min_rating: null, names: { 'vi-VN': 'Mầm', 'en-US': 'Sprout' } }] } };

describe('Menu và bảo vệ (F108-AC-09, AC-17)', () => {
  it('the Competition Admin sees "Thi đấu" and its 5 groups; the list asks module competition', async () => {
    const { backend } = await renderGameOps('/game-ops/competition', { account: 'tq.thidau', setup: (m) => void m.add('matchmaking-configs', [{ status: 'published', payload: { schema_version: 1 } }], { current: 1 }) });
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    expect(await within(nav).findByText('Thi đấu')).toBeInTheDocument();
    const groups = await screen.findByTestId('competition-groups');
    for (const g of ['Matchmaking', 'Ranking', 'Mùa giải', 'Template', 'Bảng xếp hạng']) expect(within(groups).getByText(g)).toBeInTheDocument();
    await waitFor(() => expect(backend.calls('GET', '/game-admin/matchmaking-configs')).toHaveLength(1));
    expect(backend.calls('GET', '/game-admin/matchmaking-configs')[0]!.gateway).toBe('internal');
    // group "Template": both template types as tabs
    fireEvent.click(within(groups).getByText('Template'));
    expect(await screen.findByRole('tab', { name: 'Template thử thách khối' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Template giải trường' })).toBeInTheDocument();
  });

  it.each(['ad.truong.a', 'qlcm.truong.a'])('%s: /game-ops/competition → 403, the game_ops chunk is never loaded', async (username) => {
    const backend = new MockPortalBackend().withSession(username);
    if (username === 'qlcm.truong.a') window.sessionStorage.setItem('ew.portal.sessionTenantId', TENANTS.thDemoA.tenant_id);
    const { loaders } = await renderPortal({ url: '/game-ops/competition', backend });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(loaders.game_ops).not.toHaveBeenCalled();
    expect(backend.requests.filter((r) => r.path.startsWith('/game-admin'))).toHaveLength(0);
  });

  it('a turned-off menu.game_ops.competition is hidden and its route shows "Tính năng đang tắt"', async () => {
    const { backend } = await renderGameOps('/game-ops/competition', { account: 'sa.admin', disabledMenus: ['menu.game_ops.competition'] });
    expect(await screen.findByTestId('feature-disabled')).toBeInTheDocument();
    expect(within(screen.getByRole('navigation', { name: 'Menu điều hướng' })).queryByText('Thi đấu')).toBeNull();
    expect(backend.calls('GET', '/game-admin/matchmaking-configs')).toHaveLength(0);
  });
});

describe('Phiên bản, publish theo effective_from, rollback (F108-AC-10)', () => {
  it('a draft from a published version is createGameConfig with config_id + based_on_version_id', async () => {
    let id = '';
    const r = await renderGameOps('/', {
      account: 'tq.thidau',
      setup: (m) => {
        const c = m.add('ranking-rules', [{ status: 'published', payload: RULES, published_at: '2026-11-01T01:00:00Z' }]);
        c.current = 1;
        id = c.config_id;
      },
    });
    await r.router.navigate(`/game-ops/competition/ranking-rules/${id}/v/1`);
    await r.user.click(await screen.findByRole('button', { name: 'Tạo bản nháp từ v1' }));
    await waitFor(() => expect(r.backend.calls('POST', '/game-admin/ranking-rules')).toHaveLength(1));
    const call = r.backend.calls('POST', '/game-admin/ranking-rules')[0]!;
    expect(call.headers['idempotency-key']).toBeTruthy();
    expect(call.body).toMatchObject({ config_id: id, based_on_version_id: expect.any(String), payload: RULES });
    expect(r.backend.calls('POST', `/game-admin/ranking-rules/${id}/versions`)).toHaveLength(0);
    await waitFor(() => expect(r.router.state.location.pathname).toBe(`/game-ops/competition/ranking-rules/${id}/v/2`));
  });

  it('publish takes an effective time (effective_from, no publish_at); rollback needs a reason', async () => {
    let id = '';
    const r = await renderGameOps('/', {
      account: 'sa.admin',
      setup: (m) => {
        const c = m.add('ranking-rules', [
          { status: 'published', author_id: AUTHOR, payload: RULES, published_at: '2026-11-01T01:00:00Z' },
          { status: 'approved', author_id: AUTHOR, payload: { ...RULES, tiers: { ...RULES.tiers, hysteresis: 60 } } },
        ]);
        c.current = 1;
        id = c.config_id;
      },
    });
    await r.router.navigate(`/game-ops/competition/ranking-rules/${id}/v/2`);
    await r.user.click(await screen.findByRole('button', { name: 'Xuất bản ngay' }));
    const dialog = await screen.findByRole('dialog', { name: 'Xuất bản v2' });
    expect(within(dialog).getByText('Có hiệu lực ngay')).toBeInTheDocument();
    await r.user.click(within(dialog).getByText('Có hiệu lực từ (ICT)'));
    expect(within(dialog).getByText(/trận đã bắt đầu giữ cấu hình cũ/)).toBeInTheDocument();
    // no time picked → the dialog asks for one, nothing sent
    await r.user.click(within(dialog).getByRole('button', { name: 'Xuất bản ngay' }));
    expect(await within(dialog).findByText('Chọn thời điểm có hiệu lực.')).toBeInTheDocument();
    expect(r.backend.calls('POST', `/game-admin/ranking-rules/${id}/versions/2:publish`)).toHaveLength(0);
    await r.user.click(within(dialog).getByText('Có hiệu lực ngay'));
    await r.user.click(within(dialog).getByRole('button', { name: 'Xuất bản ngay' }));
    await waitFor(() => expect(r.backend.calls('POST', `/game-admin/ranking-rules/${id}/versions/2:publish`)).toHaveLength(1));
    const pub = r.backend.calls('POST', `/game-admin/ranking-rules/${id}/versions/2:publish`)[0]!;
    expect(pub.body).toEqual({});
    expect(pub.headers['idempotency-key']).toBeTruthy();
    expect(pub.headers['if-match']).toBeTruthy();
    // rollback to v1 with a mandatory reason
    await r.user.click(screen.getByRole('tab', { name: 'Phiên bản' }));
    const panel = await screen.findByTestId('versions-panel');
    await r.user.click(await within(panel).findByRole('button', { name: 'Rollback về v1' }));
    const rb = await screen.findByRole('dialog', { name: 'Rollback v2 → v1' });
    await r.user.click(within(rb).getByRole('button', { name: /OK|Xác nhận/ }));
    expect(r.backend.calls('POST', `/game-admin/ranking-rules/${id}:rollback`)).toHaveLength(0);
    fireEvent.change(within(rb).getByRole('textbox'), { target: { value: 'Ngưỡng tier mới gây hạ tier hàng loạt' } });
    await r.user.click(within(rb).getByRole('button', { name: /OK|Xác nhận/ }));
    await waitFor(() => expect(r.backend.calls('POST', `/game-admin/ranking-rules/${id}:rollback`)).toHaveLength(1));
    expect(r.backend.calls('POST', `/game-admin/ranking-rules/${id}:rollback`)[0]!.body).toEqual({ to_version: 1, reason: 'Ngưỡng tier mới gây hạ tier hàng loạt' });
    // the singleton catalogs cannot be unpublished (server rule mirrored as a hidden button only)
    expect(within(panel).queryByRole('button', { name: 'Gỡ xuất bản' })).toBeNull();
  });

  it('a template editor shows its tenant assignments read only and no Godot preview', async () => {
    let id = '';
    const r = await renderGameOps('/', {
      account: 'sa.admin',
      setup: (m, b) => {
        const c = m.add('challenge-templates', [{ status: 'published', payload: { schema_version: 1, kind: 'grade_challenge', name: 'Giải Toán khối' } }]);
        c.current = 1;
        id = c.config_id;
        b.overrides.unshift((q) =>
          q.path === '/admin/tenant-assignments'
            ? new Response(JSON.stringify({ data: [{ tenant_id: TENANTS.thDemoA.tenant_id, tenant_name: 'TH Demo A', resource_type: 'challenge_template', resource_id: id, assigned_at: '2026-11-20T03:00:00Z' }], page: { has_more: false, limit: 50, next_cursor: null } }), { status: 200, headers: { 'Content-Type': 'application/json' } })
            : undefined,
        );
      },
    });
    await r.router.navigate(`/game-ops/competition/challenge-templates/${id}/v/1`);
    const panel = await screen.findByTestId('template-assignments');
    expect(await within(panel).findByText('TH Demo A')).toBeInTheDocument();
    const call = r.backend.calls('GET', '/admin/tenant-assignments')[0]!;
    expect(new URLSearchParams(call.search).get('resource_type')).toBe('challenge_template');
    expect(within(panel).queryByRole('button', { name: /Gán|Gỡ/ })).toBeNull();
    expect(screen.queryByTestId('preview-panel')).toBeNull();
  });
});

describe('Lỗi theo Problem Details (F108-AC-11)', () => {
  it('a 422 VALIDATION_FAILED from module competition is shown on its field; the UI applies no business rule itself', async () => {
    const r = await renderGameOps('/game-ops/competition/challenge-templates/new', {
      account: 'tq.thidau',
      setup: (_m, b) =>
        b.overrides.unshift((q) =>
          q.method === 'POST' && q.path === '/game-admin/challenge-templates'
            ? problem(422, 'VALIDATION_FAILED', { trace_id: 'trace-f108-422', errors: [{ field: 'payload/kind', code: 'INVALID', message: 'no class challenge' }] })
            : undefined,
        ),
    });
    await r.user.click(await screen.findByRole('button', { name: 'Tạo bản nháp' }));
    await waitFor(() => expect(r.backend.calls('POST', '/game-admin/challenge-templates')).toHaveLength(1));
    expect((await screen.findAllByText('Giá trị chưa hợp lệ (INVALID).')).length).toBeGreaterThan(0);
    expect(screen.getByText('Mã truy vết: trace-f108-422')).toBeInTheDocument();
  });

  it('internal gateway unreachable → "Cần kết nối mạng quản trị (VPN)"', async () => {
    await renderGameOps('/game-ops/competition/seasons', {
      account: 'tq.thidau',
      setup: (_m, b) =>
        b.overrides.unshift((q) => {
          if (q.path === '/game-admin/seasons') throw new TypeError('Failed to fetch');
          return undefined;
        }),
    });
    expect((await screen.findAllByText('Cần kết nối mạng quản trị (VPN)')).length).toBeGreaterThan(0);
  });
});
