// F056 T023 — F056-AC-09 (TC-UI-006, NFR-A11Y-003): the heatmap is a real table (caption, column/row headers), each
// cell has the level symbol ○◔◑◕● next to a token color, is reachable with the keyboard, has an aria-label and a tooltip
// with the exact value; the legend names every level (color is not the only channel).
import { fireEvent, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderOps } from '../test/render';
import { MASTERY_LEVELS, MASTERY_SYMBOL, MasteryHeatmap } from './MasteryHeatmap';

const columns = MASTERY_LEVELS.map((l) => ({ key: l, label: l }));
const rows = [
  {
    key: 'lo1',
    header: 'TOAN.3.CHIA_DU',
    cells: MASTERY_LEVELS.map((level, i) => ({ level, text: String(i), description: `Chia có dư · ${level}: ${i} học sinh`, muted: i === 0 })),
  },
];

describe('MasteryHeatmap (F056-AC-09)', () => {
  it('table semantics, symbol in every cell, aria-label + tooltip with the exact value, keyboard focus', async () => {
    await renderOps(<MasteryHeatmap caption="Bản đồ kỹ năng lớp 3A" rowHeader="LO" columns={columns} rows={rows} />);
    const table = screen.getByTestId('mastery-heatmap');
    expect(within(table).getByText('Bản đồ kỹ năng lớp 3A').tagName).toBe('CAPTION');
    expect(within(table).getAllByRole('columnheader')).toHaveLength(6);
    expect(within(table).getByRole('rowheader')).toHaveTextContent('TOAN.3.CHIA_DU');
    for (const [i, level] of MASTERY_LEVELS.entries()) {
      const cell = within(table).getByLabelText(`Chia có dư · ${level}: ${i} học sinh`);
      expect(cell).toHaveTextContent(MASTERY_SYMBOL[level]);
      expect(cell).toHaveAttribute('tabindex', '0');
      expect(cell).toHaveAttribute('data-level', level);
      expect(cell.getAttribute('style') ?? '').toMatch(/background/);
    }
    const cell = within(table).getByLabelText('Chia có dư · beginning: 1 học sinh');
    cell.focus();
    expect(cell).toHaveFocus();
    fireEvent.mouseEnter(cell);
    expect(await screen.findByRole('tooltip')).toHaveTextContent('Chia có dư · beginning: 1 học sinh');
  });

  it('legend lists the five levels with their symbol and name', async () => {
    await renderOps(<MasteryHeatmap caption="c" rowHeader="LO" columns={columns} rows={rows} />);
    const legend = screen.getByTestId('mastery-legend');
    const items = within(legend).getAllByRole('listitem');
    expect(items.map((i) => i.textContent)).toEqual(['○Chưa bắt đầu', '◔Mới bắt đầu', '◑Đang tiến bộ', '◕Thành thạo', '●Xuất sắc']);
  });
});
