// F053 T055 — F053-AC-32: the worked_example_ref picker offers only lessons sharing an LO with the question and the
// worked_example blocks of the chosen lesson.
import { describe, expect, it } from 'vitest';
import { lessonsForObjectives, workedExampleBlocks } from './WorkedExampleRefPicker';

const lesson = (id: string, los: string[], blocks: { kind: string; content?: unknown }[]) =>
  ({ content_id: id, content_type: 'lesson', version: 1, status: 'published', title: { vi: id }, objective_ids: los, blocks }) as never;

describe('WorkedExampleRefPicker (F053-AC-32)', () => {
  it('keeps lessons with a common LO only; none without LO', () => {
    const list = [lesson('L1', ['lo-a'], []), lesson('L2', ['lo-b'], []), lesson('L3', [], [])];
    expect(lessonsForObjectives(list, ['lo-a', 'lo-c']).map((l: { content_id: string }) => l.content_id)).toEqual(['L1']);
    expect(lessonsForObjectives(list, [])).toEqual([]);
  });

  it('lists the worked_example blocks (by their id) of the lesson', () => {
    const l = lesson('L1', ['lo-a'], [
      { kind: 'rich_text', content: { blocks: [] } },
      { kind: 'worked_example', id: 'we_mul7', steps: [] } as never,
      { kind: 'worked_example', content: { id: 'we_div2', steps: [] } },
    ]);
    expect(workedExampleBlocks(l)).toEqual([
      { id: 'we_mul7', label: '1. we_mul7' },
      { id: 'we_div2', label: '2. we_div2' },
    ]);
    expect(workedExampleBlocks(undefined)).toEqual([]);
  });
});
