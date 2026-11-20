// F056 T041–T045, T050, T051 (US6) — F056-AC-19 (menus by analytics.* permissions; subject filter of a subject-scoped
// academic manager; 403 → "Ngoài phạm vi quyền" without data), F056-AC-20 (no analytics permission → portal 403 page and
// no dashboard request), F056-AC-21 (X-Tenant-Id of the session; another tenant's class → "Không tìm thấy"; query keys
// carry the tenant), F056-AC-23 (catalog menu/action turned off → hidden; 403 FEATURE_DISABLED on export → message +
// getMyPermissions reloaded; the dashboard stays readable). Backend = MOCK.
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { accountWith, adTruongA, IDS, LMS_IDS, problem, renderDashboard, SCHOOL_ADMIN_DASH_PERMS, stopPortals, TENANTS } from '../../../../test/dashboard-harness';

afterEach(() => {
  stopPortals();
  cleanup();
});

const BASE = '/school-lms/school-dashboard';
const dashboardCalls = (backend: { requests: { path: string }[] }) => backend.requests.filter((r) => /^\/(lms\/(dashboards|classes\/[^/]+\/dashboard|students|reports|courses\/[^/]+\/results)|tenant\/analytics|exams\/sessions)/.test(r.path));

describe('Phân quyền và cô lập tenant (US6)', () => {
  it('F056-AC-19: menus follow analytics.* / report permissions; the subject filter of a subject-scoped manager lists only its subjects', async () => {
    await renderDashboard(BASE, { roles: [{ role: 'academic_manager', scope_type: 'subject', scope_id: IDS.subjectMath }] });
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    expect(await within(nav).findByText('School Learning Dashboard')).toBeInTheDocument();
    fireEvent.mouseDown(await screen.findByRole('combobox', { name: 'Môn' }));
    expect(await screen.findByTitle('Toán')).toBeInTheDocument();
    expect(screen.queryByTitle('Tiếng Việt')).toBeNull();
    stopPortals();
    cleanup();
    // a school user without analytics.* does not see the dashboard menu
    await renderDashboard('/school-lms/classes', { account: adTruongA(['tenant.school.read', 'tenant.class.read', 'lms.class.read']) });
    const nav2 = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    await within(nav2).findByText('Lớp & học sinh');
    expect(within(nav2).queryByText('School Learning Dashboard')).toBeNull();
    expect(within(nav2).queryByText('Báo cáo & analytics')).toBeNull();
  });

  it('F056-AC-19: a subject outside the scope (403) → "Ngoài phạm vi quyền" and no number', async () => {
    await renderDashboard(`${BASE}?school=${IDS.schoolA}&subject=${LMS_IDS.subjectViet}&preset=7d&from=2026-11-15&to=2026-11-21`, { setup: (d) => d.forbiddenSubjects.add(LMS_IDS.subjectViet) });
    expect(await screen.findAllByText('Ngoài phạm vi quyền')).toHaveLength(5);
    expect(screen.queryByTestId('metric-assignments-open')).toBeNull();
    stopPortals();
    cleanup();
    await renderDashboard(`${BASE}/classes/${IDS.class3A}?school=${IDS.schoolA}&subject=${LMS_IDS.subjectViet}&preset=7d&from=2026-11-15&to=2026-11-21`, { setup: (d) => d.forbiddenSubjects.add(LMS_IDS.subjectViet) });
    expect(await screen.findByTestId('dashboard-out-of-scope')).toHaveTextContent('Ngoài phạm vi quyền');
    expect(screen.queryByTestId('mastery-heatmap')).toBeNull();
  });

  it('F056-AC-20: Safety Moderator / Content Admin opening the dashboard URL → portal 403 page, no dashboard request', async () => {
    const sm = accountWith('sm.antoan', TENANTS.thDemoA, ['moderation.school_notification.read']);
    const r1 = await renderDashboard(`${BASE}?school=${IDS.schoolA}`, { account: sm });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(dashboardCalls(r1.backend)).toHaveLength(0);
    stopPortals();
    cleanup();
    const r2 = await renderDashboard(`${BASE}?school=${IDS.schoolA}`, { as: 'cm.noidung' });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(dashboardCalls(r2.backend)).toHaveLength(0);
  });

  it('F056-AC-21: X-Tenant-Id of the session; a class of another tenant → "Không tìm thấy"; query keys carry the tenant', async () => {
    const adB = accountWith('ad.truong.b', TENANTS.thcsDemoB, SCHOOL_ADMIN_DASH_PERMS);
    const { backend, runtime } = await renderDashboard(`${BASE}/classes/${IDS.class3A}?school=${IDS.schoolA}`, { account: adB });
    expect(await screen.findByTestId('school-lms-not-found')).toBeInTheDocument();
    const call = backend.calls('GET', `/lms/classes/${IDS.class3A}/dashboard`)[0]!;
    expect(call.headers['x-tenant-id']).toBe(TENANTS.thcsDemoB.tenant_id);
    expect(screen.queryByTestId('mastery-heatmap')).toBeNull();
    expect(document.body.textContent).not.toMatch(/TOAN\.3|Nguyễn An/);
    const keys = runtime.queryClient.getQueryCache().getAll().map((q) => q.queryKey).filter((k) => k[0] === 'school-lms' && k[1] === 'dashboard');
    expect(keys.length).toBeGreaterThan(0);
    for (const k of keys) expect(k[3]).toBe(TENANTS.thcsDemoB.tenant_id);
  });

  it('F056-AC-23: export action turned off → no export button, dashboard still readable; menu turned off → hidden + "tính năng đang tắt"', async () => {
    const { backend } = await renderDashboard(`/school-lms/results/${LMS_IDS.courseMath}`, { disabledActions: ['action.school_lms.reports.export'] });
    await screen.findByTestId('course-results');
    await waitFor(() => expect(backend.calls('GET', `/lms/courses/${LMS_IDS.courseMath}/results`).length).toBeGreaterThan(0));
    expect(screen.queryByRole('button', { name: 'Xuất báo cáo' })).toBeNull();
    stopPortals();
    cleanup();
    await renderDashboard(BASE, { disabledMenus: ['menu.school_lms.school_dashboard'] });
    expect(await screen.findByTestId('feature-disabled')).toBeInTheDocument();
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    expect(within(nav).queryByText('School Learning Dashboard')).toBeNull();
  });

  it('F056-AC-23: 403 FEATURE_DISABLED on createLmsReport → message and getMyPermissions reloaded', async () => {
    const { backend } = await renderDashboard(`/school-lms/results/${LMS_IDS.courseMath}`);
    backend.overrides.unshift((r) => (r.method === 'POST' && r.path === '/lms/reports' ? problem(403, 'FEATURE_DISABLED', { feature_key: 'action.school_lms.reports.export' }) : undefined));
    const table = await screen.findByTestId('course-results');
    const before = backend.calls('GET', '/me/permissions').length;
    fireEvent.click(await within(table).findByRole('button', { name: 'Xuất báo cáo' }));
    expect(await within(table).findByText('Tính năng này đang tạm tắt.')).toBeInTheDocument();
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(before));
  });
});
