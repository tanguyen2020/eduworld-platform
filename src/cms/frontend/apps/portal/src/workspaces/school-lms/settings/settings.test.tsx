// F052 T037, T038, T039, T040, T055, T056 — F052-AC-19..F052-AC-22, F052-AC-29, F052-AC-30 (TC-AUTH-092): tenant settings
// as JSON Merge Patch with If-Match, effective cleanup days, low-value warning, RETENTION_BELOW_MINIMUM at the field
// keeping the saved value; school policy with If-Match; curfew card (platform read-only, school window, effective,
// next-day wording, edit only with the action, none for `common`, no grade option), early "must cover" check, reason,
// clear → null, 422 at the field; cleanup preview exclusion (max items, reason, Idempotency-Key). Backend = MOCK.
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { accountWith, IDS, READ_ONLY_PERMS, renderSchoolLms, SCHOOL_PERMS, stopPortals, TENANTS } from '../../../test/school-lms-harness';
import { MAX_EXCLUDE } from '../student-cleanup/ExcludeDialog';
import { crossesMidnight, curfewCovers } from './curfewCovers';
import { cleanupWarning, mergePatch } from './tenantSettingsForm';
import type { TenantSettings } from './useSettings';

afterEach(() => stopPortals());

const PLATFORM = { start: '21:30', end: '06:30' };
const tenantAdmin = () => accountWith('ad.donvi', TENANTS.thDemoA, [...SCHOOL_PERMS, 'tenant.student_cleanup.configure', 'tenant.chat_retention.configure']);

describe('pure helpers', () => {
  it('curfew window must contain the whole platform window, across midnight (F052-AC-30)', () => {
    expect(curfewCovers({ start: '21:00', end: '07:00' }, PLATFORM)).toBe(true);
    expect(curfewCovers({ start: '21:30', end: '06:30' }, PLATFORM)).toBe(true);
    expect(curfewCovers({ start: '22:00', end: '06:30' }, PLATFORM)).toBe(false);
    expect(curfewCovers({ start: '21:00', end: '06:00' }, PLATFORM)).toBe(false);
    expect(curfewCovers({ start: '20:00', end: '23:00' }, PLATFORM)).toBe(false);
    expect(curfewCovers({ start: '2100', end: '07:00' }, PLATFORM)).toBe(false);
    expect(crossesMidnight(PLATFORM)).toBe(true);
    expect(crossesMidnight({ start: '06:00', end: '07:00' })).toBe(false);
  });
  it('merge patch of changed editable fields only; warning below the threshold (F052-AC-19)', () => {
    const before = { inactive_student_cleanup_days: 180, effective_cleanup_days: 180, gameplay_chat_retention_days: 7, default_locale: 'vi-VN' } as TenantSettings;
    const after = { ...before, inactive_student_cleanup_days: 30, gameplay_chat_retention_days: 2 };
    expect(mergePatch(before, after, new Set(['inactive_student_cleanup_days']))).toEqual({ inactive_student_cleanup_days: 30 });
    expect(mergePatch(before, before, new Set(['inactive_student_cleanup_days', 'default_locale']))).toEqual({});
    expect([cleanupWarning(30), cleanupWarning(0), cleanupWarning(180), cleanupWarning(null)]).toEqual([true, false, false, false]);
  });
  it('exclusion batch limit mirrors CleanupExclusionRequest.maxItems', () => {
    expect(MAX_EXCLUDE).toBe(500);
  });
});

describe('Cài đặt đơn vị (F052-AC-19, F052-AC-20)', () => {
  it('Merge Patch + If-Match, effective cleanup days, warning; RETENTION_BELOW_MINIMUM at the field keeps the saved value', async () => {
    const { backend } = await renderSchoolLms('/school-lms/settings', { account: tenantAdmin() });
    const card = await screen.findByTestId('tenant-settings');
    expect(within(card).getByText('180 ngày')).toBeInTheDocument();
    fireEvent.change(within(card).getByLabelText('Số ngày không đăng nhập trước khi dọn học sinh'), { target: { value: '30' } });
    expect(await within(card).findByText(/dọn nhầm học sinh nghỉ hè/)).toBeInTheDocument();
    fireEvent.click(within(card).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PATCH', '/tenant/settings')).toHaveLength(1));
    const first = backend.calls('PATCH', '/tenant/settings')[0]!;
    expect(first.body).toEqual({ inactive_student_cleanup_days: 30 });
    expect(first.headers['if-match']).toBe('"v1"');
    expect(first.headers['content-type']).toContain('application/merge-patch+json');
    fireEvent.change(within(card).getByLabelText('Thời hạn lưu chat trong trận (ngày)'), { target: { value: '2' } });
    fireEvent.click(within(card).getByRole('button', { name: 'Lưu' }));
    expect(await within(card).findByText('Thời hạn lưu chat gameplay thấp hơn mức tối thiểu cho phép.')).toBeInTheDocument();
    expect(within(card).getByLabelText('Thời hạn lưu chat trong trận (ngày)')).toHaveValue('7');
    expect(backend.calls('PATCH', '/tenant/settings')[1]!.headers['if-match']).toBe('"v2"');
  });

  it('cleanup days not editable without tenant.student_cleanup.configure', async () => {
    await renderSchoolLms('/school-lms/settings', { account: accountWith('ad.truong.a2', TENANTS.thDemoA, SCHOOL_PERMS.filter((p) => !p.endsWith('.configure'))) });
    const card = await screen.findByTestId('tenant-settings');
    expect(within(card).getByLabelText('Số ngày không đăng nhập trước khi dọn học sinh')).toBeDisabled();
  });
});

describe('Cài đặt trường + giờ giới nghiêm (F052-AC-22, F052-AC-29, F052-AC-30, TC-AUTH-092)', () => {
  const openSchoolTab = async () => fireEvent.click(await screen.findByRole('tab', { name: 'Cài đặt trường' }));

  it('platform curfew read-only, school window empty, effective = platform, next-day wording, no grade option', async () => {
    await renderSchoolLms('/school-lms/settings');
    await openSchoolTab();
    const card = await screen.findByTestId('curfew-card');
    expect(within(card).getByTestId('platform-curfew')).toHaveTextContent('21:30 đến 06:30 hôm sau (giờ Việt Nam)');
    expect(within(card).getByTestId('school-curfew')).toHaveTextContent('Chưa đặt (chỉ áp giờ của EduWorld)');
    expect(within(card).getByTestId('effective-curfew')).toHaveTextContent('21:30 đến 06:30 hôm sau');
    expect(within(card).getByText(/áp cho game và chat/)).toBeInTheDocument();
    expect(screen.queryByText(/Khối|Lớp/, { selector: '[data-testid="curfew-card"] *' })).toBeNull();
  });

  it('narrower window → error at the field and Save locked; wider window saved with reason + If-Match; clear → null', async () => {
    const { backend } = await renderSchoolLms('/school-lms/settings');
    await openSchoolTab();
    const card = await screen.findByTestId('curfew-card');
    fireEvent.click(within(card).getByRole('button', { name: 'Đặt khung giới nghiêm trường' }));
    const form = await within(card).findByTestId('curfew-form');
    fireEvent.change(within(form).getByLabelText('Bắt đầu'), { target: { value: '22:00' } });
    fireEvent.change(within(form).getByLabelText('Kết thúc'), { target: { value: '06:30' } });
    expect((await within(form).findAllByText(/phải chứa trọn giờ giới nghiêm/)).length).toBeGreaterThan(0);
    expect(within(form).getByRole('button', { name: 'Lưu' })).toBeDisabled();
    fireEvent.change(within(form).getByLabelText('Bắt đầu'), { target: { value: '21:00' } });
    fireEvent.change(within(form).getByLabelText('Kết thúc'), { target: { value: '07:00' } });
    await waitFor(() => expect(within(form).getByRole('button', { name: 'Lưu' })).toBeEnabled());
    fireEvent.click(within(form).getByRole('button', { name: 'Lưu' }));
    expect(await within(form).findByText('Trường này bắt buộc.')).toBeInTheDocument();
    fireEvent.change(within(form).getByLabelText('Lý do'), { target: { value: 'Họp phụ huynh đề nghị' } });
    fireEvent.click(within(form).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PATCH', `/schools/${IDS.schoolA}/settings`)).toHaveLength(1));
    const call = backend.calls('PATCH', `/schools/${IDS.schoolA}/settings`)[0]!;
    expect(call.body).toEqual({ curfew_window: { start: '21:00', end: '07:00' }, reason: 'Họp phụ huynh đề nghị' });
    expect(call.headers['if-match']).toBe('"v1"');
    expect(await screen.findByText('21:00 đến 07:00 hôm sau (giờ Việt Nam)', { selector: '[data-testid="effective-curfew"]' })).toBeInTheDocument();
    const card2 = screen.getByTestId('curfew-card');
    fireEvent.click(within(card2).getByRole('button', { name: 'Đặt khung giới nghiêm trường' }));
    const form2 = await within(card2).findByTestId('curfew-form');
    fireEvent.change(within(form2).getByLabelText('Lý do'), { target: { value: 'Bỏ khung riêng' } });
    fireEvent.click(within(form2).getByRole('button', { name: 'Bỏ khung trường' }));
    await waitFor(() => expect(backend.calls('PATCH', `/schools/${IDS.schoolA}/settings`)).toHaveLength(2));
    expect(backend.calls('PATCH', `/schools/${IDS.schoolA}/settings`)[1]!.body).toEqual({ curfew_window: null, reason: 'Bỏ khung riêng' });
  });

  it('422 VALIDATION_FAILED curfew_window.start → error at the field', async () => {
    const { backend } = await renderSchoolLms('/school-lms/settings');
    backend.overrides.unshift((r) =>
      r.method === 'PATCH' && r.path === `/schools/${IDS.schoolA}/settings`
        ? new Response(JSON.stringify({ status: 422, code: 'VALIDATION_FAILED', trace_id: 't', errors: [{ field: 'curfew_window.start', code: 'CURFEW_NOT_STRICTER' }] }), { status: 422, headers: { 'Content-Type': 'application/problem+json' } })
        : undefined,
    );
    await openSchoolTab();
    const card = await screen.findByTestId('curfew-card');
    fireEvent.click(within(card).getByRole('button', { name: 'Đặt khung giới nghiêm trường' }));
    const form = await within(card).findByTestId('curfew-form');
    fireEvent.change(within(form).getByLabelText('Bắt đầu'), { target: { value: '21:00' } });
    fireEvent.change(within(form).getByLabelText('Kết thúc'), { target: { value: '07:00' } });
    fireEvent.change(within(form).getByLabelText('Lý do'), { target: { value: 'Thử API' } });
    fireEvent.click(within(form).getByRole('button', { name: 'Lưu' }));
    expect(await within(form).findByText('Dữ liệu nhập chưa hợp lệ. Kiểm tra các trường được đánh dấu.')).toBeInTheDocument();
    expect(within(card).getByTestId('school-curfew')).toHaveTextContent('Chưa đặt');
  });

  it('no edit without action.school_lms.settings.edit_school; school policy saved with If-Match (F052-AC-22)', async () => {
    await renderSchoolLms('/school-lms/settings', { account: accountWith('qlcm.truong.a', TENANTS.thDemoA, READ_ONLY_PERMS) });
    await openSchoolTab();
    const card = await screen.findByTestId('curfew-card');
    expect(within(card).queryByRole('button', { name: 'Đặt khung giới nghiêm trường' })).toBeNull();
    stopPortals();
    cleanup();
    const { backend } = await renderSchoolLms('/school-lms/settings');
    await openSchoolTab();
    const policy = await screen.findByTestId('school-policy');
    fireEvent.click(within(policy).getByRole('switch', { name: 'Bảng xếp hạng lớp' }));
    fireEvent.click(within(policy).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PATCH', `/schools/${IDS.schoolA}/settings`)).toHaveLength(1));
    expect(backend.calls('PATCH', `/schools/${IDS.schoolA}/settings`)[0]!.body).toEqual({ class_leaderboard_enabled: false });
    expect(backend.calls('PATCH', `/schools/${IDS.schoolA}/settings`)[0]!.headers['if-match']).toBe('"v1"');
  });
});

describe('Giới hạn chơi mỗi ngày (F052-AC-32, OQ-17)', () => {
  const openSchoolTab = async () => fireEvent.click(await screen.findByRole('tab', { name: 'Cài đặt trường' }));

  it('"Tắt" by default; on with minutes in the contract range + reason + If-Match; off again sends null; 422 at the field', async () => {
    const { backend } = await renderSchoolLms('/school-lms/settings');
    await openSchoolTab();
    const card = await screen.findByTestId('daily-limit-card');
    expect(within(card).getByTestId('daily-limit-value')).toHaveTextContent('Tắt');
    fireEvent.click(within(card).getByRole('button', { name: 'Đổi giới hạn chơi' }));
    const form = await within(card).findByTestId('daily-limit-form');
    fireEvent.click(within(form).getByRole('switch', { name: 'Bật giới hạn' }));
    const minutes = await within(form).findByRole('spinbutton', { name: 'Số phút mỗi ngày' });
    expect(minutes).toHaveAttribute('aria-valuemin', '15');
    expect(minutes).toHaveAttribute('aria-valuemax', '300');
    fireEvent.change(minutes, { target: { value: '120' } });
    fireEvent.change(within(form).getByLabelText('Lý do'), { target: { value: 'Họp phụ huynh đề nghị' } });
    fireEvent.click(within(form).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PATCH', `/schools/${IDS.schoolA}/settings`)).toHaveLength(1));
    const call = backend.calls('PATCH', `/schools/${IDS.schoolA}/settings`)[0]!;
    expect(call.body).toEqual({ daily_play_limit_minutes: 120, reason: 'Họp phụ huynh đề nghị' });
    expect(call.headers['if-match']).toBe('"v1"');
    expect(await screen.findByText('120 phút mỗi ngày', { selector: '[data-testid="daily-limit-value"]' })).toBeInTheDocument();
    const card2 = screen.getByTestId('daily-limit-card');
    fireEvent.click(within(card2).getByRole('button', { name: 'Đổi giới hạn chơi' }));
    const form2 = await within(card2).findByTestId('daily-limit-form');
    fireEvent.click(within(form2).getByRole('switch', { name: 'Bật giới hạn' }));
    fireEvent.change(within(form2).getByLabelText('Lý do'), { target: { value: 'Tắt giới hạn' } });
    fireEvent.click(within(form2).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PATCH', `/schools/${IDS.schoolA}/settings`)).toHaveLength(2));
    expect(backend.calls('PATCH', `/schools/${IDS.schoolA}/settings`)[1]!.body).toEqual({ daily_play_limit_minutes: null, reason: 'Tắt giới hạn' });
    expect(await screen.findByText('Tắt', { selector: '[data-testid="daily-limit-value"]' })).toBeInTheDocument();
    // 422 VALIDATION_FAILED daily_play_limit_minutes → error at the field.
    backend.overrides.unshift((r) =>
      r.method === 'PATCH' && r.path === `/schools/${IDS.schoolA}/settings`
        ? new Response(JSON.stringify({ status: 422, code: 'VALIDATION_FAILED', trace_id: 't', errors: [{ field: 'daily_play_limit_minutes', code: 'INVALID' }] }), { status: 422, headers: { 'Content-Type': 'application/problem+json' } })
        : undefined,
    );
    const card3 = screen.getByTestId('daily-limit-card');
    fireEvent.click(within(card3).getByRole('button', { name: 'Đổi giới hạn chơi' }));
    const form3 = await within(card3).findByTestId('daily-limit-form');
    fireEvent.click(within(form3).getByRole('switch', { name: 'Bật giới hạn' }));
    fireEvent.change(await within(form3).findByRole('spinbutton', { name: 'Số phút mỗi ngày' }), { target: { value: '200' } });
    fireEvent.change(within(form3).getByLabelText('Lý do'), { target: { value: 'Thử API' } });
    fireEvent.click(within(form3).getByRole('button', { name: 'Lưu' }));
    expect(await within(form3).findByText('Dữ liệu nhập chưa hợp lệ. Kiểm tra các trường được đánh dấu.')).toBeInTheDocument();
  });

  it('no edit without action.school_lms.settings.edit_school', async () => {
    await renderSchoolLms('/school-lms/settings', { account: accountWith('qlcm.truong.a', TENANTS.thDemoA, READ_ONLY_PERMS) });
    await openSchoolTab();
    const card = await screen.findByTestId('daily-limit-card');
    expect(within(card).getByTestId('daily-limit-value')).toHaveTextContent('Tắt');
    expect(within(card).queryByRole('button', { name: 'Đổi giới hạn chơi' })).toBeNull();
  });
});

describe('Học sinh sắp bị dọn (F052-AC-21)', () => {
  it('select → exclude with mandatory reason and an Idempotency-Key', async () => {
    const { backend } = await renderSchoolLms('/school-lms/settings', { account: tenantAdmin() });
    fireEvent.click(await screen.findByRole('tab', { name: 'Học sinh sắp bị dọn' }));
    const page = await screen.findByTestId('cleanup-preview');
    const row = (await within(page).findByText('Trần Sắp Bị Dọn')).closest('tr')!;
    fireEvent.click(within(row).getByRole('checkbox'));
    fireEvent.click(within(page).getByRole('button', { name: 'Loại trừ / gia hạn (1)' }));
    const form = await screen.findByTestId('exclude-form');
    const dialog = form.closest('.ant-modal')! as HTMLElement;
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    expect(await within(form).findByText('Trường này bắt buộc.')).toBeInTheDocument();
    fireEvent.change(within(form).getByLabelText('Lý do'), { target: { value: 'Học sinh nghỉ ốm dài ngày' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('POST', '/tenant/student-cleanup/exclusions')).toHaveLength(1));
    const call = backend.calls('POST', '/tenant/student-cleanup/exclusions')[0]!;
    expect(call.body).toEqual({ student_ids: [IDS.studentSapBiDon], reason: 'Học sinh nghỉ ốm dài ngày', until: null });
    expect(call.headers['idempotency-key']).toBeTruthy();
    expect(screen.queryByRole('button', { name: /vĩnh viễn/ })).toBeNull();
  });
});
