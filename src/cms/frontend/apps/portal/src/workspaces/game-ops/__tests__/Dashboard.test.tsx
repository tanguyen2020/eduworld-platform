// F079 T045 (Clarification 2026-11-20) — Game Dashboard of workspace A: configs per type × workflow status from
// getGameConfigCounts (counted by the server), no play metric; the "to translate" figure waits for F082. Backend = MOCK
// (test/game-ops-mock.ts, test only, AGENTS R17).
import { configure, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { renderGameOps } from '../../../test/game-ops-mock';
import { stopPortals } from '../../../test/portal-harness';

vi.setConfig({ testTimeout: 600_000, hookTimeout: 120_000 });
configure({ asyncUtilTimeout: 60_000 });
afterEach(() => stopPortals());

describe('Game Dashboard (F079 T045)', () => {
  it('shows the server counts per type and status, and totals of the review pipeline', async () => {
    const { backend } = await renderGameOps('/game-ops/dashboard', {
      account: 'lo.vanhanh',
      setup: (m) =>
        (m.counts = [
          { config_type: 'quests', status: 'in_review', count: 3 },
          { config_type: 'quests', status: 'published', count: 12 },
          { config_type: 'events', status: 'in_review', count: 2 },
          { config_type: 'events', status: 'draft', count: 1 },
        ]),
    });
    const table = await screen.findByTestId('dashboard-counts');
    const quests = within(table).getByText('Quest').closest('tr')!;
    expect(quests).toHaveTextContent('12');
    expect(screen.getByTestId('dashboard-total-in_review')).toHaveTextContent('5');
    expect(screen.getByTestId('dashboard-total-draft')).toHaveTextContent('1');
    expect(screen.getByText(/F082/)).toBeInTheDocument();
    expect(backend.calls('GET', '/game-admin/stats/config-counts')).toHaveLength(1);
  });
});
