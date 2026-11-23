// F122 T026, T027 — banner "Đang tắt kiểm duyệt" (F122-AC-13, AC-14; ADR-0030). Backend = MOCK
// (test/chat-safety-mock.ts, test only, AGENTS R17).
// AC-13: tenant staff of the tenant with a step off see the banner on every screen (getTenantModerationStatus), System Admin
// sees it for the filtered tenants (listModerationSwitches); refreshed ≤ 60 s; gone when switched back on or expired.
// AC-14: another tenant sees nothing; the banner never shows the reason.
// F122 T065: System Admin without a tenant filter sees the NUMBER of tenants with a step off (disabled_tenant_count, counted
// by the server) and a link to the switches — never a list of tenants.
import { act, configure, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MODERATION_STATUS_REFRESH_MS } from '../../../shared/chatSafety/useModerationStatus';
import { chatSafetyState, renderSafety, T_A } from '../../../test/chat-safety-mock';
import { stopPortals } from '../../../test/portal-harness';
import { tenantFilterStore } from '../../tenantFilterStore';

vi.setConfig({ testTimeout: 900_000 }); // whole-portal screens on a shared, throttled CPU (other agents run tests too)
configure({ asyncUtilTimeout: 90_000 });

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

describe('Banner "Đang tắt kiểm duyệt" (F122-AC-13, AC-14)', () => {
  it('tenant staff of TH Demo A: banner with the step and the expiry on every screen, no reason; gone once back on', async () => {
    const { backend, state, router } = await renderSafety('/school-lms/students', { account: 'ad.truong.a' });
    const banner = await screen.findByTestId('moderation-disabled-banner');
    expect(within(screen.getByTestId('banner-slot')).getByTestId('moderation-disabled-banner')).toBe(banner);
    expect(banner).toHaveTextContent(/Đang tắt kiểm duyệt: Lọc spam — đến \d{2}\/\d{2}\/\d{4} \d{2}:\d{2}/);
    expect(banner).not.toHaveTextContent(/Sự kiện thi đấu|Điều tra|lý do/i);
    const call = backend.calls('GET', '/tenant/chat/moderation-status')[0]!;
    expect(call.headers['x-tenant-id']).toBe(T_A);
    // every screen
    await act(() => router.navigate('/school-lms/clubs'));
    expect(await screen.findByTestId('moderation-disabled-banner')).toBeInTheDocument();
    // refresh interval within 60 s, and the banner disappears when the step is back on
    expect(MODERATION_STATUS_REFRESH_MS).toBeLessThanOrEqual(60_000);
    state.status[T_A] = { tenant_id: T_A, enabled_all: true, disabled_steps: [], expires_at: null, source_scope: null };
    const n = backend.calls('GET', '/tenant/chat/moderation-status').length;
    vi.useFakeTimers({ shouldAdvanceTime: true, toFake: ['setTimeout', 'setInterval'] });
    try {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(MODERATION_STATUS_REFRESH_MS + 1000);
      });
    } finally {
      vi.useRealTimers();
    }
    await waitFor(() => expect(backend.calls('GET', '/tenant/chat/moderation-status').length).toBeGreaterThan(n));
    await waitFor(() => expect(screen.queryByTestId('moderation-disabled-banner')).toBeNull());
  });

  it('expired switch: no banner even before the next refresh', async () => {
    const st = chatSafetyState();
    st.status[T_A] = { ...st.status[T_A]!, expires_at: new Date(Date.now() - 60_000).toISOString() };
    const { backend } = await renderSafety('/school-lms/students', { account: 'ad.truong.a', state: st });
    await waitFor(() => expect(backend.calls('GET', '/tenant/chat/moderation-status').length).toBeGreaterThan(0));
    await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    expect(screen.queryByTestId('moderation-disabled-banner')).toBeNull();
  });

  it('F122-AC-14: a staff member of another tenant sees no banner', async () => {
    const { backend } = await renderSafety('/school-lms/students', { account: 'ad.truong.b' });
    await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    await waitFor(() => expect(backend.calls('GET', '/tenant/chat/moderation-status').length).toBeGreaterThan(0));
    expect(backend.calls('GET', '/tenant/chat/moderation-status').every((r) => r.headers['x-tenant-id'] !== T_A)).toBe(true);
    expect(screen.queryByTestId('moderation-disabled-banner')).toBeNull();
  });

  it('F122-AC-13 (T065): System Admin without a tenant filter — number of tenants off and a link to the switches, no tenant list', async () => {
    const st = chatSafetyState();
    st.disabledTenantCount = 3;
    const { backend } = await renderSafety('/platform-admin/tenants', { state: st });
    const banner = await screen.findByTestId('moderation-disabled-banner');
    expect(banner).toHaveTextContent('Đang tắt kiểm duyệt: 3 tenant đang tắt ít nhất một bước');
    expect(within(banner).getByRole('link', { name: 'Xem công tắc kiểm duyệt' })).toHaveAttribute('href', '/platform-admin/moderation-switches');
    expect(new URLSearchParams(backend.calls('GET', '/admin/chat/moderation-switches').at(-1)!.search).getAll('tenant_ids')).toEqual([]);
    expect(banner).not.toHaveTextContent(/TH Demo A|THCS Demo B/);
  });

  it('System Admin without a filter and no tenant off: no banner (platform row all on)', async () => {
    const st = chatSafetyState();
    st.disabledTenantCount = 0;
    const { backend } = await renderSafety('/platform-admin/tenants', { state: st });
    await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    await waitFor(() => expect(backend.calls('GET', '/admin/chat/moderation-switches').length).toBeGreaterThan(0));
    expect(screen.queryByTestId('moderation-disabled-banner')).toBeNull();
  });

  it('System Admin: banner for the tenants of the filter (listModerationSwitches), with the tenant name', async () => {
    const { backend } = await renderSafety('/platform-admin/tenants');
    await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    act(() => tenantFilterStore.set([T_A]));
    const banner = await screen.findByTestId('moderation-disabled-banner');
    expect(banner).toHaveTextContent(/TH Demo A: Lọc spam — đến/);
    expect(new URLSearchParams(backend.calls('GET', '/admin/chat/moderation-switches').at(-1)!.search).getAll('tenant_ids')).toEqual([T_A]);
    expect(banner).not.toHaveTextContent(/Điều tra|Sự kiện/);
  });
});
