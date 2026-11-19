// F057 T034/T035/T036/T051 — F057-AC-14, F057-AC-15, F057-AC-16, F057-AC-23 (TC-AUTH-020, TC-AUTH-021, TC-AUTH-092):
// student cleanup settings (schema bounds, mandatory reason, low-days warning, PUT + If-Match), cleanup runs (counts,
// "Xác nhận" → approveStudentCleanupRun with Idempotency-Key + X-Tenant-Id of the run; "Hủy" → reason 3..500 →
// cancelStudentCleanupRun, same Idempotency-Key on retry, run "Đã hủy" — OQ F057-2), chat retention
// (blocked under minimum_days, RETENTION_BELOW_MINIMUM by code), platform curfew ("Mặc định", next-day window, HH:MM
// errors on the field, reason, confirmation, If-Match, 412/422 by code). Backend = MOCK.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { problem, stopPortals, TENANTS } from '../../../test/portal-harness';
import { IDS, renderPlatform } from '../../../test/platform-backend';

// whole-portal flows with several Ant Design selects and dialogs are slow on the throttled shared CPU (2 workers max)
vi.setConfig({ testTimeout: 300_000 });
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => stopPortals());

const UUIDV7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const card = (title: string) => screen.getByText(title, { selector: '.ant-card-head-title' }).closest('.ant-card') as HTMLElement;

async function open() {
  const r = await renderPlatform('/platform-admin/system-settings');
  await screen.findByRole('heading', { level: 1, name: 'Cấu hình hệ thống' });
  await waitFor(() => expect(within(card('Dọn học sinh không đăng nhập')).getByLabelText('Số ngày không đăng nhập')).toHaveValue('180'));
  return r;
}

describe('Dọn học sinh (F057-AC-14)', () => {
  it('reason mandatory; low-days warning; PUT with If-Match and schema fields', async () => {
    const { backend, user } = await open();
    const c = card('Dọn học sinh không đăng nhập');
    const days = within(c).getByLabelText('Số ngày không đăng nhập');
    fireEvent.change(days, { target: { value: '30' } });
    expect(await within(c).findByText('Số ngày dưới 60 dễ dọn nhầm học sinh nghỉ hè. Vẫn lưu được kèm lý do.')).toBeInTheDocument();
    await user.click(within(c).getByRole('button', { name: 'Lưu' }));
    expect(await within(c).findByText('Trường này bắt buộc.')).toBeInTheDocument();
    expect(backend.calls('PUT', '/admin/settings/student-cleanup')).toHaveLength(0);
    fireEvent.change(within(c).getByLabelText('Lý do'), { target: { value: 'Rút ngắn cho năm học mới' } });
    await user.click(within(c).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PUT', '/admin/settings/student-cleanup')).toHaveLength(1));
    const req = backend.calls('PUT', '/admin/settings/student-cleanup')[0]!;
    expect(req.headers['if-match']).toBe('W/"1"');
    expect(req.body).toEqual({ default_days: 30, notice_days: 14, allow_tenant_override: true, approval_threshold_percent: 10, reason: 'Rút ngắn cho năm học mới' });
  });

  it('switching cleanup off sends default_days = null', async () => {
    const { backend, user } = await open();
    const c = card('Dọn học sinh không đăng nhập');
    await user.click(within(c).getByRole('switch', { name: 'Bật dọn học sinh' }));
    fireEvent.change(within(c).getByLabelText('Lý do'), { target: { value: 'Tạm dừng dọn' } });
    await user.click(within(c).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PUT', '/admin/settings/student-cleanup')).toHaveLength(1));
    expect((backend.calls('PUT', '/admin/settings/student-cleanup')[0]!.body as { default_days: unknown }).default_days).toBeNull();
  });
});

describe('Lần dọn chờ xác nhận (F057-AC-15)', () => {
  it('lists runs with counts; "Xác nhận" → approveStudentCleanupRun with Idempotency-Key + X-Tenant-Id', async () => {
    const { backend, user } = await open();
    const c = card('Lần dọn chờ xác nhận');
    const row = (await within(c).findByText('Chờ xác nhận')).closest('tr')!;
    expect(within(row).getByText('TH Demo A')).toBeInTheDocument();
    expect(within(row).getByText('42')).toBeInTheDocument();
    expect(within(row).getByRole('button', { name: 'Hủy lần dọn' })).toBeInTheDocument();
    // a finished run has neither action
    const done = within(c).getByText('Hoàn tất').closest('tr')!;
    expect(within(done).queryByRole('button', { name: 'Hủy lần dọn' })).toBeNull();
    await user.click(within(row).getByRole('button', { name: 'Xác nhận' }));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('Worker sẽ dọn 42 học sinh (xóa mềm, khôi phục được khi đăng nhập lại).')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/student-cleanup/runs/${IDS.runAwaiting}:approve`)).toHaveLength(1));
    const req = backend.calls('POST', `/admin/student-cleanup/runs/${IDS.runAwaiting}:approve`)[0]!;
    expect(req.headers['idempotency-key']).toMatch(UUIDV7);
    expect(req.headers['x-tenant-id']).toBe(TENANTS.thDemoA.tenant_id);
    expect(await within(c).findByText('Đã xác nhận')).toBeInTheDocument();
  });

  it('"Hủy lần dọn" → reason 3..500 → cancelStudentCleanupRun; retry keeps the Idempotency-Key; run "Đã hủy" (OQ F057-2)', async () => {
    const { backend, user } = await open();
    let failed = false;
    const path = `/admin/student-cleanup/runs/${IDS.runAwaiting}:cancel`;
    backend.overrides.unshift((r) => (r.method === 'POST' && r.path === path && !failed ? ((failed = true), problem(500, 'INTERNAL')) : undefined));
    const c = card('Lần dọn chờ xác nhận');
    const row = (await within(c).findByText('Chờ xác nhận')).closest('tr')!;
    await user.click(within(row).getByRole('button', { name: 'Hủy lần dọn' }));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText(/không học sinh nào bị dọn \(42 học sinh/)).toBeInTheDocument();
    const reason = within(dialog).getByLabelText('Lý do');
    expect(reason).toHaveAttribute('maxlength', '500');
    fireEvent.change(reason, { target: { value: 'ab' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    expect(await within(dialog).findByText('Tối thiểu 3 ký tự.')).toBeInTheDocument();
    expect(backend.calls('POST', path)).toHaveLength(0);
    fireEvent.change(reason, { target: { value: 'Danh sách học sinh chưa đúng' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    expect(await within(dialog).findByText('Mã truy vết: trace-internal')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('POST', path)).toHaveLength(2));
    const [first, second] = backend.calls('POST', path);
    expect(first!.headers['idempotency-key']).toMatch(UUIDV7);
    expect(second!.headers['idempotency-key']).toBe(first!.headers['idempotency-key']);
    expect(second!.headers['x-tenant-id']).toBe(TENANTS.thDemoA.tenant_id);
    expect(second!.body).toEqual({ reason: 'Danh sách học sinh chưa đúng' });
    expect(await screen.findByText('Đã hủy lần dọn.')).toBeInTheDocument();
    expect(await within(c).findByText('Đã hủy')).toBeInTheDocument();
    expect(backend.calls('POST', `/admin/student-cleanup/runs/${IDS.runAwaiting}:approve`)).toHaveLength(0);
  });
});

describe('Thời hạn chat gameplay (F057-AC-16)', () => {
  it('blocks a value under minimum_days; RETENTION_BELOW_MINIMUM from the server lands on the field', async () => {
    const { backend, user, state } = await open();
    const c = card('Thời hạn lưu chat gameplay');
    const days = within(c).getByLabelText('Số ngày lưu chat gameplay');
    await waitFor(() => expect(days).toHaveValue('7'));
    fireEvent.change(days, { target: { value: '2' } });
    fireEvent.change(within(c).getByLabelText('Lý do'), { target: { value: 'Giảm lưu trữ' } });
    await user.click(within(c).getByRole('button', { name: 'Lưu' }));
    expect(await within(c).findAllByText('Tối thiểu 3 ngày.')).not.toHaveLength(0);
    expect(backend.calls('PUT', '/admin/settings/chat-retention')).toHaveLength(0);
    // the server minimum rose meanwhile: 4 is accepted by the page, refused by the server by code
    state.chat.minimum_days = 5;
    backend.overrides.unshift((r) => (r.method === 'PUT' && r.path === '/admin/settings/chat-retention' ? problem(422, 'RETENTION_BELOW_MINIMUM') : undefined));
    fireEvent.change(days, { target: { value: '4' } });
    await user.click(within(c).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PUT', '/admin/settings/chat-retention')).toHaveLength(1));
    expect(backend.calls('PUT', '/admin/settings/chat-retention')[0]!.body).toEqual({ gameplay_chat_retention_days: 4, ephemeral_retention_hours: 24, allow_tenant_override: true, reason: 'Giảm lưu trữ' });
    expect(await within(c).findAllByText('Giá trị thấp hơn mức tối thiểu cho phép.')).not.toHaveLength(0);
    expect(within(c).getByText('Mã truy vết: trace-retention_below_minimum')).toBeInTheDocument();
  });
});

describe('Giờ giới nghiêm platform (F057-AC-23)', () => {
  it('shows default + next-day window; HH:MM error on the field; reason; confirmation; PUT with If-Match', async () => {
    const { backend, user } = await open();
    const c = card('Giờ giới nghiêm platform');
    expect(within(c).getByText('Mặc định')).toBeInTheDocument();
    expect(within(c).getByTestId('curfew-range')).toHaveTextContent('Từ 22:00 đến 06:00 hôm sau');
    expect(within(c).getByText('Cảnh báo trước khi tới giờ: 10 phút (chỉ đọc)')).toBeInTheDocument();
    const start = within(c).getByLabelText('Bắt đầu (HH:MM)');
    fireEvent.change(start, { target: { value: '25:00' } });
    await user.click(within(c).getByRole('button', { name: 'Lưu' }));
    expect(await within(c).findAllByText('Nhập giờ dạng HH:MM (00:00–23:59).')).not.toHaveLength(0);
    fireEvent.change(start, { target: { value: '21:00' } });
    fireEvent.change(within(c).getByLabelText('Kết thúc (HH:MM)'), { target: { value: '06:30' } });
    expect(within(c).getByTestId('curfew-range')).toHaveTextContent('Từ 21:00 đến 06:30 hôm sau');
    await user.click(within(c).getByRole('button', { name: 'Lưu' }));
    expect(await within(c).findByText('Trường này bắt buộc.')).toBeInTheDocument();
    fireEvent.change(within(c).getByLabelText('Lý do'), { target: { value: 'Theo khuyến nghị an toàn' } });
    await user.click(within(c).getByRole('button', { name: 'Lưu' }));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('Trong khung giờ này, mọi học sinh của mọi tenant bị chặn đăng nhập game, game session và chat.')).toBeInTheDocument();
    expect(backend.calls('PUT', '/admin/settings/curfew')).toHaveLength(0);
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('PUT', '/admin/settings/curfew')).toHaveLength(1));
    const req = backend.calls('PUT', '/admin/settings/curfew')[0]!;
    expect(req.headers['if-match']).toBe('W/"1"');
    expect(req.body).toEqual({ start: '21:00', end: '06:30', reason: 'Theo khuyến nghị an toàn' });
    await waitFor(() => expect(within(c).queryByText('Mặc định')).toBeNull());
  });

  it('422 field errors land on start/end; 412 shows the version message with trace_id', async () => {
    const { backend, user } = await open();
    const c = card('Giờ giới nghiêm platform');
    let n = 0;
    backend.overrides.unshift((r) => {
      if (r.method !== 'PUT' || r.path !== '/admin/settings/curfew') return undefined;
      n++;
      return n === 1 ? problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'end', code: 'CURFEW_WINDOW_INVALID' }] }) : problem(412, 'VERSION_MISMATCH');
    });
    await waitFor(() => expect(within(c).getByLabelText('Bắt đầu (HH:MM)')).toHaveValue('22:00'));
    fireEvent.change(within(c).getByLabelText('Lý do'), { target: { value: 'Thử cấu hình' } });
    await user.click(within(c).getByRole('button', { name: 'Lưu' }));
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Xác nhận' }));
    const endItem = within(c).getByLabelText('Kết thúc (HH:MM)').closest('.ant-form-item')!;
    await waitFor(() => expect(within(endItem as HTMLElement).getByText('Dữ liệu nhập chưa hợp lệ. Kiểm tra các trường được đánh dấu.')).toBeInTheDocument());
    await user.click(within(c).getByRole('button', { name: 'Lưu' }));
    const dialogs = await screen.findAllByRole('dialog');
    await user.click(within(dialogs.at(-1)!).getByRole('button', { name: 'Xác nhận' }));
    expect(await within(c).findByText('Dữ liệu đã được người khác cập nhật. Tải lại rồi thử lại.')).toBeInTheDocument();
    expect(within(c).getByText('Mã truy vết: trace-version_mismatch')).toBeInTheDocument();
  });
});
