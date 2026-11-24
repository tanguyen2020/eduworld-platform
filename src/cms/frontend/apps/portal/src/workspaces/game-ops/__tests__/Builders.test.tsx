// F079 T018–T020 — builders of workspace A. Backend = MOCK (test/game-ops-mock.ts, test only, AGENTS R17).
// AC-06: quest steps added by type and reordered by drag and drop, payload per quest.schema.json (order follows the
//        list); NPC dialogue as a graph (nodes, links picked among node ids, start node).
// AC-07: area / NPC pickers come from the client manifest (or the published configs when none is registered) — no free id.
// AC-08: other types use the form of their schema; the shop price is in-game currency only (no real-money field).
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { renderGameOps } from '../../../test/game-ops-mock';
import { stopPortals } from '../../../test/portal-harness';

vi.setConfig({ testTimeout: 600_000, hookTimeout: 120_000 });
configure({ asyncUtilTimeout: 60_000 });
afterEach(() => stopPortals());

const MANIFEST = {
  registered: true,
  platforms: [{ platform: 'web', client_version: '1.4.0', published_at: '2026-11-20T01:00:00Z' }],
  areas: [
    { area_id: 'main_hub', name: { vi: 'Quảng trường', en: 'Hub' } },
    { area_id: 'math_forest', name: { vi: 'Rừng Toán', en: 'Math forest' } },
  ],
  npcs: [{ npc_id: 'owl_teacher', name: { vi: 'Cô Cú', en: 'Ms Owl' } }],
  scene_refs: ['res://scenes/world/math_forest.tscn'],
  interactables: ['chest_1'],
};

describe('Quest builder (F079-AC-06, AC-07)', () => {
  it('adds steps talk_npc → activity → collect, reorders by drag and drop, creates the quest with order = position', async () => {
    const { backend, user } = await renderGameOps('/game-ops/quests/quests/new', { setup: (m) => (m.manifest = MANIFEST) });
    const steps = await screen.findByTestId('quest-steps');
    const add = async (type: string) => {
      await user.click(within(steps).getByRole('combobox', { name: 'Loại bước mới' }));
      await user.click((await screen.findAllByTitle(type)).at(-1)!);
      await user.click(within(steps).getByRole('button', { name: 'Thêm bước' }));
    };
    await add('Nói chuyện NPC');
    await add('Hoạt động học');
    await add('Thu thập');
    expect(within(steps).getAllByRole('listitem')).toHaveLength(3);
    // drag step 3 (collect) onto step 1
    fireEvent.dragStart(screen.getByTestId('quest-step-2'));
    fireEvent.dragOver(screen.getByTestId('quest-step-0'));
    fireEvent.drop(screen.getByTestId('quest-step-0'));
    expect(within(screen.getByTestId('quest-step-0')).getByText('Thu thập')).toBeInTheDocument();
    // area picker: manifest values only
    const area = within(screen.getByTestId('field-area_id')).getByRole('combobox', { name: 'Vùng' });
    await user.click(area);
    expect((await screen.findAllByTitle('Rừng Toán')).length).toBeGreaterThan(0);
    fireEvent.change(area, { target: { value: 'vung_tu_do' } });
    fireEvent.keyDown(area, { key: 'Enter', keyCode: 13 });
    fireEvent.change(area, { target: { value: 'rung' } });
    await user.click((await screen.findAllByTitle('Rừng Toán')).at(-1)!);
    await user.click(screen.getByRole('button', { name: 'Tạo bản nháp' }));
    await waitFor(() => expect(backend.calls('POST', '/game-admin/quests').length).toBeGreaterThan(0));
    // a retry of the same intent (slow shared CPU) keeps the key: one draft server-side (AC-20)
    const posts = backend.calls('POST', '/game-admin/quests');
    expect(new Set(posts.map((p) => p.headers['idempotency-key'])).size).toBe(1);
    const call = posts.at(-1)!;
    const payload = (call.body as { payload: { steps: { type: string; order: number; step_id: string }[]; area_id: string; schema_version: number } }).payload;
    expect(payload.steps.map((s) => [s.type, s.order])).toEqual([
      ['collect', 1],
      ['talk_npc', 2],
      ['activity', 3],
    ]);
    expect(payload.area_id).toBe('math_forest');
    expect(payload.schema_version).toBe(2); // const of quest.schema.json, filled by the form
    expect(call.headers['idempotency-key']).toBeTruthy();
  });

  it('without a registered manifest the area picker lists the published world areas (every page)', async () => {
    const { backend } = await renderGameOps('/game-ops/quests/quests/new', {
      setup: (m) => m.add('world-areas', [{ status: 'published', payload: { area_id: 'sea_port', name: { vi: 'Cảng biển', en: 'Port' } } }]),
    });
    const area = within(await screen.findByTestId('field-area_id')).getByRole('combobox', { name: 'Vùng' });
    fireEvent.mouseDown(area);
    expect((await screen.findAllByTitle('Cảng biển')).length).toBeGreaterThan(0);
    const q = new URLSearchParams(backend.calls('GET', '/game-admin/world-areas')[0]!.search);
    expect(q.get('status')).toBe('published');
    expect(q.get('limit')).toBe('100');
  });
});

describe('NPC dialogue graph (F079-AC-06)', () => {
  it('adds nodes, links n1 → n2 by picking the node id, sets the start node; the overview lists the edges', async () => {
    const { user } = await renderGameOps('/game-ops/npcs/npc-dialogues/new', { setup: (m) => (m.manifest = MANIFEST) });
    await user.click(await screen.findByRole('button', { name: 'Thêm node' }));
    await user.click(screen.getByRole('button', { name: 'Thêm node' }));
    const graph = screen.getByTestId('dialogue-graph');
    expect(within(graph).getByText('Node không đi tới được từ node bắt đầu: n1, n2')).toBeInTheDocument();
    await user.click(within(graph).getByRole('combobox', { name: 'Node bắt đầu' }));
    await user.click((await screen.findAllByTitle('n1')).at(-1)!);
    await user.click(screen.getAllByText('n1').find((el) => el.closest('.ant-collapse-header'))!);
    const next = await screen.findByRole('combobox', { name: 'next' });
    await user.click(next);
    await user.click((await screen.findAllByTitle('n2')).at(-1)!);
    await waitFor(() => expect(screen.getByTestId('dialogue-edge-n1')).toHaveTextContent('→ n2'));
    expect(within(graph).queryByText(/không đi tới được/)).toBeNull();
    // the NPC of the dialogue comes from the manifest
    await user.click(screen.getByRole('combobox', { name: 'NPC' }));
    expect((await screen.findAllByTitle('Cô Cú')).length).toBeGreaterThan(0);
  });
});

describe('Shop item (F079-AC-08)', () => {
  it('price = amount of the in-game currency only (currency fixed to xu, no real-money field)', async () => {
    const { backend, user } = await renderGameOps('/game-ops/shop/shop-items/new');
    await screen.findByTestId('create-config');
    expect(screen.queryByText(/VND|tiền thật|real money/i)).toBeNull();
    expect(screen.queryByLabelText('Tiền tệ trong game')).toBeNull();
    const price = screen.getByRole('group', { name: 'Giá' });
    fireEvent.change(within(price).getByRole('spinbutton', { name: 'Số lượng' }), { target: { value: '120' } });
    await user.click(screen.getByRole('button', { name: 'Tạo bản nháp' }));
    await waitFor(() => expect(backend.calls('POST', '/game-admin/shop-items')).toHaveLength(1));
    expect((backend.calls('POST', '/game-admin/shop-items')[0]!.body as { payload: { price: unknown } }).payload.price).toEqual({ amount: 120, currency_code: 'xu' });
  });
});
