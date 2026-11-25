// F079 T011, T035, T043 — list of a config type and the protection of workspace A. Backend = MOCK (test/game-ops-mock.ts,
// test only, AGENTS R17).
// AC-01: listGameConfigs of the type (status filter, cursor "Tải thêm"), columns code / title / status Tag / version /
//        author / thumbnail; menus only for the permissions of getMyPermissions.
// AC-16: thumbnail of the version, placeholder + label while the render is pending / failed.
// AC-18: a user without any game.* permission gets the 403 page on /game-ops/* and the chunk is never loaded.
// AC-21: a turned-off menu is hidden; its route shows "Tính năng đang tắt".
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { renderGameOps } from '../../../test/game-ops-mock';
import { SAFETY_ACCOUNTS } from '../../../test/chat-safety-mock';
import { MockPortalBackend, renderPortal, stopPortals, TENANTS } from '../../../test/portal-harness';

vi.setConfig({ testTimeout: 600_000, hookTimeout: 120_000 });
configure({ asyncUtilTimeout: 60_000 });
afterEach(() => stopPortals());

describe('Danh sách cấu hình (F079-AC-01, AC-16)', () => {
  it('lists the newest version of each item with status, version, author, thumbnail; filters by status; pages with the cursor', async () => {
    const { backend, user } = await renderGameOps('/game-ops/items/items', {
      setup: (m) => {
        m.add('items', [{ status: 'published', title: 'Mũ phi công', payload: { item_id: 'hat_pilot', name: { vi: 'Mũ phi công', en: 'Pilot hat' } }, thumbnail_url: 'https://cdn.test/hat.png', render_status: 'ready' }]);
        m.add('items', [{ status: 'draft', title: 'Áo mưa', payload: { item_id: 'raincoat' }, render_status: 'pending' }]);
        m.add('items', [{ status: 'published', payload: { item_id: 'cape' }, render_status: 'failed' }]);
        for (let i = 0; i < 20; i++) m.add('items', [{ status: 'draft', payload: { item_id: `filler_${i}` } }]);
      },
    });
    expect(await screen.findByText('hat_pilot')).toBeInTheDocument();
    const row = screen.getByText('hat_pilot').closest('tr')!;
    expect(within(row).getByText('Mũ phi công')).toBeInTheDocument();
    expect(within(row).getByText('v1')).toBeInTheDocument();
    expect(within(row).getByRole('img', { name: /Mũ phi công v1/ })).toHaveAttribute('src', 'https://cdn.test/hat.png');
    expect(within(screen.getByText('raincoat').closest('tr')!).getByText('Đang render')).toBeInTheDocument();
    expect(within(screen.getByText('cape').closest('tr')!).getByText('Render lỗi')).toBeInTheDocument();
    // first page from the server only; "Tải thêm" asks the next cursor
    const list = backend.calls('GET', '/game-admin/items');
    expect(list[0]!.gateway).toBe('internal');
    expect(new URLSearchParams(list[0]!.search).get('cursor')).toBeNull();
    expect(screen.queryByText('filler_19')).toBeNull();
    await user.click(screen.getByRole('button', { name: 'Tải thêm' }));
    expect(await screen.findByText('filler_19')).toBeInTheDocument();
    expect(new URLSearchParams(backend.calls('GET', '/game-admin/items').at(-1)!.search).get('cursor')).toBe('c20');
    // status filter goes to the server
    await user.click(screen.getByRole('combobox', { name: 'Trạng thái' }));
    await user.click((await screen.findAllByTitle('Đã phát hành')).at(-1)!);
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/game-admin/items').at(-1)!.search).get('status')).toBe('published'));
  });

  it('menus follow getMyPermissions: the Live-ops account sees Sự kiện, not Media nor Dịch nội dung', async () => {
    await renderGameOps('/game-ops/events/events', { account: 'lo.vanhanh' });
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    expect(await within(nav).findByText('Sự kiện & mùa')).toBeInTheDocument();
    // liveops: game.event.write, game.content.publish… — no game.quest.write / game.item.write / game.content.translate
    expect(within(nav).queryByText('Media nội dung game')).toBeNull();
    expect(within(nav).queryByText('Dịch nội dung game')).toBeNull();
  });

  it('a type without type module says so and calls no API (currencies)', async () => {
    const { backend } = await renderGameOps('/game-ops/shop/currencies');
    expect(await screen.findByTestId('type-not-served')).toBeInTheDocument();
    expect(backend.calls('GET', '/game-admin/currencies')).toHaveLength(0);
  });
});

describe('Bảo vệ workspace A (F079-AC-18, AC-21)', () => {
  it.each(['ad.truong.a', 'qlcm.truong.a', 'sm.antoan'])('%s: /game-ops/quests → 403, the game_ops chunk is never loaded', async (username) => {
    const backend = new MockPortalBackend().withSession(username === 'sm.antoan' ? 'sa.admin' : username);
    if (username === 'sm.antoan') backend.user = SAFETY_ACCOUNTS['sm.antoan']!;
    if (username === 'qlcm.truong.a') window.sessionStorage.setItem('ew.portal.sessionTenantId', TENANTS.thDemoA.tenant_id);
    const { loaders } = await renderPortal({ url: '/game-ops/quests', backend });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(loaders.game_ops).not.toHaveBeenCalled();
    expect(backend.requests.filter((r) => r.path.startsWith('/game-admin'))).toHaveLength(0);
  });

  it('a turned-off menu (shop) is hidden and its route shows "Tính năng đang tắt"; items still work', async () => {
    const { backend } = await renderGameOps('/game-ops/shop', { disabledMenus: ['menu.game_ops.shop'] });
    expect(await screen.findByTestId('feature-disabled')).toBeInTheDocument();
    expect(within(screen.getByRole('navigation', { name: 'Menu điều hướng' })).queryByText('Shop, tiền tệ')).toBeNull();
    expect(backend.calls('GET', '/game-admin/shop-items')).toHaveLength(0);
  });

  it('403 FEATURE_DISABLED from the API: "tính năng tạm tắt" and getMyPermissions is reloaded', async () => {
    const { backend } = await renderGameOps('/game-ops/items/items', {
      setup: (_m, b) => b.overrides.unshift((r) => (r.path === '/game-admin/items' ? new Response(JSON.stringify({ status: 403, code: 'FEATURE_DISABLED', feature_key: 'menu.game_ops.items', trace_id: 'trace-fd' }), { status: 403, headers: { 'Content-Type': 'application/problem+json' } }) : undefined)),
    });
    const before = backend.calls('GET', '/me/permissions').length;
    expect(await screen.findByTestId('game-ops-feature-disabled')).toBeInTheDocument();
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(before));
  });

  it('the create button is inside ActionGate: hidden when action.game_ops.items.edit is off', async () => {
    await renderGameOps('/game-ops/items/items', { disabledActions: ['action.game_ops.items.edit'] });
    await screen.findByTestId('config-list');
    expect(screen.queryByRole('button', { name: /Tạo Vật phẩm/ })).toBeNull();
    fireEvent.keyDown(document.body, { key: 'Escape' });
  });
});
