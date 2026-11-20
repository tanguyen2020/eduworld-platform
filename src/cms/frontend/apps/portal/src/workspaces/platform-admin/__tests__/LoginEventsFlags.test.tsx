// F059 T039/T040/T042 — F059-AC-18…21 (TC-TEN-018): cross-tenant game login events (tenant_ids[], student, from/to
// default 7 days, end_reason labels, cursor paging, no IP); feature flags (list with default/override/protected,
// platform default and tenant override with If-Match + reason, 412 → reload + "Đã có người sửa", no create/delete).
// Backend = MOCK.
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

const DAY = 86_400_000;

describe('Đăng nhập game liên tenant (F059-AC-18/19)', () => {
  it('7-day default, tenant_ids[] and student filter, Vietnamese end reasons, cursor paging, no IP', async () => {
    const { backend, user } = await renderAccess('/platform-admin/student-activity');
    expect(await screen.findByText('Đăng nhập trên thiết bị khác')).toBeInTheDocument();
    const first = backend.calls('GET', '/admin/login-events')[0]!;
    const q = new URLSearchParams(first.search);
    expect(new Date(q.get('to')!).getTime() - new Date(q.get('from')!).getTime()).toBe(7 * DAY);
    expect(first.headers['x-tenant-id']).toBeUndefined();
    expect(screen.getByText('Đang mở')).toBeInTheDocument();
    expect(screen.getByText('Windows · máy lớp 3A')).toBeInTheDocument();
    expect(screen.queryByText('Giờ giới nghiêm')).toBeNull(); // third event on the next page
    await user.click(screen.getByRole('button', { name: 'Tải thêm' }));
    expect(await screen.findByText('Giờ giới nghiêm')).toBeInTheDocument();
    expect(backend.calls('GET', '/admin/login-events').some((r) => new URLSearchParams(r.search).get('cursor') === '2')).toBe(true);
    const page = document.body.textContent ?? '';
    expect(page).not.toMatch(/\bIP\b|ip_address|\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/);
    fireEvent.change(screen.getByLabelText('Mã học sinh (UUID)'), { target: { value: AIDS.hsAn } });
    await user.click(screen.getByRole('button', { name: 'Lọc' }));
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/admin/login-events').at(-1)!.search).get('student_id')).toBe(AIDS.hsAn));
    tenantFilterStore.set([TENANTS.thDemoA.tenant_id]);
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/admin/login-events').at(-1)!.search).getAll('tenant_ids')).toEqual([TENANTS.thDemoA.tenant_id]));
  });
});

describe('Feature flag (F059-AC-20/21)', () => {
  it('list, platform default with If-Match + reason; protected item cannot be turned off; no create/delete', async () => {
    const { backend, user } = await renderAccess('/platform-admin/feature-toggles');
    expect(await screen.findByText('portal_new_dashboard')).toBeInTheDocument();
    expect(within(screen.getByText('menu.platform_admin.audit_logs').closest('tr')!).getByText('Không tắt được')).toBeInTheDocument();
    for (const name of ['Thêm', 'Tạo', 'Xóa']) expect(screen.queryByRole('button', { name })).toBeNull();
    fireEvent.click(within(screen.getByText('menu.platform_admin.audit_logs').closest('tr')!).getByRole('button', { name: 'Sửa' })); // row may sit under the table's loading overlay (pointer-events: none) under CPU load
    let dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByLabelText('Tắt')).toBeDisabled();
    await user.click(within(dialog).getByRole('button', { name: 'Hủy' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    fireEvent.click(within(screen.getByText('portal_new_dashboard').closest('tr')!).getByRole('button', { name: 'Sửa' })); // row may sit under the table's loading overlay (pointer-events: none) under CPU load
    dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByLabelText('Bật'));
    const reason = within(dialog).getByLabelText('Lý do');
    expect(reason).toHaveAttribute('maxlength', '500');
    fireEvent.change(reason, { target: { value: 'Mở thử màn mới' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PUT', '/admin/feature-flags/portal_new_dashboard')).toHaveLength(1));
    const put = backend.calls('PUT', '/admin/feature-flags/portal_new_dashboard')[0]!;
    expect(put.headers['if-match']).toBe('W/"4"');
    expect(put.headers['x-tenant-id']).toBeUndefined();
    expect(put.body).toEqual({ scope: 'platform', state: 'enabled', reason: 'Mở thử màn mới', description: 'Màn tổng quan mới' });
    expect(await screen.findByText('Đã lưu. Thay đổi áp dụng sau vài chục giây.')).toBeInTheDocument();
    await waitFor(() => expect(within(screen.getByText('portal_new_dashboard').closest('tr')!).getByText('5')).toBeInTheDocument());
  });

  it('tenant override with X-Tenant-Id; 412 reloads the value and says "Đã có người sửa"', async () => {
    const { backend, state, user } = await renderAccess('/platform-admin/feature-toggles');
    await screen.findByText('portal_new_dashboard');
    fireEvent.click(within(screen.getByText('portal_new_dashboard').closest('tr')!).getByRole('button', { name: 'Sửa' })); // row may sit under the table's loading overlay (pointer-events: none) under CPU load
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByLabelText('Ghi đè cho một tenant'));
    await user.click(within(dialog).getByRole('combobox', { name: 'Tenant đích' }));
    await user.click((await screen.findAllByTitle('TH Demo A (th-demo-a)')).at(-1)!);
    await user.click(within(dialog).getByLabelText('Bật'));
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Thử ở TH Demo A' } });
    // someone else saved meanwhile
    state.flags[0]!.version = 7;
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(await within(dialog).findByText(/Đã có người sửa/)).toBeInTheDocument();
    await waitFor(() => expect(within(dialog).getByText('Phiên bản hiện tại: 7')).toBeInTheDocument());
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PUT', '/admin/feature-flags/portal_new_dashboard')).toHaveLength(2));
    const [stale, ok] = backend.calls('PUT', '/admin/feature-flags/portal_new_dashboard');
    expect(stale!.headers['if-match']).toBe('W/"4"');
    expect(ok!.headers['if-match']).toBe('W/"7"');
    expect(ok!.headers['x-tenant-id']).toBe(TENANTS.thDemoA.tenant_id);
    expect(ok!.body).toMatchObject({ scope: 'tenant', state: 'enabled', reason: 'Thử ở TH Demo A' });
  });
});
