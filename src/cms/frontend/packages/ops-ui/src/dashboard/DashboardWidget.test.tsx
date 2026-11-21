// F056 T012 — F056-AC-02 (TC-DASH-003): "Cập nhật lúc HH:mm" from data_as_of, grain label, "Tạm tính" when is_partial,
// "Đang tính số liệu…" instead of 0 for a missing value of an unclosed period; F056-AC-04 hints by Problem code, own
// skeleton per widget; empty state with guidance.
import { screen } from '@testing-library/react';
import dayjs from 'dayjs';
import { describe, expect, it } from 'vitest';
import { renderOps } from '../test/render';
import { DashboardMetric, DashboardWidget } from './DashboardWidget';

const asOf = '2026-11-21T03:05:00Z';

describe('DashboardWidget (F056-AC-02, F056-AC-04)', () => {
  it('shows updated-at time, grain and "Tạm tính"', async () => {
    await renderOps(
      <DashboardWidget title="Bài giao" testId="w" meta={{ data_as_of: asOf, grain: 'week', is_partial: true }}>
        x
      </DashboardWidget>,
    );
    const meta = screen.getByTestId('dashboard-meta');
    expect(meta).toHaveTextContent(`Cập nhật lúc ${dayjs(asOf).format('HH:mm')}`);
    expect(meta).toHaveTextContent('Theo tuần');
    expect(meta).toHaveTextContent('Tạm tính');
  });

  it('F056 T052: "Chưa có số liệu trước ngày …" when available_from is after the range start; replica lag warning', async () => {
    const r = await renderOps(
      <DashboardWidget title="A" meta={{ data_as_of: asOf, grain: 'day', is_partial: false, from: '2026-09-01', available_from: '2026-10-15', replica_lag_warning: true }} />,
    );
    const meta = screen.getByTestId('dashboard-meta');
    expect(meta).toHaveTextContent('Chưa có số liệu trước ngày 15/10/2026');
    expect(meta).toHaveTextContent('Số liệu có thể chậm');
    r.unmount();
    await renderOps(<DashboardWidget title="A" meta={{ data_as_of: asOf, grain: 'day', is_partial: false, from: '2026-11-01', available_from: '2026-10-15', replica_lag_warning: false }} />);
    expect(screen.getByTestId('dashboard-meta')).not.toHaveTextContent('Chưa có số liệu trước ngày');
    expect(screen.getByTestId('dashboard-meta')).not.toHaveTextContent('Số liệu có thể chậm');
  });

  it('no "Tạm tính" when the data is final; daily/monthly labels', async () => {
    const { unmount } = await renderOps(<DashboardWidget title="A" meta={{ data_as_of: asOf, grain: 'day', is_partial: false }} />);
    expect(screen.getByTestId('dashboard-meta')).toHaveTextContent('Theo ngày');
    expect(screen.queryByText('Tạm tính')).toBeNull();
    unmount();
    await renderOps(<DashboardWidget title="A" meta={{ data_as_of: asOf, grain: 'month', is_partial: false }} />);
    expect(screen.getByTestId('dashboard-meta')).toHaveTextContent('Theo tháng');
  });

  it('missing value of an unclosed period → "Đang tính số liệu…", never 0; closed → "Chưa có số liệu"; 0 stays 0', async () => {
    const r1 = await renderOps(<DashboardMetric label="Phút học" value={null} partial testId="m" />);
    expect(screen.getByTestId('m')).toHaveTextContent('Đang tính số liệu…');
    expect(screen.getByTestId('m')).not.toHaveTextContent('0');
    r1.unmount();
    const r2 = await renderOps(<DashboardMetric label="Phút học" value={undefined} testId="m" />);
    expect(screen.getByTestId('m')).toHaveTextContent('Chưa có số liệu');
    r2.unmount();
    const r3 = await renderOps(<DashboardMetric label="Tỉ lệ" value={0.684} format="percent" testId="m" />);
    expect(screen.getByTestId('m')).toHaveTextContent('68%');
    r3.unmount();
    const r4 = await renderOps(<DashboardMetric label="Bài" value={0} testId="m" />);
    expect(screen.getByTestId('m')).toHaveTextContent('0');
    r4.unmount();
    await renderOps(<DashboardMetric label="Phút" value={42} format="minutes" testId="m" />);
    expect(screen.getByTestId('m')).toHaveTextContent('42 phút');
  });

  it('own skeleton while loading; hints for DASHBOARD_RANGE_TOO_LARGE / QUERY_TIMEOUT / BUSY; other codes use the message', async () => {
    const r = await renderOps(<DashboardWidget title="A" loading />);
    expect(screen.getByTestId('dashboard-widget-skeleton')).toBeInTheDocument();
    r.unmount();
    const cases: [string, string][] = [
      ['DASHBOARD_RANGE_TOO_LARGE', 'Hãy thu hẹp khoảng thời gian'],
      ['DASHBOARD_QUERY_TIMEOUT', 'Hãy thu hẹp phạm vi'],
      ['DASHBOARD_BUSY', 'Hệ thống đang bận'],
      ['PERMISSION_DENIED', 'Ngoài phạm vi quyền'],
    ];
    for (const [code, text] of cases) {
      const c = await renderOps(<DashboardWidget title="A" error={{ code, message: 'Ngoài phạm vi quyền' }} meta={{ data_as_of: asOf, grain: 'day', is_partial: false }} />);
      expect(screen.getByRole('alert')).toHaveTextContent(text);
      expect(screen.queryByTestId('dashboard-meta')).toBeNull();
      c.unmount();
    }
  });

  it('empty state with guidance instead of zeros (start of the school year)', async () => {
    await renderOps(<DashboardWidget title="A" empty emptyHint="Chưa có bài giao. Hãy giao bài đầu tiên." />);
    expect(screen.getByText('Chưa có bài giao. Hãy giao bài đầu tiên.')).toBeInTheDocument();
  });
});
