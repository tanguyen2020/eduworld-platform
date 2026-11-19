// F053 T039 — F053-AC-21 (two columns vi/en, tabs on narrow screens, content_language first labelled "Gốc"),
// F053-AC-22 (badges Đủ / Thiếu bản dịch / Thiếu bản gốc / Lỗi thời per field from translation_status + outdated_fields).
import { fireEvent, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { renderOps } from '../test/render';
import { BilingualField } from './BilingualField';
import { fieldStatus, fromLangMap, toLangMap } from './translationStatus';

describe('translationStatus', () => {
  it('plain string = content_language; object = with translation', () => {
    expect(toLangMap('Xin chào', 'vi')).toEqual({ vi: 'Xin chào' });
    expect(toLangMap({ vi: 'A', en: 'B' }, 'vi')).toEqual({ vi: 'A', en: 'B' });
    expect(fromLangMap({ vi: 'A', en: '' }, 'vi')).toBe('A');
    expect(fromLangMap({ vi: 'A', en: 'B' }, 'vi')).toEqual({ vi: 'A', en: 'B' });
    expect(fromLangMap({ vi: '', en: 'B' }, 'vi')).toEqual({ en: 'B' });
    expect(fromLangMap({}, 'vi')).toBeUndefined();
  });

  it('field status', () => {
    expect(fieldStatus({ vi: 'A', en: 'B' }, 'vi')).toBe('complete');
    expect(fieldStatus('A', 'vi')).toBe('partial');
    expect(fieldStatus({ en: 'B' }, 'vi')).toBe('missing_original');
    expect(fieldStatus({ vi: 'A', en: 'B' }, 'vi', true)).toBe('outdated');
    expect(fieldStatus(undefined, 'vi')).toBe('empty');
  });
});

describe('BilingualField', () => {
  it('columns: original first with "Gốc" (content_language = en puts English first)', async () => {
    await renderOps(<BilingualField id="stem" label="Đề bài" value={{ vi: 'Con mèo', en: 'The cat' }} onChange={() => {}} contentLanguage="en" maxLength={1000} layout="columns" />);
    const inputs = screen.getAllByRole('textbox');
    expect(inputs.map((i) => i.getAttribute('aria-label'))).toEqual(['Đề bài (Tiếng Anh)', 'Đề bài (Tiếng Việt)']);
    expect(screen.getByText('Gốc')).toBeInTheDocument();
    expect(screen.getByText('Đủ')).toBeInTheDocument();
  });

  it('narrow screens: tabs VI | EN', async () => {
    await renderOps(<BilingualField id="stem" label="Đề bài" value="Con mèo" onChange={() => {}} contentLanguage="vi" maxLength={1000} layout="tabs" />);
    const tabs = screen.getAllByRole('tab');
    expect(tabs.map((t) => t.textContent)).toEqual(['VI', 'EN']);
    expect(screen.getByText('Thiếu bản dịch')).toBeInTheDocument();
  });

  it('badges: Lỗi thời from outdated_fields, Thiếu bản gốc when the original is empty', async () => {
    const { rerender } = await renderOps(<BilingualField id="a" label="Giải thích" value={{ vi: 'A', en: 'B' }} outdated onChange={() => {}} contentLanguage="vi" maxLength={1000} layout="columns" />);
    expect(screen.getByText('Lỗi thời')).toBeInTheDocument();
    rerender(<BilingualField id="a" label="Giải thích" value={{ en: 'B' }} required onChange={() => {}} contentLanguage="vi" maxLength={1000} layout="columns" />);
    expect(screen.getByText('Thiếu bản gốc')).toBeInTheDocument();
    expect(screen.getByText('Bắt buộc nhập bản theo ngôn ngữ gốc.')).toBeInTheDocument();
  });

  it('typing stores a plain string while only the original exists, an object once translated; maxLength caps input', async () => {
    const onChange = vi.fn();
    await renderOps(<BilingualField id="s" label="Đề" value="Một" onChange={onChange} contentLanguage="vi" maxLength={10} layout="columns" />);
    fireEvent.change(screen.getByRole('textbox', { name: 'Đề (Tiếng Anh)' }), { target: { value: 'One' } });
    expect(onChange).toHaveBeenLastCalledWith({ vi: 'Một', en: 'One' });
    expect(screen.getByRole('textbox', { name: 'Đề (Tiếng Việt)' })).toHaveAttribute('maxlength', '10');
    const counter = within(document.querySelector('[data-field="s"]') as HTMLElement).getAllByText('3/10');
    expect(counter.length).toBeGreaterThan(0);
  });
});
