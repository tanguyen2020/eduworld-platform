// F056 T029, T030, T034, T035, T038, T039 (US3, US4, US5) — F056-AC-12 (course results: graded columns, score_10,
// status, cursor pages ≤ 100, no XP/rating/competition/manual input), F056-AC-13 (exam analysis: distribution, median,
// pass rate, p-value without rapid_guess, misconceptions, reliability warning, "Giao ôn tập" button), F056-AC-14/15
// (createLmsReport with Idempotency-Key → "Đang chuẩn bị" → link with expiry; EXPORT_LIMIT_REACHED message),
// F056-AC-16 (reference label + "Chưa đủ dữ liệu"), F056-AC-17 (tenant summary "< 5" + createTenantReport),
// F056-AC-18 (System Admin: tenant_ids[] from the filter, no X-Tenant-Id, Tenant column, createPlatformReport).
// Backend = MOCK.
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { tenantFilterStore } from '../../../../shell/tenantFilterStore';
import { adTruongA, DASH_IDS, IDS, LMS_IDS, lmsAssignment, renderDashboard, stopPortals, TENANTS } from '../../../../test/dashboard-harness';
import type { LmsMockState } from '../../../../test/lms-mock';

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
  cleanup();
});

const graded = (lms: LmsMockState) => {
  lms.assignments.set(DASH_IDS.assignmentGraded, lmsAssignment({ assignment_id: DASH_IDS.assignmentGraded, title: 'Kiểm tra Phép nhân', mode: 'graded', exam_session_id: LMS_IDS.examSession }));
  lms.assignments.set(DASH_IDS.assignmentPractice, lmsAssignment({ assignment_id: DASH_IDS.assignmentPractice, title: 'Luyện tập thêm', mode: 'practice' }));
};

describe('Bảng kết quả và phân tích (US3)', () => {
  it('F056-AC-12: graded columns with score_10 and status, "làm quá nhanh" mark, cursor pages, no XP/rating/manual input', async () => {
    const { backend } = await renderDashboard(`/school-lms/results/${LMS_IDS.courseMath}`, { setup: (_d, l) => graded(l) });
    const table = await screen.findByTestId('course-results');
    expect(await within(table).findByRole('link', { name: 'Mở phân tích bài Kiểm tra Phép nhân' })).toBeInTheDocument();
    expect(within(table).getByTestId('score')).toHaveTextContent('8,5');
    expect(within(table).getByText('Làm quá nhanh')).toBeInTheDocument();
    expect(new URLSearchParams(backend.calls('GET', `/lms/courses/${LMS_IDS.courseMath}/results`)[0]!.search).get('limit')).toBe('100');
    fireEvent.click(within(table).getByRole('button', { name: 'Xem thêm' }));
    expect(await within(table).findByText('Không nộp')).toBeInTheDocument();
    expect(backend.calls('GET', `/lms/courses/${LMS_IDS.courseMath}/results`).some((r) => new URLSearchParams(r.search).get('cursor') === 'r2')).toBe(true);
    expect(table.textContent).not.toMatch(/XP|rating|thi đấu/i);
    expect(within(table).queryAllByRole('textbox')).toHaveLength(0);
    expect(within(table).queryAllByRole('spinbutton')).toHaveLength(0);
  });

  it('F056-AC-13: analysis of the exam session (distribution, median, pass rate, p-value w/o rapid guess, misconceptions, reliability) + practice button', async () => {
    const { backend, router } = await renderDashboard(`/school-lms/results/${LMS_IDS.courseMath}/analysis/${DASH_IDS.assignmentGraded}`, { setup: (_d, l) => graded(l) });
    const w = await screen.findByTestId('widget-analysis');
    await waitFor(() => expect(within(w).getByTestId('metric-median')).toHaveTextContent('6,5'));
    expect(backend.calls('GET', `/exams/sessions/${LMS_IDS.examSession}/report`)).toHaveLength(1);
    expect(within(w).getByTestId('metric-pass-rate')).toHaveTextContent('75%');
    expect(within(w).getByTestId('reliability-warning')).toBeInTheDocument();
    expect(within(w).getByTestId('score-distribution').getAttribute('aria-label')).toMatch(/^Phổ điểm .*; 3–3,5: 8;/);
    expect(within(w).getByText('Tỉ lệ đúng không tính các lượt làm quá nhanh.')).toBeInTheDocument();
    const row = within(w).getByText('Câu 1').closest('tr')!;
    expect(row).toHaveTextContent('0,42');
    expect(row).toHaveTextContent('10%');
    expect(row).toHaveTextContent('nhan_nham_bang_7 · 9');
    expect(row).toHaveTextContent('Câu cần xem lại');
    fireEvent.click(within(row).getByRole('button', { name: 'Giao ôn tập cho học sinh sai câu này' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/school-lms/assignments/new'));
    expect(new URLSearchParams(router.state.location.search).get('mode')).toBe('practice');
    expect(new URLSearchParams(router.state.location.search).get('class_id')).toBe(IDS.class3A);
  });

  it('F056-AC-13: an assignment without exam session explains that item analysis is not available', async () => {
    await renderDashboard(`/school-lms/results/${LMS_IDS.courseMath}/analysis/${DASH_IDS.assignmentPractice}`, { setup: (_d, l) => graded(l) });
    expect(await screen.findByTestId('no-item-analysis')).toBeInTheDocument();
  });
});

describe('Xuất báo cáo (US4)', () => {
  it('F056-AC-14: createLmsReport with Idempotency-Key → "Đang chuẩn bị báo cáo" → link with expiry; nothing downloaded from the create request', async () => {
    const { backend } = await renderDashboard(`/school-lms/results/${LMS_IDS.courseMath}`, { setup: (_d, l) => graded(l) });
    const table = await screen.findByTestId('course-results');
    fireEvent.click(await within(table).findByRole('button', { name: 'Xuất báo cáo' }));
    await waitFor(() => expect(backend.calls('POST', '/lms/reports')).toHaveLength(1));
    const post = backend.calls('POST', '/lms/reports')[0]!;
    expect(post.headers['idempotency-key']).toBeTruthy();
    expect(post.body).toEqual({ report_type: 'course_results', format: 'xlsx', course_id: LMS_IDS.courseMath });
    expect(await within(table).findByText('Đang chuẩn bị báo cáo…')).toBeInTheDocument();
    const link = await within(table).findByTestId('export-link', {}, { timeout: 20_000 });
    expect(link.getAttribute('href')).toMatch(/^https:\/\/exports\.test\/.+\.xlsx\?sig=mock$/);
    expect(within(table).getByText(/^Link hết hạn lúc \d{2}:\d{2}$/)).toBeInTheDocument();
    expect(backend.requests.filter((r) => r.path.startsWith('/lms/reports/')).length).toBeGreaterThanOrEqual(2);
  });

  it('F056-AC-15: 429 EXPORT_LIMIT_REACHED → explained, export can be retried; expired link → "Tạo lại báo cáo"', async () => {
    const { dash } = await renderDashboard(`/school-lms/results/${LMS_IDS.courseMath}`, {
      setup: (d, l) => {
        graded(l);
        d.exportLimit = true;
        d.linkTtlSeconds = 1;
      },
    });
    const table = await screen.findByTestId('course-results');
    fireEvent.click(await within(table).findByRole('button', { name: 'Xuất báo cáo' }));
    expect(await within(table).findByText('Bạn đang có quá nhiều báo cáo đang chuẩn bị. Hãy chờ một báo cáo xong rồi thử lại.')).toBeInTheDocument();
    dash.exportLimit = false;
    fireEvent.click(within(table).getByRole('button', { name: 'Xuất báo cáo' }));
    expect(await within(table).findByTestId('export-link', {}, { timeout: 20_000 })).toBeInTheDocument();
    expect(await within(table).findByRole('button', { name: 'Tạo lại báo cáo' }, { timeout: 10_000 })).toBeEnabled();
    expect(within(table).queryByTestId('export-link')).toBeNull();
  });

  it('F056-AC-16: reference report and reference levels carry "Tham khảo — không thay đánh giá chính thức"; insufficient data → "Chưa đủ dữ liệu"', async () => {
    await renderDashboard('/school-lms/reports');
    fireEvent.mouseDown(await screen.findByRole('combobox', { name: 'Loại báo cáo' }));
    fireEvent.click(await screen.findByTitle('Báo cáo tham khảo TT27/TT22'));
    expect(await screen.findByTestId('regulation-disclaimer')).toHaveTextContent('Tham khảo — không thay đánh giá chính thức');
    expect(screen.getByRole('button', { name: 'Xuất báo cáo' })).toBeDisabled();
    stopPortals();
    cleanup();
    await renderDashboard(`/school-lms/school-dashboard/reference?school=${IDS.schoolA}&class=${IDS.class3A}&subject=${IDS.subjectMath}&preset=7d&from=2026-11-15&to=2026-11-21`);
    const page = await screen.findByTestId('dashboard-reference');
    expect(within(page).getByTestId('regulation-disclaimer')).toHaveTextContent('Tham khảo — không thay đánh giá chính thức');
    expect(await within(page).findByText('Chưa đủ dữ liệu')).toBeInTheDocument();
    expect(within(page).getByText('Hoàn thành')).toBeInTheDocument();
    expect(await within(page).findByText('Mức tham khảo suy ra từ kết quả tự chấm theo hồ sơ đánh giá.')).toBeInTheDocument();
  });
});

describe('Tổng hợp đơn vị và platform (US5)', () => {
  const TENANT_ADMIN = ['analytics.tenant.read', 'report.export', 'lms.report.read', 'tenant.school.read', 'tenant.class.read'];

  it('F056-AC-17: getTenantLearningSummary by school × grade × subject, "< 5" for suppressed cells, export via createTenantReport', async () => {
    const { backend } = await renderDashboard('/school-lms/reports', { account: adTruongA(TENANT_ADMIN) });
    fireEvent.click(await screen.findByRole('tab', { name: 'Tổng hợp đơn vị' }));
    const w = await screen.findByTestId('widget-tenant-summary');
    expect(await within(w).findByText('Khối 4')).toBeInTheDocument();
    const suppressed = within(w).getByText('Khối 4').closest('tr')!;
    expect(within(suppressed).getAllByTestId('suppressed-cell')).toHaveLength(5);
    expect(suppressed.textContent).not.toMatch(/\d{2,}/);
    expect(within(w).getByText('Khối 3').closest('tr')).toHaveTextContent('71%');
    const call = backend.calls('GET', '/tenant/analytics/learning/summary')[0]!;
    expect(call.search).not.toContain('tenant_ids');
    fireEvent.click(within(screen.getByTestId('tenant-summary')).getByRole('button', { name: 'Xuất báo cáo' }));
    await waitFor(() => expect(backend.calls('POST', '/tenant/reports')).toHaveLength(1));
    const post = backend.calls('POST', '/tenant/reports')[0]!;
    expect(post.headers['idempotency-key']).toBeTruthy();
    expect(post.body).toMatchObject({ report_type: 'learning_summary', format: 'xlsx' });
  });

  it('F056-AC-18: System Admin — tenant_ids[] from the tenant filter, no X-Tenant-Id, Tenant column, createPlatformReport', async () => {
    tenantFilterStore.set([TENANTS.thDemoA.tenant_id, TENANTS.thcsDemoB.tenant_id]);
    const { backend } = await renderDashboard('/platform-admin/reports', { as: 'sa.admin' });
    const w = await screen.findByTestId('widget-platform-summary', {}, { timeout: 30_000 });
    await waitFor(() => expect(backend.calls('GET', '/admin/analytics/learning/summary').length).toBeGreaterThan(0));
    const call = backend.calls('GET', '/admin/analytics/learning/summary').at(-1)!;
    expect(new URLSearchParams(call.search).getAll('tenant_ids').sort()).toEqual([TENANTS.thDemoA.tenant_id, TENANTS.thcsDemoB.tenant_id].sort());
    expect(call.headers['x-tenant-id']).toBeUndefined();
    expect(call.gateway).toBe('internal');
    await within(w).findAllByText('Khối 3');
    expect(within(w).getAllByRole('columnheader', { name: 'Tenant' }).length).toBe(2);
    expect(within(w).getAllByTestId('suppressed-cell').length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole('button', { name: 'Xuất báo cáo' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/reports')).toHaveLength(1));
    const post = backend.calls('POST', '/admin/reports')[0]!;
    expect(post.headers['idempotency-key']).toBeTruthy();
    expect((post.body as { tenant_ids: string[] }).tenant_ids.sort()).toEqual([TENANTS.thDemoA.tenant_id, TENANTS.thcsDemoB.tenant_id].sort());
  });
});

