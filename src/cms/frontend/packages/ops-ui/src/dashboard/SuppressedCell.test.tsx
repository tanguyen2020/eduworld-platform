// F056 T016 — F056-AC-06 (TC-DASH-009): a suppressed value shows "< 5" with a tooltip, never a number.
import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderOps } from '../test/render';
import { DashboardMetric } from './DashboardWidget';
import { SuppressedCell } from './SuppressedCell';

describe('SuppressedCell (F056-AC-06)', () => {
  it('shows "< 5" and explains it in a tooltip', async () => {
    await renderOps(<SuppressedCell />);
    const cell = screen.getByTestId('suppressed-cell');
    expect(cell).toHaveTextContent('< 5');
    expect(cell).toHaveAttribute('aria-label', 'Dưới 5 học sinh, số liệu được ẩn');
    fireEvent.mouseEnter(cell);
    expect(await screen.findByText('Nhóm dưới 5 học sinh: số liệu được ẩn để bảo vệ học sinh.')).toBeInTheDocument();
  });

  it('DashboardMetric with suppressed=true never renders the value', async () => {
    await renderOps(<DashboardMetric label="HS hoạt động" value={3} suppressed testId="m" />);
    expect(screen.getByTestId('m')).toHaveTextContent('< 5');
    expect(screen.getByTestId('m')).not.toHaveTextContent('3');
  });
});
