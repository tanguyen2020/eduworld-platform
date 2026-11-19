// F057 T022/T023 — F057-AC-07, F057-AC-08 (TC-CMS-CLN-01, TC-TEN-020): wizard source tenant → type (no student data)
// → record ids (UUID, ≤ maxItems) → one target ≠ source (the source is not offered, no "all tenants") → preview step
// skipped while previewTenantClone does not exist (said on screen) → createTenantClone with Idempotency-Key → job
// polled to a final status, id_map and errors shown, never re-run by itself. Backend = MOCK.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { stopPortals, TENANTS } from '../../../test/portal-harness';
import { IDS, platformState, renderPlatform } from '../../../test/platform-backend';

// whole-portal flows with several Ant Design selects and dialogs are slow on the throttled shared CPU (2 workers max)
vi.setConfig({ testTimeout: 300_000 });
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => stopPortals());

const UUIDV7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
type User = Awaited<ReturnType<typeof renderPlatform>>['user'];

async function choose(user: User, label: string, title: string) {
  await user.click(screen.getByRole('combobox', { name: label }));
  await user.click((await screen.findAllByTitle(title)).at(-1)!);
}

async function openClone(state = platformState()) {
  const r = await renderPlatform('/platform-admin/tenant-data', { state });
  await r.user.click(await screen.findByRole('tab', { name: 'Clone giữa tenant' }));
  await screen.findByText('Không bao giờ clone dữ liệu học sinh (học sinh, ghi danh, lượt làm bài, kết quả, chat, mức thành thạo).');
  return r;
}

async function fillSource(user: User) {
  await choose(user, 'Tenant nguồn', 'TH Demo A (th-demo-a)');
  await choose(user, 'Loại dữ liệu', 'Bộ câu hỏi');
  const ids = screen.getByRole('combobox', { name: 'Mã bản ghi (UUID)' });
  fireEvent.change(ids, { target: { value: IDS.questionSet } });
  fireEvent.keyDown(ids, { key: 'Enter', code: 'Enter', keyCode: 13 });
  await user.click(screen.getByRole('button', { name: 'Tiếp' }));
}

describe('Clone giữa tenant (F057-AC-07)', () => {
  it('runs the wizard, sends one createTenantClone with Idempotency-Key and follows the job to succeeded', async () => {
    const { backend, user } = await openClone();
    await fillSource(user);
    await choose(user, 'Tenant đích', 'Phòng GD Demo (pgd-demo)');
    await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    expect(await screen.findByText(/Chưa có API xem trước đồ thị phụ thuộc/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    expect(await screen.findByText('Clone 1 bản ghi loại Bộ câu hỏi từ TH Demo A sang Phòng GD Demo.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Chạy clone' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/tenant-clones')).toHaveLength(1));
    const req = backend.calls('POST', '/admin/tenant-clones')[0]!;
    expect(req.body).toEqual({ source_tenant_id: TENANTS.thDemoA.tenant_id, target_tenant_id: TENANTS.pgdDemo.tenant_id, resource_type: 'question_set', resource_ids: [IDS.questionSet] });
    expect(req.headers['idempotency-key']).toMatch(UUIDV7);
    expect(req.headers['x-tenant-id']).toBeUndefined();
    await waitFor(() => expect(screen.getByTestId('clone-status')).toHaveTextContent('Thành công'), { timeout: 15_000 });
    expect(backend.calls('GET', `/admin/tenant-clones/${IDS.job}`).length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText(IDS.cloneCopy)).toBeInTheDocument();
    expect(screen.getByText('Đã clone: 1')).toBeInTheDocument();
  });

  it('partially_failed: errors listed, polling stops, no automatic re-run', async () => {
    const state = platformState();
    state.cloneFinal = 'partially_failed';
    const { backend, user } = await openClone(state);
    await fillSource(user);
    await choose(user, 'Tenant đích', 'Phòng GD Demo (pgd-demo)');
    await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    await user.click(await screen.findByRole('button', { name: 'Tiếp' }));
    await user.click(await screen.findByRole('button', { name: 'Chạy clone' }));
    await waitFor(() => expect(screen.getByTestId('clone-status')).toHaveTextContent('Thành công một phần'), { timeout: 15_000 });
    expect(screen.getByText('learning step 2: media missing')).toBeInTheDocument();
    expect(screen.getByText(/Job không tự chạy lại/)).toBeInTheDocument();
    const polls = backend.calls('GET', `/admin/tenant-clones/${IDS.job}`).length;
    await new Promise((r) => setTimeout(r, 2500));
    expect(backend.calls('GET', `/admin/tenant-clones/${IDS.job}`)).toHaveLength(polls);
    expect(backend.calls('POST', '/admin/tenant-clones')).toHaveLength(1);
  });
});

describe('Quy tắc clone (F057-AC-08)', () => {
  it('no student data type, record ids must be UUIDs, target ≠ source and no "all tenants" option', async () => {
    const { user, backend } = await openClone();
    await user.click(screen.getByRole('combobox', { name: 'Loại dữ liệu' }));
    const listbox = await screen.findByRole('listbox');
    expect(within(listbox.parentElement!).queryByText('Học sinh đã dọn')).toBeNull();
    expect(screen.queryByTitle('Học sinh đã dọn')).toBeNull();
    expect(screen.queryByTitle('Tài khoản đã gộp')).toBeNull();
    await user.click((await screen.findAllByTitle('Câu hỏi')).at(-1)!);
    await choose(user, 'Tenant nguồn', 'TH Demo A (th-demo-a)');
    const ids = screen.getByRole('combobox', { name: 'Mã bản ghi (UUID)' });
    fireEvent.change(ids, { target: { value: 'not-a-uuid' } });
    fireEvent.keyDown(ids, { key: 'Enter', code: 'Enter', keyCode: 13 });
    await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    expect(await screen.findByText('Mã bản ghi phải là UUID hợp lệ.')).toBeInTheDocument();
    await user.click(screen.getByTitle('not-a-uuid').parentElement!.querySelector('.ant-select-selection-item-remove')!);
    fireEvent.change(ids, { target: { value: IDS.question } });
    fireEvent.keyDown(ids, { key: 'Enter', code: 'Enter', keyCode: 13 });
    await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    await user.click(await screen.findByRole('combobox', { name: 'Tenant đích' }));
    await screen.findAllByTitle('Phòng GD Demo (pgd-demo)');
    const options = document.querySelectorAll('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option');
    const titles = [...options].map((o) => o.getAttribute('title'));
    expect(titles).not.toContain('TH Demo A (th-demo-a)');
    expect(titles.some((x) => /tất cả|mọi tenant/i.test(x ?? ''))).toBe(false);
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    expect(await screen.findByText('Chọn một tenant đích.')).toBeInTheDocument();
    expect(backend.calls('POST', '/admin/tenant-clones')).toHaveLength(0);
  });
});
