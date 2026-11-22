// Owner rule 2026-11-22 (ADMIN_PORTAL §1.6): dropdown search ignores case and Vietnamese diacritics; SearchSelect is
// searchable by default, shows an empty state and warns when the list was cut.
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { SearchSelect } from './SearchSelect';
import { foldVi, optionText, viFilterOption } from './viSearch';

describe('viSearch', () => {
  it('folds case, diacritics and đ', () => {
    expect(foldVi('  Trường  THCS Đà Nẵng ')).toBe('truong thcs da nang');
  });
  it('matches every word of the input anywhere in the option text', () => {
    expect(viFilterOption('da nang', { label: 'Sở GD&ĐT Đà Nẵng' })).toBe(true);
    expect(viFilterOption('NANG da', { label: 'Đà Nẵng' })).toBe(true);
    expect(viFilterOption('ha noi', { label: 'Đà Nẵng' })).toBe(false);
    expect(viFilterOption('', { label: 'x' })).toBe(true);
  });
  it('reads searchText, element labels, title and value', () => {
    expect(optionText({ searchText: 'abc', label: 'x' })).toBe('abc');
    expect(optionText({ label: <span>Toán <b>lớp 3</b></span> })).toContain('lớp 3');
    expect(optionText({ value: 'v1' })).toBe('v1');
  });
});

describe('SearchSelect', () => {
  const options = [
    { value: 'a', label: 'Tiếng Việt' },
    { value: 'b', label: 'Toán' },
  ];
  it('filters options client-side without diacritics', async () => {
    render(<SearchSelect aria-label="mon" open options={options} />);
    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'tieng viet' } });
    expect(await screen.findByTitle('Tiếng Việt')).toBeInTheDocument();
    expect(screen.queryByTitle('Toán')).toBeNull();
  });
  it('shows the truncated warning', () => {
    render(<SearchSelect aria-label="mon" open truncated options={options} />);
    expect(screen.getByTestId('select-truncated')).toBeInTheDocument();
  });
});
