// F056 T033 — F056-AC-14 (TC-DASH-008): the export button shows "Đang chuẩn bị báo cáo" while the job runs, the signed
// link with its expiry time only when the job succeeded, never a direct file; after expiry the link disappears and the
// button offers to create the report again (F056-AC-15); failure codes and refused requests are explained.
import { act, fireEvent, screen } from '@testing-library/react';
import dayjs from 'dayjs';
import { describe, expect, it, vi } from 'vitest';
import { renderOps } from '../test/render';
import { ExportButton, linkUsable } from './ExportButton';

const NOW = Date.parse('2026-11-21T03:00:00Z');
const expires = '2026-11-21T03:15:00Z';

describe('ExportButton (F056-AC-14, F056-AC-15)', () => {
  it('running job: "Đang chuẩn bị báo cáo", button disabled, no link', async () => {
    const onExport = vi.fn();
    await renderOps(<ExportButton onExport={onExport} job={{ status: 'running' }} now={() => NOW} />);
    expect(screen.getByRole('status')).toHaveTextContent('Đang chuẩn bị báo cáo…');
    expect(screen.getByRole('button', { name: 'Xuất báo cáo' })).toBeDisabled();
    expect(screen.queryByTestId('export-link')).toBeNull();
  });

  it('succeeded job: link + expiry time; click export calls onExport', async () => {
    const onExport = vi.fn();
    await renderOps(<ExportButton onExport={onExport} job={{ status: 'succeeded', download_url: 'https://files.example/r.xlsx?sig=1', download_expires_at: expires }} now={() => NOW} />);
    expect(screen.getByTestId('export-link')).toHaveAttribute('href', 'https://files.example/r.xlsx?sig=1');
    expect(screen.getByText(`Link hết hạn lúc ${dayjs(expires).format('HH:mm')}`)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Xuất báo cáo' }));
    expect(onExport).toHaveBeenCalledTimes(1);
  });

  it('expired link (time passed or status expired) → no link, "Tạo lại báo cáo"', async () => {
    expect(linkUsable({ status: 'succeeded', download_url: 'u', download_expires_at: expires }, Date.parse(expires) + 1)).toBe(false);
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    let now = NOW;
    try {
      await renderOps(<ExportButton onExport={vi.fn()} job={{ status: 'succeeded', download_url: 'https://files.example/r.xlsx', download_expires_at: expires }} now={() => now} />);
      expect(screen.getByTestId('export-link')).toBeInTheDocument();
      now = Date.parse(expires) + 1000;
      await act(async () => {
        vi.advanceTimersByTime(15 * 60 * 1000 + 100);
      });
      expect(screen.queryByTestId('export-link')).toBeNull();
      expect(screen.getByRole('button', { name: 'Tạo lại báo cáo' })).toBeEnabled();
      expect(screen.getByText('Link tải đã hết hạn. Bấm để tạo lại báo cáo.')).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it('failed job and refused request messages', async () => {
    const r = await renderOps(<ExportButton onExport={vi.fn()} job={{ status: 'failed', error: 'too_many_rows' }} now={() => NOW} />);
    expect(screen.getByText('Báo cáo quá lớn. Hãy thu hẹp phạm vi.')).toBeInTheDocument();
    r.unmount();
    await renderOps(<ExportButton onExport={vi.fn()} errorMessage="Bạn đang có quá nhiều báo cáo đang chạy." now={() => NOW} />);
    expect(screen.getByRole('alert')).toHaveTextContent('Bạn đang có quá nhiều báo cáo đang chạy.');
  });
});
