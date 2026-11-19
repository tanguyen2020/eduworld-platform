// F059 T035–T037 — F059-AC-15…17 (TC-CMS-TEN-02, TC-CMS-RBAC-04): system-wide audit with mandatory from/to (default
// last 24 h), filters (tenant_ids[], actor, action within maxLength, resource) in the URL, detail drawer with `changes`
// as escaped JSON and `trace_id`; permission log = one query with action_prefix=role&action_prefix=user_role (OQ F059-3),
// optionally narrowed to one exact action; `422` over the range keeps the
// filters and shows the limit; no edit/delete of audit entries. Backend = MOCK.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { tenantFilterStore } from '../../../shell/tenantFilterStore';
import { stopPortals, TENANTS } from '../../../test/portal-harness';
import { AIDS, renderAccess } from '../../../test/platform-access-backend';

vi.setConfig({ testTimeout: 600_000 });
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

const H = 3_600_000;

describe('Nhật ký toàn hệ thống (F059-AC-15/17)', () => {
  it('from/to default 24 h, no X-Tenant-Id, tenant_ids[] from the filter, columns and escaped changes', async () => {
    const { backend, router, user } = await renderAccess('/platform-admin/audit-logs');
    await screen.findByRole('heading', { level: 1, name: 'Nhật ký toàn hệ thống' });
    expect(await screen.findByText('user_role.granted')).toBeInTheDocument();
    const first = backend.calls('GET', '/admin/audit-logs')[0]!;
    const q = new URLSearchParams(first.search);
    expect(new Date(q.get('to')!).getTime() - new Date(q.get('from')!).getTime()).toBe(24 * H);
    expect(first.headers['x-tenant-id']).toBeUndefined();
    expect(first.gateway).toBe('internal');
    const row = screen.getByText('trace-audit-1').closest('tr')!;
    expect(within(row).getByText('Bổ nhiệm')).toBeInTheDocument();
    expect(within(row).getByText('Nhân sự')).toBeInTheDocument();
    expect(within(row).getByText('TH Demo A')).toBeInTheDocument();
    expect(within(screen.getByText('trace-audit-2').closest('tr')!).getAllByText('Hệ thống').length).toBeGreaterThan(0);
    for (const name of ['Sửa', 'Xóa']) expect(screen.queryByRole('button', { name })).toBeNull();
    await user.click(within(row).getByRole('button', { name: 'Xem' }));
    const changes = await screen.findByTestId('audit-changes');
    expect(changes.textContent).toContain('"note": "<img src=x onerror=alert(1)>"');
    expect(changes.querySelector('img')).toBeNull();
    // filters → query + URL
    const action = screen.getByLabelText('Hành động');
    expect(action).toHaveAttribute('maxlength', '128');
    fireEvent.change(action, { target: { value: 'platform_settings.changed' } });
    fireEvent.change(screen.getByLabelText('Người làm (UUID)'), { target: { value: '0193a4d2-1111-7000-8000-000000000003' } });
    await user.click(screen.getByRole('button', { name: 'Lọc' }));
    await waitFor(() => expect(new URLSearchParams(router.state.location.search).get('action')).toBe('platform_settings.changed'));
    await waitFor(() => {
      const last = new URLSearchParams(backend.calls('GET', '/admin/audit-logs').at(-1)!.search);
      expect(last.get('action')).toBe('platform_settings.changed');
      expect(last.get('actor_id')).toBe('0193a4d2-1111-7000-8000-000000000003');
    });
    tenantFilterStore.set([TENANTS.thDemoA.tenant_id, TENANTS.pgdDemo.tenant_id]);
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/admin/audit-logs').at(-1)!.search).getAll('tenant_ids')).toEqual([TENANTS.thDemoA.tenant_id, TENANTS.pgdDemo.tenant_id]));
  });

  it('422 over the maximum range: limit shown, filters kept', async () => {
    const { backend } = await renderAccess(`/platform-admin/audit-logs?from=2024-01-01T00:00:00.000Z&to=2026-01-01T00:00:00.000Z&action=user_role.granted&resource_id=${AIDS.qlcm}`);
    expect(await screen.findByText('Dữ liệu nhập chưa hợp lệ. Kiểm tra các trường được đánh dấu.')).toBeInTheDocument();
    expect(screen.getByText(/tối đa 93 ngày mỗi lần tra cứu/)).toBeInTheDocument();
    expect(screen.getByLabelText('Hành động')).toHaveValue('user_role.granted');
    expect(screen.getByLabelText('Mã tài nguyên (UUID)')).toHaveValue(AIDS.qlcm);
    expect(backend.calls('GET', '/admin/audit-logs').length).toBeGreaterThan(0);
  });
});

describe('Nhật ký phân quyền (F059-AC-16)', () => {
  it('one query with action_prefix=role&action_prefix=user_role; optional exact action', async () => {
    const { backend, user } = await renderAccess('/platform-admin/audit-logs?tab=permissions');
    expect(await screen.findByText('Bổ nhiệm')).toBeInTheDocument();
    // platform_settings.changed is outside the permission groups
    expect(screen.queryByText('Tắt báo cáo')).toBeNull();
    const first = new URLSearchParams(backend.calls('GET', '/admin/audit-logs').at(-1)!.search);
    expect(first.getAll('action_prefix')).toEqual(['role', 'user_role']);
    expect(first.get('action')).toBeNull();
    await user.click(screen.getByRole('combobox', { name: 'Hành động' }));
    await screen.findAllByTitle('user_role.granted');
    const options = [...document.querySelectorAll('.ant-select-item-option')].map((o) => o.getAttribute('title'));
    expect(options.length).toBeGreaterThan(0);
    for (const o of options) expect(o).toMatch(/^(role|user_role)\./);
    await user.click((await screen.findAllByTitle('user_role.granted')).at(-1)!);
    await user.click(screen.getByRole('button', { name: 'Lọc' }));
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/admin/audit-logs').at(-1)!.search).get('action')).toBe('user_role.granted'));
    expect(await screen.findByText('Bổ nhiệm')).toBeInTheDocument();
    for (const r of backend.calls('GET', '/admin/audit-logs')) expect(new URLSearchParams(r.search).getAll('action_prefix')).toEqual(['role', 'user_role']);
  });
});
