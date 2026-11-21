// F035 T066/T071/T072 — F035-AC-35, F035-AC-37 (TC-UI-006, TC-CMS-MENU-02): research screens of workspace D for
// sa.admin (agreements with tenant_ids[] and X-Tenant-Id, studies, cohorts (school, grade), activation, waves, report with hidden
// cells, pseudonymised dataset, experiments start/stop/results), menus from the catalog only (hidden for a school
// admin, hidden when turned off, buttons hidden when their action is off). Backend = MOCK (src/test/research-backend).
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { stopPortals, TENANTS } from '../../../../test/portal-harness';
import { renderResearch, RIDS } from '../../../../test/research-backend';
import { tenantFilterStore } from '../../../../shell/tenantFilterStore';

vi.setConfig({ testTimeout: 600_000 });
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

describe('Nghiên cứu hiệu quả (F035-AC-35)', () => {
  it('lists and creates studies; the detail activates, adds a cohort with X-Tenant-Id and schedules a wave', async () => {
    const { backend, user } = await renderResearch('/platform-admin/efficacy-studies');
    await screen.findByRole('heading', { level: 1, name: 'Nghiên cứu hiệu quả học tập' });
    expect(await screen.findByText('math-g3-2026')).toBeInTheDocument();
    // create
    await user.click(screen.getByRole('button', { name: 'Tạo nghiên cứu' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Mã nghiên cứu'), { target: { value: 'math-g3-b' } });
    fireEvent.change(within(dialog).getByLabelText('Tên nghiên cứu'), { target: { value: 'Toán 3 – nhóm B' } });
    fireEvent.change(within(dialog).getByLabelText('Môn học (subject_id)'), { target: { value: RIDS.subject } });
    fireEvent.change(within(dialog).getByLabelText('Khối'), { target: { value: '3' } });
    await user.type(within(dialog).getByRole('combobox', { name: 'Mục tiêu học tập (objective_id)' }), `${RIDS.lo}{enter}`);
    fireEvent.change(within(dialog).getByLabelText('Dạng đề A (blueprint version)'), { target: { value: RIDS.formA } });
    fireEvent.change(within(dialog).getByLabelText('Dạng đề B (blueprint version)'), { target: { value: RIDS.formB } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/learning/efficacy-studies')).toHaveLength(1));
    const post = backend.calls('POST', '/admin/learning/efficacy-studies')[0]!;
    expect(post.body).toEqual({ code: 'math-g3-b', title: 'Toán 3 – nhóm B', subject_id: RIDS.subject, grade_level: 3, objective_ids: [RIDS.lo], form_a_blueprint_version_id: RIDS.formA, form_b_blueprint_version_id: RIDS.formB, min_intervention_weeks: 8 });
    expect(post.headers['idempotency-key']).toBeTruthy();
    expect(post.headers['x-tenant-id']).toBeUndefined();
    // detail: add a cohort (school of TH Demo A × grade of the study, X-Tenant-Id), activate
    await user.click(screen.getByRole('link', { name: 'math-g3-2026' }));
    await screen.findByRole('heading', { level: 1, name: 'math-g3-2026 · Toán 3 – học kỳ I' });
    await user.click(screen.getByRole('button', { name: 'Thêm cụm' }));
    const add = await screen.findByRole('dialog');
    await user.click(within(add).getByRole('combobox', { name: 'Tenant đích' }));
    await user.click((await screen.findAllByTitle('TH Demo A (th-demo-a)')).at(-1)!);
    fireEvent.change(within(add).getByLabelText('Mã trường (school_id)'), { target: { value: RIDS.school } });
    fireEvent.change(within(add).getByLabelText('Cặp số'), { target: { value: '1' } });
    await user.click(within(add).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/learning/efficacy-studies/${RIDS.study}/cohorts`)).toHaveLength(1));
    const cls = backend.calls('POST', `/admin/learning/efficacy-studies/${RIDS.study}/cohorts`)[0]!;
    expect(cls.headers['x-tenant-id']).toBe(TENANTS.thDemoA.tenant_id);
    expect(cls.body).toEqual({ school_id: RIDS.school, pair_no: 1 });
    expect(await screen.findByText('Khối 3')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Kích hoạt' }));
    await user.click((await screen.findAllByRole('button', { name: 'Kích hoạt' })).at(-1)!);
    await waitFor(() => expect(backend.calls('POST', `/admin/learning/efficacy-studies/${RIDS.study}:activate`)).toHaveLength(1));
    expect(await screen.findByText('Nghiên cứu đã kích hoạt: cặp cụm, nhánh và dạng đề đã khóa.')).toBeInTheDocument();
    // waves tab: no score is ever shown
    await user.click(screen.getByRole('tab', { name: 'Đợt kiểm tra' }));
    expect(await screen.findByText(/Đợt kiểm tra không công bố điểm/)).toBeInTheDocument();
  });

  it('report: aggregates only, hidden cells, descriptive_only; dataset export link', async () => {
    const { backend, user } = await renderResearch(`/platform-admin/efficacy-studies/${RIDS.study}`);
    await screen.findByRole('heading', { level: 1, name: 'math-g3-2026 · Toán 3 – học kỳ I' });
    await user.click(screen.getByRole('tab', { name: 'Báo cáo' }));
    await user.click(await screen.findByRole('button', { name: 'Tạo báo cáo' }));
    expect(await screen.findByText('Số cụm mỗi nhóm dưới khuyến nghị cỡ mẫu: chỉ dùng như thống kê mô tả.')).toBeInTheDocument();
    expect(screen.getByText('0.512')).toBeInTheDocument();
    expect(screen.getAllByText('ẩn').length).toBeGreaterThan(0);
    expect(backend.calls('POST', `/admin/learning/efficacy-studies/${RIDS.study}/reports`)[0]!.body).toEqual({});
    await user.click(screen.getByRole('button', { name: 'Xuất bộ dữ liệu' }));
    expect(await screen.findByRole('link', { name: /Tải CSV \(49 dòng/ })).toHaveAttribute('href', 'https://files.example.test/efficacy.csv');
    const exp = backend.calls('POST', '/admin/reports')[0]!;
    expect(exp.body).toMatchObject({ report_type: 'efficacy_dataset', format: 'csv', params: { study_id: RIDS.study } });
    const page = document.body.textContent ?? '';
    expect(page).not.toMatch(/hs\.an|student_id/);
  });

  it('agreements: tenant_ids[] required to list, create and revoke with the X-Tenant-Id of the tenant', async () => {
    const { backend, user } = await renderResearch('/platform-admin/efficacy-studies');
    await screen.findByRole('heading', { level: 1, name: 'Nghiên cứu hiệu quả học tập' });
    await user.click(screen.getByRole('tab', { name: 'Thỏa thuận tham gia' }));
    expect(await screen.findByText(/Chọn ít nhất một tenant/)).toBeInTheDocument();
    expect(backend.calls('GET', '/admin/learning/research-agreements')).toHaveLength(0);
    await user.click(screen.getByRole('combobox', { name: 'Chọn tenant để xem thỏa thuận' }));
    await user.click((await screen.findAllByTitle('TH Demo A (th-demo-a)')).at(-1)!);
    expect(await screen.findByText('HD-2026-01')).toBeInTheDocument();
    expect(new URLSearchParams(backend.calls('GET', '/admin/learning/research-agreements').at(-1)!.search).getAll('tenant_ids')).toEqual([TENANTS.thDemoA.tenant_id]);
    await user.click(within(screen.getByText('HD-2026-01').closest('tr')!).getByRole('button', { name: 'Thu hồi' }));
    const dialog = await screen.findByRole('dialog');
    const reason = within(dialog).getByLabelText('Lý do');
    expect(reason).toHaveAttribute('maxlength', '1000');
    fireEvent.change(reason, { target: { value: 'Trường rút khỏi pilot theo văn bản' } });
    await user.click(within(dialog).getByRole('button', { name: 'Thu hồi' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/learning/research-agreements/${RIDS.agreement}:revoke`)).toHaveLength(1));
    const r = backend.calls('POST', `/admin/learning/research-agreements/${RIDS.agreement}:revoke`)[0]!;
    expect(r.headers['x-tenant-id']).toBe(TENANTS.thDemoA.tenant_id);
    expect(r.body).toEqual({ reason: 'Trường rút khỏi pilot theo văn bản' });
  });
});

describe('Thử nghiệm A/B (F035-AC-35)', () => {
  it('detail: start shadow, results with a hidden cell, stop with a reason', async () => {
    const { backend, user } = await renderResearch('/platform-admin/learning-experiments');
    await screen.findByRole('heading', { level: 1, name: 'Thử nghiệm A/B' });
    await user.click(await screen.findByRole('link', { name: 'mastery-we-1-1' }));
    await screen.findByRole('heading', { level: 1, name: 'mastery-we-1-1' });
    const results = await screen.findByRole('region', { name: 'Kết quả theo nhánh' });
    expect(await within(results).findByText('0.71')).toBeInTheDocument();
    expect(within(results).getByText('ẩn')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Chạy shadow' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/learning/experiments/${RIDS.experiment}:start`)).toHaveLength(1));
    expect(backend.calls('POST', `/admin/learning/experiments/${RIDS.experiment}:start`)[0]!.body).toEqual({ exposure: 'shadow' });
    await user.click(await screen.findByRole('button', { name: 'Dừng thử nghiệm' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Dừng thử nghiệm theo kế hoạch' } });
    await user.click(within(dialog).getByRole('button', { name: 'Dừng thử nghiệm' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/learning/experiments/${RIDS.experiment}:stop`)).toHaveLength(1));
    expect(backend.calls('POST', `/admin/learning/experiments/${RIDS.experiment}:stop`)[0]!.body).toEqual({ reason: 'Dừng thử nghiệm theo kế hoạch' });
  });

  it('create: arms with the control first, recommendation rule_params only on the test arm', async () => {
    const { backend, user } = await renderResearch('/platform-admin/learning-experiments');
    await screen.findByRole('heading', { level: 1, name: 'Thử nghiệm A/B' });
    await user.click(screen.getByRole('button', { name: 'Tạo thử nghiệm' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Mã thử nghiệm'), { target: { value: 'rec-fewer' } });
    await user.click(within(dialog).getByLabelText('Luật gợi ý'));
    await user.click(within(dialog).getByRole('combobox', { name: 'Tenant tham gia' }));
    await user.click((await screen.findAllByTitle('TH Demo A (th-demo-a)')).at(-1)!);
    const max = within(dialog).getAllByLabelText('Số gợi ý tối đa (1–3)');
    fireEvent.change(max[0]!, { target: { value: '2' } }); // control: ignored (backend refuses parameters on the control)
    fireEvent.change(max[1]!, { target: { value: '1' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/learning/experiments')).toHaveLength(1));
    const body = backend.calls('POST', '/admin/learning/experiments')[0]!.body as Record<string, unknown>;
    expect(body).toMatchObject({ code: 'rec-fewer', kind: 'recommendation_rule', unit: 'school_grade', tenant_ids: [TENANTS.thDemoA.tenant_id] });
    // default period: from the current hour for 30 days
    expect(new Date(String(body.end_at)).getTime() - new Date(String(body.start_at)).getTime()).toBe(30 * 86_400_000);
    expect(body.arms).toEqual([
      { code: 'control', allocation_pct: 50, is_control: true },
      { code: 'test', allocation_pct: 50, is_control: false, rule_params: { max_recommendations: 1 } },
    ]);
  });
});

describe('Danh mục menu/action (F035-AC-35, AC-37)', () => {
  it('a school admin has no research menu and gets the 403 page', async () => {
    const { backend } = await renderResearch('/platform-admin/efficacy-studies', { account: 'ad.truong.a' });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Nghiên cứu hiệu quả học tập' })).toBeNull();
    expect(screen.queryByRole('link', { name: 'Thử nghiệm A/B' })).toBeNull();
    expect(backend.requests.some((r) => r.path.startsWith('/admin/learning/'))).toBe(false);
  });

  it('turned-off menus are hidden; a turned-off edit action hides its buttons', async () => {
    await renderResearch('/platform-admin/trash', { disabledMenus: ['menu.platform_admin.efficacy_studies', 'menu.platform_admin.learning_experiments'] });
    await screen.findByRole('heading', { level: 1, name: 'Thùng rác' });
    expect(screen.queryByRole('link', { name: 'Nghiên cứu hiệu quả học tập' })).toBeNull();
    expect(screen.queryByRole('link', { name: 'Thử nghiệm A/B' })).toBeNull();
    stopPortals();
    const { backend } = await renderResearch('/platform-admin/efficacy-studies', { disabledActions: ['action.platform_admin.efficacy_studies.edit'] });
    await screen.findByText('math-g3-2026');
    expect(screen.queryByRole('button', { name: 'Tạo nghiên cứu' })).toBeNull();
    expect(backend.calls('POST', '/admin/learning/efficacy-studies')).toHaveLength(0);
  });
});
