// F079 T019 — ManifestRefPicker (F079-AC-07): a reference (area, NPC, scene, item…) is chosen from the client manifest /
// published content only — a searchable list, no free id: typing an unknown value selects nothing; a stored value that
// is not in the list any more is flagged.
import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { renderOps } from '../../test/render';
import { ManifestRefPicker } from '../ManifestRefPicker';

const options = [
  { value: 'main_hub', label: 'Quảng trường' },
  { value: 'math_forest', label: 'Rừng Toán' },
];

describe('ManifestRefPicker (F079-AC-07)', () => {
  it('only offers the manifest values; free text is not accepted', async () => {
    const onChange = vi.fn();
    await renderOps(<ManifestRefPicker ariaLabel="Vùng" options={options} value={undefined} onChange={onChange} />);
    const box = screen.getByRole('combobox', { name: 'Vùng' });
    fireEvent.mouseDown(box);
    fireEvent.change(box, { target: { value: 'khong_co' } });
    fireEvent.keyDown(box, { key: 'Enter', keyCode: 13 });
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.change(box, { target: { value: 'rung' } });
    fireEvent.click(await screen.findByTitle('Rừng Toán'));
    expect(onChange).toHaveBeenCalledWith('math_forest', expect.anything());
  });

  it('flags a stored value missing from the manifest', async () => {
    await renderOps(<ManifestRefPicker ariaLabel="Vùng" options={options} value="old_area" onChange={() => undefined} />);
    expect(screen.getByText(/không có trong danh sách hiện hành/)).toBeInTheDocument();
  });
});
