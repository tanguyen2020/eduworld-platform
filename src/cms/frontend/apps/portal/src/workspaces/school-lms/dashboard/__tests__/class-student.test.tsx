// F056 T022, T024, T025, T044 (US2, US6) — F056-AC-08 (class drill-down keeps range/subject, breadcrumb school → class
// → student), F056-AC-09 (MasteryHeatmap with symbols on the class page), F056-AC-10 ("Giao ôn tập theo LO" opens the
// F055 wizard with mode=practice, LO, class, weak students; finishing it is the wizard's "Giao bài"), F056-AC-11
// (profile tabs; no Game Rating / position / friends / chat / IP / device id), F056-AC-22 (student search limited by
// the OpenAPI `q` maxLength). Backend = MOCK.
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { DASH_IDS, IDS, LMS_IDS, adTruongA, renderDashboard, SCHOOL_ADMIN_DASH_PERMS, stopPortals } from '../../../../test/dashboard-harness';

afterEach(() => {
  stopPortals();
  cleanup();
});

const BASE = '/school-lms/school-dashboard';
const RANGE = 'preset=30d&from=2026-10-23&to=2026-11-21';

describe('Drill-down lớp và hồ sơ học sinh (US2)', () => {
  it('F056-AC-08/AC-09: from the overview to the class keeps range + subject; breadcrumb; heatmap with symbols; cursor pages', async () => {
    const { backend, router } = await renderDashboard(`${BASE}?school=${IDS.schoolA}&subject=${IDS.subjectMath}&${RANGE}`);
    const assignments = await screen.findByTestId('widget-assignments');
    fireEvent.click((await within(assignments).findAllByRole('link', { name: '3A' }))[0]!);
    const classPage = await screen.findByTestId('dashboard-class');
    expect(router.state.location.pathname).toBe(`${BASE}/classes/${IDS.class3A}`);
    await waitFor(() => expect(backend.calls('GET', `/lms/classes/${IDS.class3A}/dashboard`).length).toBeGreaterThan(0));
    const q = new URLSearchParams(backend.calls('GET', `/lms/classes/${IDS.class3A}/dashboard`)[0]!.search);
    expect([q.get('from'), q.get('to'), q.get('subject_id'), q.get('page_size')]).toEqual(['2026-10-23', '2026-11-21', IDS.subjectMath, '100']);
    const crumbs = classPage.querySelector('.ant-breadcrumb')!;
    expect(within(crumbs as HTMLElement).getByRole('link', { name: 'Tổng quan trường' })).toBeInTheDocument();
    expect(await within(crumbs as HTMLElement).findByText('3A')).toBeInTheDocument();
    const heatmap = await screen.findByTestId('mastery-heatmap');
    const cell = within(heatmap).getByLabelText('TOAN.3.CHIA_DU (YCCĐ 3.12) · Mới bắt đầu: 2 học sinh');
    expect(cell).toHaveTextContent('◔');
    expect(cell).toHaveAttribute('tabindex', '0');
    // students: page 1 then "Tải thêm" (cursor)
    const students = screen.getByTestId('widget-students');
    expect(await within(students).findByText('Nguyễn An')).toBeInTheDocument();
    fireEvent.click(within(students).getByRole('button', { name: 'Xem thêm' }));
    // page 2 holds Lê Bình, who is not in the (MOCK) class roster of listTenantClassStudents → neutral fallback name
    expect(await within(students).findByRole('link', { name: 'Học sinh' })).toBeInTheDocument();
    expect(backend.calls('GET', `/lms/classes/${IDS.class3A}/dashboard`).some((r) => new URLSearchParams(r.search).get('cursor') === 'p2')).toBe(true);
    // to the student: breadcrumb school → class → student
    fireEvent.click(within(students).getByRole('link', { name: 'Nguyễn An' }));
    const studentPage = await screen.findByTestId('dashboard-student');
    const crumbs2 = studentPage.querySelector('.ant-breadcrumb') as HTMLElement;
    expect(within(crumbs2).getByRole('link', { name: '3A' })).toHaveAttribute('href', expect.stringContaining(`${BASE}/classes/${IDS.class3A}?`));
    expect(await within(crumbs2).findByText('Nguyễn An')).toBeInTheDocument();
  });

  it('F056-AC-10: "Giao ôn tập theo LO" opens the F055 wizard with mode=practice, the LO, the class and the weak students', async () => {
    const { router } = await renderDashboard(`${BASE}/classes/${IDS.class3A}?school=${IDS.schoolA}&${RANGE}`);
    const heatmap = await screen.findByTestId('mastery-heatmap');
    const row = within(heatmap).getByText('TOAN.3.CHIA_DU (YCCĐ 3.12)').closest('tr')!;
    fireEvent.click(within(row).getByRole('button', { name: 'Giao ôn tập theo LO' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/school-lms/assignments/new'));
    const q = new URLSearchParams(router.state.location.search);
    expect(q.get('mode')).toBe('practice');
    expect(q.get('objective_id')).toBe(DASH_IDS.loChiaDu);
    expect(q.get('class_id')).toBe(IDS.class3A);
    expect(q.get('student_ids')!.split(',')).toEqual([IDS.studentAn, LMS_IDS.studentBinh]);
    expect(await screen.findByText('Giao bài mới')).toBeInTheDocument();
    stopPortals();
    cleanup();
    // without lms.assignment.write the shortcut is not offered
    await renderDashboard(`${BASE}/classes/${IDS.class3A}?school=${IDS.schoolA}&${RANGE}`, { account: adTruongA(SCHOOL_ADMIN_DASH_PERMS.filter((p) => p !== 'lms.assignment.write')) });
    await screen.findByTestId('mastery-heatmap');
    expect(screen.queryByRole('button', { name: 'Giao ôn tập theo LO' })).toBeNull();
  });

  it('F056-AC-11: profile tabs (mastery, trend, history, recommendations, login activity); no rating/friends/chat/IP/device', async () => {
    const { backend } = await renderDashboard(`${BASE}/students/${IDS.studentAn}?school=${IDS.schoolA}&class=${IDS.class3A}&${RANGE}`);
    const profile = await screen.findByTestId('widget-profile');
    expect(await within(profile).findByText('Hồ sơ học tập: Nguyễn An')).toBeInTheDocument();
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
    expect(backend.requests.some((r) => /friends|chat|rating|position/.test(r.path))).toBe(false);
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

  it('F056-AC-22: the student search is capped at the OpenAPI maxLength of listTenantClassStudents.q', async () => {
    const { backend } = await renderDashboard(`${BASE}/classes/${IDS.class3A}?school=${IDS.schoolA}&${RANGE}`);
    const box = await screen.findByLabelText('Tìm học sinh theo tên');
    expect(box).toHaveAttribute('maxlength', '100');
    fireEvent.change(box, { target: { value: 'An' } });
    fireEvent.keyDown(box, { key: 'Enter', code: 'Enter', keyCode: 13 });
    await waitFor(() => expect(backend.calls('GET', `/tenant/schools/${IDS.schoolA}/classes/${IDS.class3A}/students`).some((r) => new URLSearchParams(r.search).get('q') === 'An')).toBe(true));
  });
});
