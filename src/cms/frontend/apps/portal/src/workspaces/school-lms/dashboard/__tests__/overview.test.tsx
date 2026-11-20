// F056 T011, T014, T015, T017 (US1) — F056-AC-01 (getSchoolDashboard with school_id/from/to + filters, 5 widgets in the
// order of the 5 questions), F056-AC-02 (updated-at, grain, "Tạm tính", "Đang tính số liệu…" instead of 0), F056-AC-04
// (DASHBOARD_* hints, BUSY retried by the apiClient, at most dashboard_max_concurrent_per_user in flight), F056-AC-05
// (neutral reason labels, "làm quá nhanh", no ranking), F056-AC-07 (safety: count + link only, no chat content).
// Backend = MOCK.
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import dayjs from 'dayjs';
import { afterEach, describe, expect, it } from 'vitest';
import { AS_OF, IDS, problem, renderDashboard, schoolDashboardFixture, stopPortals } from '../../../../test/dashboard-harness';

afterEach(() => {
  stopPortals();
  cleanup();
});

const URL = '/school-lms/school-dashboard';

describe('Tổng quan trường (US1)', () => {
  it('F056-AC-01/AC-02: getSchoolDashboard(school_id, from, to, filters) → 5 widgets in order with data time, grain, Tạm tính', async () => {
    const { backend } = await renderDashboard(URL);
    const widgets = await screen.findAllByTestId(/^widget-/);
    await waitFor(() => expect(screen.getByTestId('metric-assignments-open')).toHaveTextContent('2'));
    expect(screen.getAllByTestId(/^widget-/).map((w) => w.getAttribute('data-testid'))).toEqual(['widget-assignments', 'widget-weak', 'widget-attention', 'widget-selfStudy', 'widget-safety']);
    expect(widgets).toHaveLength(5);
    const call = backend.calls('GET', '/lms/dashboards/school').at(-1)!;
    const q = new URLSearchParams(call.search);
    expect(q.get('school_id')).toBe(IDS.schoolA);
    expect(dayjs(q.get('to')!).diff(dayjs(q.get('from')!), 'day')).toBe(6);
    expect(call.headers['x-tenant-id']).toBe('0193a4d2-0000-7000-8000-00000000000a');
    for (const w of screen.getAllByTestId(/^widget-/)) {
      const meta = within(w).getByTestId('dashboard-meta');
      expect(meta).toHaveTextContent(`Cập nhật lúc ${dayjs(AS_OF).format('HH:mm')}`);
      expect(meta).toHaveTextContent('Theo ngày');
      expect(meta).toHaveTextContent('Tạm tính');
    }
    expect(screen.getByTestId('metric-completion')).toHaveTextContent('60%');
    expect(screen.getByTestId('metric-avg-minutes')).toHaveTextContent('42 phút');
    expect(screen.getByTestId('metric-reco-started')).toHaveTextContent('68%');
    expect(screen.getByTestId('metric-improved')).toHaveTextContent('37');
    // subject filter → new request with subject_id
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Môn' }));
    fireEvent.click(await screen.findByTitle('Toán'));
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/lms/dashboards/school').at(-1)!.search).get('subject_id')).toBe(IDS.subjectMath));
    fireEvent.click(screen.getByText('30 ngày'));
    await waitFor(() => {
      const last = new URLSearchParams(backend.calls('GET', '/lms/dashboards/school').at(-1)!.search);
      expect(dayjs(last.get('to')!).diff(dayjs(last.get('from')!), 'day')).toBe(29);
    });
  });

  it('F056-AC-02: a missing value of an unclosed period shows "Đang tính số liệu…", not 0; empty school → guidance', async () => {
    await renderDashboard(URL, {
      setup: (d) => {
        d.school = () => {
          const s = schoolDashboardFixture() as { widgets: Record<string, unknown> };
          s.widgets.self_study = { avg_active_minutes_per_student: null, recommendation_started_rate: null, objectives_improved_after_recommendation: 3 };
          s.widgets.weak_objectives = [];
          return s;
        };
      },
    });
    expect(await screen.findByTestId('metric-avg-minutes')).toHaveTextContent('Đang tính số liệu…');
    expect(screen.getByTestId('metric-reco-started')).toHaveTextContent('Đang tính số liệu…');
    expect(within(screen.getByTestId('widget-weak')).getByText('Chưa có LO nào cần củng cố trong khoảng này.')).toBeInTheDocument();
  });

  it('F056-AC-05: neutral reason labels incl. "Làm quá nhanh", link to the profile, no ranking or score column', async () => {
    await renderDashboard(URL);
    const w = await screen.findByTestId('widget-attention');
    expect(await within(w).findByText('2 học sinh cần hỗ trợ')).toBeInTheDocument();
    const an = within(w).getByText('Nguyễn An');
    expect(an.closest('a')).toHaveAttribute('href', expect.stringContaining(`/school-lms/school-dashboard/students/${IDS.studentAn}?`));
    expect(within(w).getByTestId('reason-rapid_guess')).toHaveTextContent('Làm quá nhanh');
    expect(within(w).getByTestId('reason-missed_assignments')).toHaveTextContent('Chưa làm bài giao gần đây');
    expect(within(w).getByTestId('reason-inactive_game')).toHaveTextContent('Chưa vào học gần đây');
    expect(w.textContent).not.toMatch(/kém|xếp hạng|hạng \d|điểm/i);
  });

  it('F056-AC-07: safety widget = count + link to the safety notifications screen; no chat content anywhere', async () => {
    const { backend } = await renderDashboard(URL);
    const w = await screen.findByTestId('widget-safety');
    await waitFor(() => expect(within(w).getByTestId('metric-safety')).toHaveTextContent('1'));
    expect(within(w).getByRole('link', { name: 'Xem thông báo an toàn' })).toHaveAttribute('href', '/school-lms/safety-notifications');
    expect(backend.requests.some((r) => /chat|messages/.test(r.path))).toBe(false);
    expect(document.body.textContent).not.toMatch(/tin nhắn|nội dung chat/i);
  });

  it('F056-AC-04: DASHBOARD_RANGE_TOO_LARGE / DASHBOARD_QUERY_TIMEOUT → hints in every widget, timeout not repeated', async () => {
    const { backend } = await renderDashboard(URL, { setup: (d) => (d.next.school = [problem(504, 'DASHBOARD_QUERY_TIMEOUT')]) });
    const alerts = await screen.findAllByText(/Hãy thu hẹp phạm vi/);
    expect(alerts).toHaveLength(5);
    expect(backend.calls('GET', '/lms/dashboards/school')).toHaveLength(1);
    stopPortals();
    cleanup();
    await renderDashboard(URL, { setup: (d) => (d.next.school = [problem(422, 'DASHBOARD_RANGE_TOO_LARGE')]) });
    expect(await screen.findAllByText(/Hãy thu hẹp khoảng thời gian/)).toHaveLength(5);
  });

  it('F056-AC-04: 429 DASHBOARD_BUSY is retried after 2–5 s and the widgets then load', async () => {
    const busy = () => new Response(JSON.stringify({ type: 'about:blank', title: 'x', status: 429, code: 'DASHBOARD_BUSY', trace_id: 't' }), { status: 429, headers: { 'Content-Type': 'application/problem+json', 'Retry-After': '1' } });
    const { backend } = await renderDashboard(URL, { setup: (d) => (d.next.school = [busy()]) });
    const started = Date.now();
    await waitFor(() => expect(screen.getByTestId('metric-assignments-open')).toHaveTextContent('2'), { timeout: 15_000 });
    expect(Date.now() - started).toBeGreaterThanOrEqual(1900);
    expect(backend.calls('GET', '/lms/dashboards/school').length).toBeGreaterThanOrEqual(2);
  });
});
