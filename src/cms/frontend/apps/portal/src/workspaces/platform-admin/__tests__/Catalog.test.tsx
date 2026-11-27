// F057 T029/T030/T031 — F057-AC-11, F057-AC-12, F057-AC-13, F057-AC-03 (TC-ORG-101, TC-ORG-103, TC-CMS-TEN-03):
// countries (only on/off + bilingual name with a reason, no delete), provinces (`status=all`, search, effective dates,
// successor, edit = names/official_code only, new unit + succession; edits send If-Match = row `version`, OQ F057-3),
// school catalog (System Admin: ≥ 1 tenant → `tenant_ids[]` without X-Tenant-Id, Tenant column, status filter — OQ F057-4),
// school CSV import (target tenant mandatory, dry-run with Idempotency-Key, errors per row, apply only after
// confirmation). Backend = MOCK.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { tenantFilterStore } from '../../../shell/tenantFilterStore';
import { stopPortals, TENANTS } from '../../../test/portal-harness';
import { IDS, platformState, renderPlatform } from '../../../test/platform-backend';

// whole-portal flows with several Ant Design selects and dialogs are slow on the throttled shared CPU (2 workers max)
// the cursor test of countries takes ~230 s alone on a shared CPU (55 rows, user-event on Ant Design): 600 s like ChatAssets
vi.setConfig({ testTimeout: 600_000 });
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

const UUIDV7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

async function openTab(name: string, state?: ReturnType<typeof platformState>) {
  const r = await renderPlatform('/platform-admin/catalog', { state });
  await screen.findByRole('heading', { level: 1, name: 'Danh mục quốc gia, tỉnh, trường' });
  await r.user.click(screen.getByRole('tab', { name }));
  return r;
}

describe('Quốc gia (F057-AC-11)', () => {
  it('lists countries; toggling status needs a reason and calls updateCountry with If-Match = row version; no delete action', async () => {
    const { backend, user } = await openTab('Quốc gia');
    const section = await screen.findByRole('region', { name: 'Quốc gia' });
    await within(section).findByText('Việt Nam');
    // a refetch keeps the old rows under the table spinner (`.ant-spin-blur`, pointer-events: none): click once it is done
    await waitFor(() => expect(section.querySelector('.ant-spin-blur')).toBeNull());
    const row = within(section).getByText('Việt Nam').closest('tr')!;
    expect(within(row).queryByRole('button', { name: /xóa/i })).toBeNull();
    await user.click(within(row).getByRole('button', { name: 'Tắt' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    expect(await within(dialog).findByText('Vui lòng nhập lý do.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Thử nghiệm môi trường local' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('PATCH', '/admin/catalog/countries/VN')).toHaveLength(1));
    const req = backend.calls('PATCH', '/admin/catalog/countries/VN')[0]!;
    expect(req.body).toEqual({ status: 'inactive', reason: 'Thử nghiệm môi trường local' });
    expect(req.headers['if-match']).toBe('W/"1"');
    expect(await screen.findByText('Đã lưu.')).toBeInTheDocument();
  });
});

describe('Tỉnh/thành (F057-AC-12)', () => {
  it('status=all with filter, search, effective dates and successor; edit sends only names/official_code', async () => {
    const { backend, user } = await openTab('Tỉnh/thành');
    expect(await screen.findByText('Hà Tây')).toBeInTheDocument();
    expect(backend.calls('GET', '/catalog/provinces').some((r) => new URLSearchParams(r.search).get('status') === 'all')).toBe(true);
    const old = screen.getByText('Hà Tây').closest('tr')!;
    expect(within(old).getByText('vn.ha-noi')).toBeInTheDocument();
    expect(within(old).getByText('→ 2008-08-01')).toBeInTheDocument();
    await user.click(screen.getByText('Đang dùng', { selector: '.ant-segmented-item-label' }));
    await waitFor(() => expect(backend.calls('GET', '/catalog/provinces').some((r) => new URLSearchParams(r.search).get('status') === 'active')).toBe(true));
    await waitFor(() => expect(screen.queryByText('Hà Tây')).toBeNull());
    await user.click(screen.getByText('Tất cả', { selector: '.ant-segmented-item-label' }));
    await screen.findByText('Hà Tây');
    fireEvent.change(screen.getByRole('searchbox', { name: 'Tìm theo tên hoặc mã' }), { target: { value: 'hà nội' } });
    await waitFor(() => expect(screen.queryByText('Hà Tây')).toBeNull());
    const row = screen.getByText('vn.ha-noi').closest('tr')!;
    await user.click(within(row).getByRole('button', { name: 'Sửa' }));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByLabelText('Mã chính thức')).toHaveAttribute('maxlength', '16');
    expect(within(dialog).queryByLabelText('Mã')).toBeNull();
    fireEvent.change(within(dialog).getByLabelText('Mã chính thức'), { target: { value: '01' } });
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Đối chiếu Nghị quyết' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PATCH', `/admin/catalog/provinces/${IDS.provinceHn}`)).toHaveLength(1));
    expect(backend.calls('PATCH', `/admin/catalog/provinces/${IDS.provinceHn}`)[0]!.body).toEqual({
      name: { vi: 'Hà Nội', en: 'Hanoi' },
      short_name: { vi: 'Hà Nội', en: 'Hanoi' },
      official_code: '01',
      reason: 'Đối chiếu Nghị quyết',
    });
    expect(backend.calls('PATCH', `/admin/catalog/provinces/${IDS.provinceHn}`)[0]!.headers['if-match']).toBe('W/"1"');
  });

  it('boundary change = new unit (createProvince) + succession (createProvinceSuccession), both with Idempotency-Key', async () => {
    const { backend, user } = await openTab('Tỉnh/thành');
    await screen.findByText('Hà Tây');
    await user.click(screen.getByRole('button', { name: 'Ghi kế thừa' }));
    const dialog = await screen.findByRole('dialog');
    const pick = async (label: string, title: string) => {
      await user.click(within(dialog).getByLabelText(label));
      await user.click((await screen.findAllByTitle(title)).at(-1)!);
    };
    await pick('Đơn vị cũ', 'Hà Tây (vn.old.ha-tay)');
    await pick('Đơn vị mới', 'Hà Nội (vn.ha-noi)');
    const date = within(dialog).getByLabelText('Ngày hiệu lực');
    await user.click(date);
    fireEvent.change(date, { target: { value: '01/08/2008' } });
    fireEvent.keyDown(date, { key: 'Enter', code: 'Enter', keyCode: 13 });
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Sáp nhập theo nghị quyết' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/catalog/province-successions')).toHaveLength(1));
    const req = backend.calls('POST', '/admin/catalog/province-successions')[0]!;
    expect(req.body).toEqual({ old_province_id: IDS.provinceOld, new_province_id: IDS.provinceHn, effective_date: '2008-08-01', kind: 'merged', reason: 'Sáp nhập theo nghị quyết' });
    expect(req.headers['idempotency-key']).toMatch(UUIDV7);
    expect(backend.requests.filter((r) => r.method === 'PATCH')).toHaveLength(0);
  });
});

describe('Bảng danh mục theo trang cursor (owner 2026-11-23, API_DESIGN §5)', () => {
  const province = (i: number) => ({
    province_id: `0193a4d2-7700-7000-8000-${String(i).padStart(12, '0')}`, code: `vn.p${String(i).padStart(3, '0')}`, country_code: 'VN', official_code: null,
    name: { vi: `Tỉnh ${String(i).padStart(3, '0')}`, en: `Province ${i}` }, short_name: { vi: 'T', en: 'P' }, province_type: 'province',
    effective_from: null, effective_to: null, status: 'active', successor_code: null,
  });

  it('countries: first page only, "Tải thêm" asks page 2 with the cursor and appends it without duplicates', async () => {
    const state = platformState();
    state.countries = Array.from({ length: 55 }, (_, i) => ({ country_code: `C${String(i).padStart(2, '0')}`, name: { vi: `Nước ${String(i).padStart(2, '0')}`, en: `Country ${i}` }, default_locale: 'vi-VN', status: 'active' }));
    const { backend, user } = await openTab('Quốc gia', state);
    const section = await screen.findByRole('region', { name: 'Quốc gia' });
    const rows = () => section.querySelectorAll('tbody tr[data-row-key]');
    await waitFor(() => expect(rows()).toHaveLength(50));
    expect(new URLSearchParams(backend.calls('GET', '/catalog/countries').at(-1)!.search).get('cursor')).toBeNull();
    await user.click(within(section).getByRole('button', { name: 'Tải thêm' }));
    await waitFor(() => expect(rows()).toHaveLength(55));
    expect(new Set([...rows()].map((r) => r.getAttribute('data-row-key'))).size).toBe(55);
    expect(new URLSearchParams(backend.calls('GET', '/catalog/countries').at(-1)!.search).get('cursor')).toBe('c50');
    expect(within(section).queryByRole('button', { name: 'Tải thêm' })).toBeNull();
  });

  it('provinces: the table pages with the cursor; the succession dialog reads every page (dropdown)', async () => {
    const state = platformState();
    state.provinces = [...state.provinces, ...Array.from({ length: 120 }, (_, i) => province(i))];
    const { backend, user } = await openTab('Tỉnh/thành', state);
    const section = await screen.findByRole('region', { name: 'Tỉnh/thành' });
    const rows = () => section.querySelectorAll('tbody tr[data-row-key]');
    await waitFor(() => expect(rows()).toHaveLength(50));
    await user.click(within(section).getByRole('button', { name: 'Tải thêm' }));
    await waitFor(() => expect(rows()).toHaveLength(100));
    expect(new Set([...rows()].map((r) => r.getAttribute('data-row-key'))).size).toBe(100);
    // dropdown data: every page of status=all, limit 100, until has_more = false (122 rows → 2 requests)
    await waitFor(() => {
      const all = backend.calls('GET', '/catalog/provinces').filter((r) => new URLSearchParams(r.search).get('limit') === '100');
      expect(all.map((r) => new URLSearchParams(r.search).get('cursor'))).toEqual([null, 'c100']);
    });
  });
});

describe('Danh mục trường (F057-AC-13)', () => {
  it('System Admin: at least one tenant, then listCatalogSchools with tenant_ids[] (no X-Tenant-Id), Tenant column, status filter', async () => {
    const { backend, user } = await openTab('Danh mục trường');
    expect(await screen.findByText('Chọn ít nhất một tenant ở bộ lọc tenant để xem danh mục trường.')).toBeInTheDocument();
    expect(backend.calls('GET', '/catalog/schools')).toHaveLength(0);
    tenantFilterStore.set([TENANTS.thDemoA.tenant_id, TENANTS.pgdDemo.tenant_id]);
    expect(await screen.findByText('Trường TH A')).toBeInTheDocument();
    const first = backend.calls('GET', '/catalog/schools')[0]!;
    expect(first.headers['x-tenant-id']).toBeUndefined();
    expect(new URLSearchParams(first.search).getAll('tenant_ids')).toEqual([TENANTS.thDemoA.tenant_id, TENANTS.pgdDemo.tenant_id]);
    const other = screen.getByText('Trường TH 2').closest('tr')!;
    expect(within(other).getByText('Phòng GD Demo')).toBeInTheDocument();
    await user.click(screen.getByRole('combobox', { name: 'Cấp học' }));
    await user.click((await screen.findAllByTitle('Tiểu học')).at(-1)!);
    await waitFor(() => expect(backend.calls('GET', '/catalog/schools').some((r) => new URLSearchParams(r.search).get('school_level') === 'primary')).toBe(true));
    await user.click(screen.getByRole('combobox', { name: 'Trạng thái' }));
    await user.click((await screen.findAllByTitle('Chờ duyệt')).at(-1)!);
    await waitFor(() => expect(backend.calls('GET', '/catalog/schools').some((r) => new URLSearchParams(r.search).get('status') === 'pending_review')).toBe(true));
    await waitFor(() => expect(screen.queryByText('Trường TH 2')).toBeNull());
    expect(screen.getByText('Trường TH A')).toBeInTheDocument();
  });
});

describe('Import trường CSV (F057-AC-13, F057-AC-03)', () => {
  it('target tenant mandatory; dry-run → errors per row → apply only after confirmation', async () => {
    const { backend, user } = await openTab('Import trường (CSV)');
    await user.click(await screen.findByRole('button', { name: 'Kiểm tra (dry-run)' }));
    expect(await screen.findByText('Chọn một tenant đích.')).toBeInTheDocument();
    expect(await screen.findByText('Chọn một tệp CSV.')).toBeInTheDocument();
    expect(backend.calls('POST', '/admin/schools:import')).toHaveLength(0);

    await user.click(screen.getByRole('combobox', { name: 'Tenant' }));
    await user.click((await screen.findAllByTitle('Phòng GD Demo (pgd-demo)')).at(-1)!);
    const file = new File(['school_external_id,school_code,name,province_code\n'], 'truong.csv', { type: 'text/csv' });
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    await user.upload(input, file);
    expect(screen.getByRole('button', { name: 'Áp dụng' })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: 'Kiểm tra (dry-run)' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/schools:import')).toHaveLength(1));
    const dry = backend.calls('POST', '/admin/schools:import')[0]!;
    expect(dry.headers['x-tenant-id']).toBe(TENANTS.pgdDemo.tenant_id);
    expect(dry.headers['idempotency-key']).toMatch(UUIDV7);
    expect(String(dry.body)).toContain('name="dry_run"\r\n\r\ntrue');
    expect(String(dry.body)).toContain('filename="truong.csv"');

    const result = await screen.findByRole('region', { name: 'Kết quả dry-run' });
    expect(within(result).getByText('PROVINCE_INACTIVE')).toBeInTheDocument();
    expect(within(result).getByText('province_code')).toBeInTheDocument();
    expect(within(result).getByText('Tổng 3 dòng · tạo 0 · cập nhật 0 · lỗi 1')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Áp dụng' }));
    const confirm = await screen.findByRole('dialog');
    expect(within(confirm).getByText('Dry-run: 3 dòng, 1 dòng lỗi. Dòng lỗi không được áp dụng.')).toBeInTheDocument();
    expect(backend.calls('POST', '/admin/schools:import')).toHaveLength(1);
    await user.click(within(confirm).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/schools:import')).toHaveLength(2));
    const apply = backend.calls('POST', '/admin/schools:import')[1]!;
    expect(String(apply.body)).toContain('name="dry_run"\r\n\r\nfalse');
    expect(apply.headers['idempotency-key']).not.toBe(dry.headers['idempotency-key']);
    expect(await screen.findByRole('region', { name: 'Kết quả áp dụng' })).toBeInTheDocument();
  });
});
