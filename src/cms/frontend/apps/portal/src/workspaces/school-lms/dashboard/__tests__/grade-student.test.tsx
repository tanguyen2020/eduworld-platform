// F056 T022, T024, T025, T044 (US2, US6) rewritten for ADR-0095 (no class level) — F056-AC-08 (school → grade → student:
// the grade filter keeps range/subject, breadcrumb school → grade → student, no class page, list or link even when the
// server still sends class fields), F056-AC-10 ("Giao ôn tập theo LO" opens the F055 wizard with mode=practice, the LO,
// the school, subject and grade of the filters), F056-AC-11 (profile tabs; no Game Rating / position / friends / chat /
// IP / device id), F056-AC-21 (out of scope → "Không tìm thấy"). Backend = MOCK.
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { DASH_IDS, IDS, LMS_IDS, adTruongA, renderDashboard, SCHOOL_ADMIN_DASH_PERMS, stopPortals } from '../../../../test/dashboard-harness';

afterEach(() => {
  stopPortals();
  cleanup();
});

const BASE = '/school-lms/school-dashboard';
const RANGE = 'preset=30d&from=2026-10-23&to=2026-11-21';

describe('Trường → khối → học sinh (US2, ADR-0095)', () => {
  it('F056-AC-08: grade filter (grades of the school, 1–9) keeps range + subject; no class anywhere; student breadcrumb school → grade → student', async () => {
    const { backend, router } = await renderDashboard(`${BASE}?school=${IDS.schoolA}&subject=${IDS.subjectMath}&${RANGE}`);
    await screen.findByTestId('widget-assignments');
    await waitFor(() => expect(screen.getByTestId('metric-assignments-open')).toHaveTextContent('2'));
    // no class filter, link, list or name although the MOCK server still sends class fields (WP-L2 removes them)
    expect(screen.queryByRole('combobox', { name: 'Lớp' })).toBeNull();
    expect(screen.queryByRole('link', { name: '3A' })).toBeNull();
    expect(document.body.textContent).not.toMatch(/\b3A\b|lớp/i);
    expect(within(screen.getByTestId('widget-assignments')).getByText('2/3')).toBeInTheDocument();
    // grade options = grades the school teaches (TH Demo A: 1–5)
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Khối' }));
    expect(await screen.findByTitle('Khối 5')).toBeInTheDocument();
    expect(screen.queryByTitle('Khối 6')).toBeNull();
    fireEvent.click(screen.getByTitle('Khối 3'));
    await waitFor(() => {
      const q = new URLSearchParams(backend.calls('GET', '/lms/dashboards/school').at(-1)!.search);
      expect([q.get('grade_level'), q.get('subject_id'), q.get('from'), q.get('to'), q.get('class_id')]).toEqual(['3', IDS.subjectMath, '2026-10-23', '2026-11-21', null]);
    });
    // to the student (the name travels with the link, never in the URL)
    const attention = screen.getByTestId('widget-attention');
    fireEvent.click(within(attention).getByRole('link', { name: 'Nguyễn An' }));
    const studentPage = await screen.findByTestId('dashboard-student');
    expect(router.state.location.pathname).toBe(`${BASE}/students/${IDS.studentAn}`);
    expect(router.state.location.search).not.toMatch(/Nguy|class/);
    const crumbs = studentPage.querySelector('.ant-breadcrumb') as HTMLElement;
    expect(within(crumbs).getByRole('link', { name: 'Tổng quan trường' })).toHaveAttribute('href', expect.stringContaining('grade=3'));
    expect(within(crumbs).getByText('Khối 3')).toBeInTheDocument();
    expect(await within(crumbs).findByText('Nguyễn An')).toBeInTheDocument();
    expect(backend.requests.some((r) => /\/classes(\/|$)/.test(r.path))).toBe(false);
  });

  it('F056-AC-10: "Giao ôn tập theo LO" opens the F055 wizard with mode=practice, the LO, the school, subject and grade', async () => {
    const { router } = await renderDashboard(`${BASE}?school=${IDS.schoolA}&subject=${IDS.subjectMath}&grade=3&${RANGE}`);
    const weak = await screen.findByTestId('widget-weak');
    expect(await within(weak).findByText('55% học sinh dưới mức thành thạo')).toBeInTheDocument();
    expect(within(weak).queryByRole('link', { name: 'Bản đồ kỹ năng' })).toBeNull();
    fireEvent.click(within(weak).getByRole('link', { name: 'Giao ôn tập theo LO' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/school-lms/assignments/new'));
    const q = new URLSearchParams(router.state.location.search);
    expect(Object.fromEntries(q)).toEqual({ mode: 'practice', objective_id: DASH_IDS.loChiaDu, school_id: IDS.schoolA, subject_id: IDS.subjectMath, grade: '3' });
    expect(await screen.findByText('Giao bài mới')).toBeInTheDocument();
    stopPortals();
    cleanup();
    // without lms.assignment.write the shortcut is not offered
    await renderDashboard(`${BASE}?school=${IDS.schoolA}&${RANGE}`, { account: adTruongA(SCHOOL_ADMIN_DASH_PERMS.filter((p) => p !== 'lms.assignment.write')) });
    await screen.findByText('55% học sinh dưới mức thành thạo');
    expect(screen.queryByRole('link', { name: 'Giao ôn tập theo LO' })).toBeNull();
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
