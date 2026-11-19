// F052 T009, T010, T013, T014 — F052-AC-01, F052-AC-02, F052-AC-03, F052-AC-07, F052-AC-24 (TC-ORG-102, TC-ORG-104,
// TC-TEN-001, TC-TEN-003, TC-TEN-004): schools list by status, edit with If-Match and active provinces only, SIS data
// read-only, approve locked until a province is chosen, out-of-scope school → "Không tìm thấy", System Admin must pick
// one tenant. Backend = MOCK (src/test/school-lms-mock.ts).
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { tenantFilterStore } from '../../../shell/tenantFilterStore';
import { accountWith, IDS, renderSchoolLms, SCHOOL_PERMS, stopPortals, TENANTS } from '../../../test/school-lms-harness';
import { readOnlyFields, updateBody } from './schoolForm';
import { pendingReasons } from './ApproveSchoolDialog';
import type { School } from './useSchools';

afterEach(() => {
  stopPortals();
  tenantFilterStore.set([]);
});

const unitAdmin = accountWith('ad.donvi', TENANTS.pgdDemo, SCHOOL_PERMS);

describe('school form rules (pure)', () => {
  const sis = { source: 'sis', province: { code: 'vn.ha-noi', name: { vi: 'Hà Nội', en: 'Hanoi' } } } as School;
  it('SIS school: SIS data read-only, province editable only while missing (F052-AC-02)', () => {
    expect([...readOnlyFields(sis)].sort()).toEqual(['address', 'grade_max', 'grade_min', 'name_en', 'name_vi', 'province_code', 'school_level']);
    expect(readOnlyFields({ ...sis, province: undefined }).has('province_code')).toBe(false);
    expect(readOnlyFields({ ...sis, source: 'manual' }).size).toBe(0);
  });
  it('PATCH body has changed editable fields only', () => {
    const s = { school_id: 'x', source: 'manual', name: { vi: 'A' }, school_level: 'primary', grade_min: 1, grade_max: 5, status: 'active', province: undefined } as unknown as School;
    expect(updateBody(s, { name_vi: 'A', school_level: 'primary', grade_min: 1, grade_max: 5, address: null })).toBeNull();
    expect(updateBody(s, { name_vi: 'A', name_en: 'School A', school_level: 'primary', grade_min: 1, grade_max: 5, province_code: 'vn.ha-noi' })).toEqual({ name: { vi: 'A', en: 'School A' }, province_code: 'vn.ha-noi' });
  });
  it('pending reasons name the missing province (F052-AC-03)', () => {
    expect(pendingReasons({ status: 'pending_review', province: undefined })).toContain('portal.school_lms.schools.pending.noProvince');
    expect(pendingReasons({ status: 'active', province: undefined })).toEqual([]);
  });
});

describe('Trường screen', () => {
  it('lists only the schools the API returns, filtered by status (F052-AC-01)', async () => {
    const { backend } = await renderSchoolLms('/school-lms/schools', { account: unitAdmin });
    expect(await screen.findByText('TH Chờ Duyệt')).toBeInTheDocument();
    expect(screen.queryByText('THCS Demo B')).toBeNull();
    fireEvent.click(screen.getByText('Chờ duyệt', { selector: '.ant-segmented-item-label' }));
    await waitFor(() => expect(backend.requests.some((r) => r.path === '/tenant/schools' && r.search.includes('status=pending_review'))).toBe(true));
    const call = backend.calls('GET', '/tenant/schools')[0]!;
    expect(call.headers['x-tenant-id']).toBe(TENANTS.pgdDemo.tenant_id);
  });

  it('edit sends If-Match, offers active provinces only, keeps SIS data read-only (F052-AC-02, TC-ORG-102)', async () => {
    const { backend } = await renderSchoolLms(`/school-lms/schools/${IDS.schoolA}`);
    fireEvent.click(await screen.findByRole('button', { name: 'Sửa' }));
    const form = await screen.findByTestId('school-edit-form');
    expect(within(form).getByLabelText('Tên (tiếng Việt)')).toBeDisabled();
    expect(within(form).getByText('Dữ liệu do SIS cung cấp chỉ xem được ở đây; hãy sửa trong SIS hoặc file CSV.')).toBeInTheDocument();
    await waitFor(() => expect(backend.calls('GET', '/catalog/provinces')[0]?.search).toContain('status=active'));
    fireEvent.click(within(form).getByRole('button', { name: 'Lưu' }));
    // nothing editable changed → no PATCH
    await waitFor(() => expect(screen.queryByTestId('school-edit-form')).toBeNull());
    expect(backend.calls('PATCH', `/tenant/schools/${IDS.schoolA}`)).toHaveLength(0);
  });

  it('approve is locked until a province is chosen, then PATCH province + approve with If-Match (F052-AC-03, TC-ORG-104)', async () => {
    const { backend } = await renderSchoolLms(`/school-lms/schools/${IDS.schoolPending}`, { account: unitAdmin });
    fireEvent.click(await screen.findByRole('button', { name: 'Duyệt trường' }));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText(/SIS chưa gửi tỉnh\/thành/)).toBeInTheDocument();
    expect(within(dialog).getByTestId('approve-ok')).toBeDisabled();
    fireEvent.mouseDown(within(dialog).getByRole('combobox', { name: 'Tỉnh/thành' }));
    fireEvent.click(await screen.findByText('Hà Nội', { selector: '.ant-select-item-option-content' }));
    await waitFor(() => expect(within(dialog).getByTestId('approve-ok')).toBeEnabled());
    fireEvent.click(within(dialog).getByTestId('approve-ok'));
    await waitFor(() => expect(backend.calls('POST', `/tenant/schools/${IDS.schoolPending}:approve`)).toHaveLength(1));
    expect(backend.calls('PATCH', `/tenant/schools/${IDS.schoolPending}`)[0]!.body).toEqual({ province_code: 'vn.ha-noi' });
    expect(backend.calls('PATCH', `/tenant/schools/${IDS.schoolPending}`)[0]!.headers['if-match']).toBe('"v1"');
    expect(backend.calls('POST', `/tenant/schools/${IDS.schoolPending}:approve`)[0]!.headers['if-match']).toBe('"v2"');
  });

  it('a school of another tenant shows "Không tìm thấy" (F052-AC-07, TC-TEN-001)', async () => {
    const adB = accountWith('ad.truong.b', TENANTS.thcsDemoB, SCHOOL_PERMS);
    await renderSchoolLms(`/school-lms/schools/${IDS.schoolA}`, { account: adB });
    expect(await screen.findByTestId('school-lms-not-found')).toBeInTheDocument();
    expect(screen.queryByText('TH Demo A')).toBeNull();
  });

  it('System Admin must pick exactly one tenant; that tenant is sent as X-Tenant-Id (F052-AC-24, TC-TEN-004)', async () => {
    const { backend } = await renderSchoolLms('/school-lms/schools', { as: 'sa.admin' });
    expect(await screen.findByText('Chọn đúng một tenant để làm việc')).toBeInTheDocument();
    expect(backend.calls('GET', '/tenant/schools')).toHaveLength(0);
    tenantFilterStore.set([TENANTS.thDemoA.tenant_id]);
    expect(await screen.findByText('TH Demo A')).toBeInTheDocument();
    expect(backend.calls('GET', '/tenant/schools')[0]!.headers['x-tenant-id']).toBe(TENANTS.thDemoA.tenant_id);
  });
});
