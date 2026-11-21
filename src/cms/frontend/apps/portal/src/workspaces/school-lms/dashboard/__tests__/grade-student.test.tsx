// F056 T022, T024, T025, T044 (US2, US6) rewritten for ADR-0095 (no class level) — F056-AC-08 (school → grade → student:
// grades of the school on the overview, the grade page (getGradeDashboard) keeps range/subject, breadcrumb school → grade
// → student, "< 5" for a grade under the small-cell threshold, no class page, list or link), F056-AC-09 (MasteryHeatmap
// with symbols on the grade page), F056-AC-10 ("Giao ôn tập theo LO" opens the F055 wizard with mode=practice, the LO,
// the school, subject, grade and the weak students), F056-AC-11 (profile tabs; no Game Rating / position / friends /
// chat / IP / device id), F056-AC-21 (out of scope → "Không tìm thấy"). Backend = MOCK.
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { DASH_IDS, IDS, LMS_IDS, adTruongA, renderDashboard, SCHOOL_ADMIN_DASH_PERMS, stopPortals } from '../../../../test/dashboard-harness';

afterEach(() => {
  stopPortals();
  cleanup();
});

const BASE = '/school-lms/school-dashboard';
const RANGE = 'preset=30d&from=2026-10-23&to=2026-11-21';
const GRADE3 = `/lms/schools/${IDS.schoolA}/grades/3/dashboard`;

describe('Trường → khối → học sinh (US2, ADR-0095)', () => {
  it('F056-AC-08/AC-09: overview grades (one under 5 students → "< 5") → grade page keeps range + subject; heatmap with symbols; cursor pages; breadcrumb school → grade → student; no class', async () => {
    const { backend, router } = await renderDashboard(`${BASE}?school=${IDS.schoolA}&subject=${IDS.subjectMath}&${RANGE}`);
    const assignments = await screen.findByTestId('widget-assignments');
    await waitFor(() => expect(within(assignments).getAllByTestId('grade-row')).toHaveLength(2));
    const rows = within(assignments).getAllByTestId('grade-row');
    expect(within(rows[1]!).getByTestId('suppressed-cell')).toHaveTextContent('< 5');
    expect(within(assignments).getAllByTestId('progress-by-grade')[1]!).toHaveTextContent('< 5');
    expect(document.body.textContent).not.toMatch(/\b3A\b|lớp/i);
    fireEvent.click(within(rows[0]!).getByRole('link', { name: 'Khối 3' }));
    const page = await screen.findByTestId('dashboard-grade');
    expect(router.state.location.pathname).toBe(`${BASE}/grades/3`);
    await waitFor(() => expect(backend.calls('GET', GRADE3).length).toBeGreaterThan(0));
    const q = new URLSearchParams(backend.calls('GET', GRADE3)[0]!.search);
    expect([q.get('from'), q.get('to'), q.get('subject_id'), q.get('page_size')]).toEqual(['2026-10-23', '2026-11-21', IDS.subjectMath, '100']);
    const crumbs = page.querySelector('.ant-breadcrumb') as HTMLElement;
    expect(within(crumbs).getByRole('link', { name: 'Tổng quan trường' })).toBeInTheDocument();
    expect(within(crumbs).getByText('Khối 3')).toBeInTheDocument();
    const heatmap = await screen.findByTestId('mastery-heatmap');
    const cell = within(heatmap).getByLabelText('TOAN.3.CHIA_DU (YCCĐ 3.12) · Mới bắt đầu: 2 học sinh');
    expect(cell).toHaveTextContent('◔');
    // students: names from listTenantGradeStudents, page 1 then "Xem thêm" (cursor)
    const students = screen.getByTestId('widget-students');
    expect(await within(students).findByText('Nguyễn An')).toBeInTheDocument();
    fireEvent.click(within(students).getByRole('button', { name: 'Xem thêm' }));
    expect(await within(students).findByText('Lê Bình')).toBeInTheDocument();
    expect(backend.calls('GET', GRADE3).some((r) => new URLSearchParams(r.search).get('cursor') === 'p2')).toBe(true);
    fireEvent.click(within(students).getByRole('link', { name: 'Nguyễn An' }));
    const studentPage = await screen.findByTestId('dashboard-student');
    expect(router.state.location.search).not.toMatch(/Nguy|class/);
    const crumbs2 = studentPage.querySelector('.ant-breadcrumb') as HTMLElement;
    expect(within(crumbs2).getByText('Khối 3')).toBeInTheDocument();
    expect(await within(crumbs2).findByText('Nguyễn An')).toBeInTheDocument();
    expect(backend.requests.some((r) => /\/classes(\/|$)/.test(r.path))).toBe(false);
  });

  it('F056-AC-08: a grade under the small-cell threshold shows "< 5" and no heatmap', async () => {
    await renderDashboard(`${BASE}/grades/4?school=${IDS.schoolA}&${RANGE}`);
    expect(await screen.findByTestId('grade-suppressed')).toBeInTheDocument();
    expect(screen.getAllByTestId('suppressed-cell').length).toBeGreaterThan(0);
    expect(screen.queryByTestId('mastery-heatmap')).toBeNull();
  });

  it('F056-AC-10: "Giao ôn tập theo LO" (grade heatmap and overview) opens the F055 wizard with mode=practice, the LO, the school, subject, grade and weak students', async () => {
    const { router } = await renderDashboard(`${BASE}/grades/3?school=${IDS.schoolA}&subject=${IDS.subjectMath}&${RANGE}`);
    const heatmap = await screen.findByTestId('mastery-heatmap');
    const row = within(heatmap).getByText('TOAN.3.CHIA_DU (YCCĐ 3.12)').closest('tr')!;
    fireEvent.click(within(row).getByRole('button', { name: 'Giao ôn tập theo LO' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/school-lms/assignments/new'));
    const q = new URLSearchParams(router.state.location.search);
    expect([q.get('mode'), q.get('objective_id'), q.get('school_id'), q.get('subject_id'), q.get('grade'), q.get('class_id')]).toEqual(['practice', DASH_IDS.loChiaDu, IDS.schoolA, IDS.subjectMath, '3', null]);
    expect(q.get('student_ids')!.split(',')).toEqual([IDS.studentAn, LMS_IDS.studentBinh]);
    expect(await screen.findByText('Giao bài mới')).toBeInTheDocument();
    stopPortals();
    cleanup();
    // overview: weak LO of one grade → skill map of that grade and the practice link with the grade
    const r2 = await renderDashboard(`${BASE}?school=${IDS.schoolA}&subject=${IDS.subjectMath}&${RANGE}`);
    const weak = await screen.findByTestId('widget-weak');
    expect(await within(weak).findByText(/55% học sinh dưới mức thành thạo · Khối 3/)).toBeInTheDocument();
    expect(within(weak).getByRole('link', { name: 'Bản đồ kỹ năng' })).toHaveAttribute('href', expect.stringContaining(`${BASE}/grades/3?`));
    fireEvent.click(within(weak).getByRole('link', { name: 'Giao ôn tập theo LO' }));
    await waitFor(() => expect(r2.router.state.location.pathname).toBe('/school-lms/assignments/new'));
    expect(new URLSearchParams(r2.router.state.location.search).get('grade')).toBe('3');
    stopPortals();
    cleanup();
    // without lms.assignment.write the shortcut is not offered
    await renderDashboard(`${BASE}/grades/3?school=${IDS.schoolA}&${RANGE}`, { account: adTruongA(SCHOOL_ADMIN_DASH_PERMS.filter((p) => p !== 'lms.assignment.write')) });
    await screen.findByTestId('mastery-heatmap');
    expect(screen.queryByRole('button', { name: 'Giao ôn tập theo LO' })).toBeNull();
  });

  it('F056-AC-11: profile tabs (mastery, trend, history, recommendations, login activity); no rating/friends/chat/IP/device', async () => {
    const { backend } = await renderDashboard(`${BASE}/students/${IDS.studentAn}?school=${IDS.schoolA}&${RANGE}`);
    const profile = await screen.findByTestId('widget-profile');
    // opened by URL (no link state): neutral name, never a class
    expect(await within(profile).findByText('Hồ sơ học tập: Học sinh')).toBeInTheDocument();
    const q = new URLSearchParams(backend.calls('GET', `/lms/students/${IDS.studentAn}/analytics`)[0]!.search);
    expect([q.get('from'), q.get('to')]).toEqual(['2026-10-23', '2026-11-21']);
    for (const tab of ['Mức thành thạo', 'Xu hướng 8 tuần', 'Lịch sử bài', 'Gợi ý đang hiển thị', 'Hoạt động đăng nhập']) expect(within(profile).getByRole('tab', { name: tab })).toBeInTheDocument();
    expect(within(profile).getByLabelText('Nhân chia trong bảng · Thành thạo: 3 LO')).toHaveTextContent('◕');
    fireEvent.click(within(profile).getByRole('tab', { name: 'Lịch sử bài' }));
    expect(await within(profile).findByText('Bảng nhân 6–9')).toBeInTheDocument();
    expect(within(profile).getByText('8/10')).toBeInTheDocument();
    fireEvent.click(within(profile).getByRole('tab', { name: 'Gợi ý đang hiển thị' }));
    expect(await within(profile).findByText('Luyện LO còn yếu')).toBeInTheDocument();
    fireEvent.click(within(profile).getByRole('tab', { name: 'Hoạt động đăng nhập' }));
    expect(await within(profile).findByTestId('login-activity')).toBeInTheDocument();
    expect(await within(profile).findByText('Windows · PC')).toBeInTheDocument();
    const text = document.body.textContent ?? '';
    expect(text).not.toMatch(/203\.0\.113\.7|deadbeefcafe|raw-device-123|Game Rating|bạn bè|vị trí|tin nhắn/i);
    expect(backend.requests.some((r) => /friends|chat|rating|position|classes/.test(r.path))).toBe(false);
    stopPortals();
    cleanup();
    // without lms.student_activity.read: no login tab, no login-events request
    const r2 = await renderDashboard(`${BASE}/students/${IDS.studentAn}?school=${IDS.schoolA}&${RANGE}`, { account: adTruongA(SCHOOL_ADMIN_DASH_PERMS.filter((p) => p !== 'lms.student_activity.read')) });
    const p2 = await screen.findByTestId('widget-profile');
    await within(p2).findByRole('tab', { name: 'Mức thành thạo' });
    expect(within(p2).queryByRole('tab', { name: 'Hoạt động đăng nhập' })).toBeNull();
    expect(r2.backend.requests.some((r) => r.path.endsWith('/login-events'))).toBe(false);
  });

  it('F056-AC-11/AC-21: a student outside the scope → "Không tìm thấy" without data', async () => {
    await renderDashboard(`${BASE}/students/${LMS_IDS.studentBinh}?school=${IDS.schoolA}&${RANGE}`);
    expect(await screen.findByTestId('school-lms-not-found')).toBeInTheDocument();
    expect(screen.queryByTestId('widget-profile')).toBeNull();
  });

  it('ADR-0095: the old class URL of the dashboard is gone (no class page, no class request)', async () => {
    const { backend } = await renderDashboard(`${BASE}/classes/${IDS.class3A}?school=${IDS.schoolA}&${RANGE}`);
    await screen.findByRole('heading', { level: 1 });
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(0));
    expect(screen.queryByTestId('dashboard-class')).toBeNull();
    expect(backend.requests.some((r) => /\/lms\/classes\//.test(r.path))).toBe(false);
  });
});
