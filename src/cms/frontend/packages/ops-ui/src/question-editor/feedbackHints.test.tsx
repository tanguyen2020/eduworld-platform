// F053 T020 — F053-AC-03: misconception tag + bilingual feedback per WRONG option (not for the correct one), up to 5
// hint levels, and the "như học sinh thấy" preview shows no feedback, hint, explanation or answer.
import { fireEvent, screen } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import type { RichContent } from '../rich-content/types';
import { renderOps } from '../test/render';
import { HintsField } from './HintsField';
import { newQuestionDraft, type OptionItem, type QuestionDraft } from './mappers';
import { QuestionEditor } from './QuestionEditor';
import { QuestionPreview } from './QuestionPreview';

const text = (s: string): RichContent => ({ blocks: [{ type: 'paragraph', text: s }] });

function Editor({ initial, onValue }: { initial: QuestionDraft; onValue: (d: QuestionDraft) => void }) {
  const [d, setD] = useState(initial);
  return (
    <QuestionEditor
      value={d}
      contentLanguage="vi"
      onChange={(n) => {
        setD(n);
        onValue(n);
      }}
    />
  );
}

describe('option feedback and hints (F053-AC-03)', () => {
  it('feedback fields only on wrong options; values land in render_payload.options[] (server strips them)', async () => {
    const d = newQuestionDraft('single_choice');
    const opts = d.render_payload.options as OptionItem[];
    const onValue = vi.fn();
    await renderOps(<Editor initial={{ ...d, answer_key: { correct_option_id: opts[0]!.id } }} onValue={onValue} />);
    expect(screen.queryByTestId(`feedback-${opts[0]!.id}`)).toBeNull();
    expect(screen.getByTestId(`feedback-${opts[1]!.id}`)).toBeInTheDocument();
    fireEvent.change(document.getElementById('render_payload.options.1.misconception_tag')!, { target: { value: 'off_by_one' } });
    fireEvent.change(document.getElementById('render_payload.options.1.feedback.vi')!, { target: { value: 'Em đếm lại nhé' } });
    const last = onValue.mock.calls.at(-1)![0] as QuestionDraft;
    expect((last.render_payload.options as OptionItem[])[1]).toMatchObject({ misconception_tag: 'off_by_one', feedback: 'Em đếm lại nhé' });
    expect(document.getElementById('render_payload.options.1.misconception_tag')).toHaveAttribute('maxlength', '64');
    expect(document.getElementById('render_payload.options.1.feedback.vi')).toHaveAttribute('maxlength', '500');
  });

  it('hints: at most 5 levels', async () => {
    const onChange = vi.fn();
    await renderOps(<HintsField value={[1, 2, 3, 4, 5].map((n) => text(`Gợi ý ${n}`))} onChange={onChange} contentLanguage="vi" />);
    expect(screen.getByText('Thêm gợi ý').closest('button')).toBeDisabled();
    fireEvent.click(screen.getAllByText('Xóa gợi ý')[0]!.closest('button')!);
    expect(onChange.mock.calls[0]![0]).toHaveLength(4);
  });

  it('student preview shows stem and options only — no feedback, misconception, hint, explanation or answer', async () => {
    const opts: OptionItem[] = [
      { id: 'o_aa11', content: text('56') },
      { id: 'o_bb22', content: text('54'), feedback: 'PHAN-HOI-BI-MAT', misconception_tag: 'TAG-BI-MAT' },
    ];
    await renderOps(<QuestionPreview type="single_choice" stem={text('7 × 8 = ?')} renderPayload={{ options: opts, shuffle_options: true }} language="vi" contentLanguage="vi" />);
    const preview = screen.getByTestId('student-preview');
    expect(preview).toHaveTextContent('7 × 8 = ?');
    expect(preview).toHaveTextContent('56');
    expect(preview).toHaveTextContent('54');
    expect(preview).not.toHaveTextContent('PHAN-HOI-BI-MAT');
    expect(preview).not.toHaveTextContent('TAG-BI-MAT');
  });
});
