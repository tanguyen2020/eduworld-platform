// F053 T054 — F053-AC-31: worked_example block with a problem, 1–8 steps (rich content + audio_ref, required for
// grades 1–3), optional self-explanation prompt and an id unique in the version.
import { fireEvent, screen } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderOps } from '../test/render';
import type { WorkedExample } from './types';
import { newWorkedExample, newWorkedExampleId, validateWorkedExample, WORKED_EXAMPLE_ID, WorkedExampleBlock } from './WorkedExampleBlock';

const p = (s: string) => ({ blocks: [{ type: 'paragraph' as const, text: s }] });

describe('validateWorkedExample', () => {
  it('ids: pattern we_*, unique among the other blocks of the version', () => {
    const id = newWorkedExampleId(['we_000001']);
    expect(id).toMatch(WORKED_EXAMPLE_ID);
    expect(newWorkedExampleId(['we_000000'], () => 0)).not.toBe('we_000000');
    const we: WorkedExample = { ...newWorkedExample(), id: 'we_mul7', problem: p('7 × 6 = ?'), steps: [{ content: p('7 × 5 = 35') }] };
    expect(validateWorkedExample(we, { requireAudio: false, otherIds: ['we_mul7'] })).toEqual([{ issue: 'ID_DUPLICATE' }]);
    expect(validateWorkedExample({ ...we, id: 'Bad Id' }, { requireAudio: false })).toEqual([{ issue: 'ID_INVALID' }]);
  });

  it('1–8 steps; grades 1–3 need audio on every step', () => {
    const base: WorkedExample = { ...newWorkedExample(), problem: p('7 × 6 = ?') };
    const steps = (n: number) => Array.from({ length: n }, (_, i) => ({ content: p(`Bước ${i}`), audio_ref: null }));
    expect(validateWorkedExample({ ...base, steps: [] }, { requireAudio: false })).toEqual([{ issue: 'STEPS_MIN' }]);
    expect(validateWorkedExample({ ...base, steps: steps(9) }, { requireAudio: false })).toEqual([{ issue: 'STEPS_MAX' }]);
    expect(validateWorkedExample({ ...base, steps: steps(3) }, { requireAudio: false })).toEqual([]);
    expect(validateWorkedExample({ ...base, steps: steps(2) }, { requireAudio: true })).toEqual([
      { issue: 'STEP_AUDIO_REQUIRED', step: 0 },
      { issue: 'STEP_AUDIO_REQUIRED', step: 1 },
    ]);
  });
});

function Harness({ onValue, requireAudio, steps = 1 }: { onValue: (v: WorkedExample) => void; requireAudio: boolean; steps?: number }) {
  const [v, setV] = useState<WorkedExample>({ ...newWorkedExample(), id: 'we_test01', problem: p('7 × 6 = ?'), steps: Array.from({ length: steps }, (_, i) => ({ content: p(`Bước ${i + 1}`), audio_ref: null })) });
  return (
    <WorkedExampleBlock
      value={v}
      contentLanguage="vi"
      requireAudio={requireAudio}
      onChange={(n) => {
        setV(n);
        onValue(n);
      }}
      mediaSlot={({ kind, id, onChange }) => (
        <button type="button" onClick={() => onChange('0193a4d2-0000-7000-8000-000000000077')} data-testid={id}>
          {kind}
        </button>
      )}
    />
  );
}

describe('WorkedExampleBlock', () => {
  it('adds a step (limit 8 checked by validateWorkedExample); audio of grades 1–3 is reported until set', async () => {
    const onValue = vi.fn();
    await renderOps(<Harness onValue={onValue} requireAudio steps={1} />);
    expect(screen.getByText('Bước 1 thiếu âm thanh đọc.')).toBeInTheDocument();
    const add = () => screen.getByText('Thêm bước (tối đa 8)').closest('button')!;
    expect(add()).toBeEnabled();
    fireEvent.click(add());
    expect((onValue.mock.calls.at(-1)![0] as WorkedExample).steps).toHaveLength(2);
    expect(screen.getByText('Bước 2 thiếu âm thanh đọc.')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('worked_example.we_test01.steps.0.audio_ref'));
    expect(screen.queryByText('Bước 1 thiếu âm thanh đọc.')).toBeNull();
    expect((onValue.mock.calls.at(-1)![0] as WorkedExample).steps[0]!.audio_ref).toBe('0193a4d2-0000-7000-8000-000000000077');
  });
});
