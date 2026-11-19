// F053 T034 — F053-AC-16 (Toán / Lý / Hóa tabs, inline \( \) / block \[ \]), F053-AC-17/18 (red field on a local
// error or a server MATH_RENDER_FAILED), F053-AC-20 (warning for typed-answer types with formulas, QuestionEditor).
// MathLive itself is a MOCK module in jsdom (src/test/setup.ts).
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { newQuestionDraft } from '../question-editor/mappers';
import { QuestionEditor } from '../question-editor/QuestionEditor';
import { renderOps } from '../test/render';
import { splitMath, wrapLatex } from './delimiters';
import { keyboardLayouts, KEYBOARD_TABS } from './keyboardLayouts';
import { MathInput } from './MathInput';
import { MathInsertButton } from './MathInsertButton';

describe('keyboard and delimiters (F053-AC-16)', () => {
  it('three tabs Toán / Lý / Hóa with fraction, root, power, \\ce and \\pu keys', () => {
    const t = (k: string) => ({ 'math.keyboard.math': 'Toán', 'math.keyboard.physics': 'Lý', 'math.keyboard.chemistry': 'Hóa' })[k] ?? k;
    const layouts = keyboardLayouts(t);
    expect(KEYBOARD_TABS).toEqual(['math', 'physics', 'chemistry']);
    expect(layouts.map((l) => l.label)).toEqual(['Toán', 'Lý', 'Hóa']);
    const all = JSON.stringify(layouts);
    for (const key of ['\\\\frac{#@}{#?}', '\\\\sqrt{#0}', '#@^{#?}', '#@_{#?}', 'pmatrix', '\\\\ce{#0}', '\\\\pu{#0 m/s}']) expect(all).toContain(key);
  });

  it('inline \\( \\) and block \\[ \\] delimiters round-trip', () => {
    expect(wrapLatex('\\frac{3}{4}', false)).toBe('\\(\\frac{3}{4}\\)');
    expect(wrapLatex('x^2', true)).toBe('\\[x^2\\]');
    expect(splitMath('Tính \\(\\frac{3}{4}\\) và \\[x^2\\] nhé')).toEqual([
      { kind: 'text', value: 'Tính ' },
      { kind: 'math', latex: '\\frac{3}{4}', display: false, start: 5, end: 20 },
      { kind: 'text', value: ' và ' },
      { kind: 'math', latex: 'x^2', display: true, start: 24, end: 31 },
      { kind: 'text', value: ' nhé' },
    ]);
  });
});

function Controlled({ initial, error }: { initial: string; error?: string }) {
  const [v, setV] = useState(initial);
  const [d, setD] = useState(false);
  return <MathInput label="Công thức" value={v} onChange={setV} display={d} onDisplayChange={setD} error={error} />;
}

describe('MathInput', () => {
  it('mounts <math-field> (MathLive) and the LaTeX source; preview rendered by MathJax', async () => {
    await renderOps(<Controlled initial="\frac{3}{4}" />);
    await waitFor(() => expect(document.querySelector('math-field')).not.toBeNull());
    expect((document.querySelector('math-field') as unknown as { value: string }).value).toBe('\\frac{3}{4}');
    expect(await screen.findByRole('img', { name: 'phân số 3 trên 4' })).toBeInTheDocument();
  });

  it('F053-AC-17: a forbidden command / syntax error turns the field red with the message', async () => {
    await renderOps(<Controlled initial="\href{x}{y}" />);
    expect(await screen.findByText('Lệnh \\href không được phép trong công thức.')).toBeInTheDocument();
    expect(screen.getByLabelText('Mã LaTeX của Công thức')).toHaveAttribute('aria-invalid', 'true');
    fireEvent.change(screen.getByLabelText('Mã LaTeX của Công thức'), { target: { value: '\\frac{1' } });
    expect(await screen.findByText('Công thức sai cú pháp.')).toBeInTheDocument();
  });

  it('F053-AC-18: server MATH_RENDER_FAILED message for this formula is shown and the field is red', async () => {
    await renderOps(<Controlled initial="x^2" error="Máy chủ từ chối công thức này" />);
    expect(screen.getByText('Máy chủ từ chối công thức này')).toBeInTheDocument();
    expect(screen.getByLabelText('Mã LaTeX của Công thức')).toHaveAttribute('aria-invalid', 'true');
  });

  it('insert button: OK disabled until the formula is valid, then inserts the delimited LaTeX', async () => {
    const onInsert = vi.fn();
    await renderOps(<MathInsertButton onInsert={onInsert} />);
    fireEvent.click(screen.getByText('∑ Công thức').closest('button')!);
    const ok = (await screen.findByText('Chèn')).closest('button')!;
    expect(ok).toBeDisabled();
    fireEvent.change(screen.getByLabelText('Mã LaTeX của Công thức'), { target: { value: '\\sqrt{2}' } });
    fireEvent.click(screen.getByText('Riêng một dòng'));
    await waitFor(() => expect(screen.getByText('Chèn').closest('button')!).toBeEnabled());
    fireEvent.click(screen.getByText('Chèn').closest('button')!);
    expect(onInsert).toHaveBeenCalledWith('\\[\\sqrt{2}\\]');
  });
});

describe('F053-AC-20 in the question editor', () => {
  it('short answer with a formula in the stem shows the ADR-0055 warning', async () => {
    const d = newQuestionDraft('short_answer');
    await renderOps(<QuestionEditor value={{ ...d, stem: { blocks: [{ type: 'paragraph', text: 'Rút gọn \\(\\frac{2}{4}\\)' }] } }} onChange={() => {}} contentLanguage="vi" />);
    expect(screen.getByTestId('expression-warning')).toBeInTheDocument();
  });

  it('single choice with the same formula: no warning', async () => {
    const d = newQuestionDraft('single_choice');
    await renderOps(<QuestionEditor value={{ ...d, stem: { blocks: [{ type: 'paragraph', text: 'Rút gọn \\(\\frac{2}{4}\\)' }] } }} onChange={() => {}} contentLanguage="vi" />);
    expect(screen.queryByTestId('expression-warning')).toBeNull();
  });
});
