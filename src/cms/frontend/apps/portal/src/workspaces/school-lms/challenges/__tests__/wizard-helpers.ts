// Test helpers (test only) driving the challenge wizard and Ant Design selects / date pickers in jsdom.
import { expect } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';

/** Opens a select and clicks the option of its dropdown. */
export async function pickOption(label: string, option: string) {
  fireEvent.mouseDown(await screen.findByRole('combobox', { name: label }));
  const options = () => screen.queryAllByTitle(option).filter((e) => e.classList.contains('ant-select-item-option'));
  await waitFor(() => expect(options().length).toBeGreaterThan(0));
  fireEvent.click(options().at(-1)!);
}

export function typeDate(label: string, value: string) {
  const input = screen.getByRole('textbox', { name: label }) as HTMLInputElement;
  fireEvent.mouseDown(input);
  fireEvent.change(input, { target: { value } });
  fireEvent.keyDown(input, { key: 'Enter', code: 'Enter' });
  fireEvent.blur(input);
}

export const next = () => fireEvent.click(screen.getByRole('button', { name: 'Tiếp' }));

/** Picks the grade of a student picker, searches (≥ 2 characters) and picks the option. */
export async function pickStudent(label: string, grade: number, search: string, option: string) {
  await pickOption(`${label} — khối`, `Khối ${grade}`);
  const box = screen.getByRole('combobox', { name: label });
  fireEvent.mouseDown(box);
  fireEvent.change(box, { target: { value: search } });
  await waitFor(() => expect(screen.getAllByTitle(option).some((e) => e.classList.contains('ant-select-item-option'))).toBe(true));
  fireEvent.click(screen.getAllByTitle(option).filter((e) => e.classList.contains('ant-select-item-option')).at(-1)!);
}

/** The only school of the scope is picked automatically. */
export async function waitSchool(name = 'TH Demo A') {
  await waitFor(() => expect(within(screen.getByRole('combobox', { name: 'Trường' }).closest('.ant-select')!).getByText(name)).toBeInTheDocument(), { timeout: 120_000 });
}

/** Steps template → details → targets with valid data (tournament template, grade 3, + Cường, − Bình); ends on review. */
export async function fillWizard(opts: { include?: boolean; exclude?: boolean } = {}) {
  await pickOption('Template', 'Giải Toán khối');
  next();
  fireEvent.change(await screen.findByLabelText('Tên thử thách'), { target: { value: 'Giải Toán khối 3' } });
  typeDate('Bắt đầu lúc', '2026-12-01 08:00:00');
  fireEvent.change(screen.getByLabelText('Số người tham gia tối đa'), { target: { value: '32' } });
  next();
  await waitSchool();
  await pickOption('Khối', 'Khối 3');
  if (opts.include !== false) await pickStudent('Thêm học sinh (cùng trường)', 4, 'Cường', 'Phạm Cường (hs.cuong.4b)');
  if (opts.exclude !== false) await pickStudent('Loại trừ học sinh', 3, 'Bình', 'Lê Bình (hs.binh.3a)');
  next();
  await screen.findByTestId('challenge-review');
}
