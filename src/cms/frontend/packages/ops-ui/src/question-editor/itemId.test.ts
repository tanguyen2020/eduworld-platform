// F053 T018 — F053-AC-02: item ids follow question.schema.json itemId, are random (never A/B/C/D or an index) and
// unique in the version; answer_key must reference existing ids.
import { describe, expect, it } from 'vitest';
import { itemId, ITEM_ID_PATTERN } from './itemId';
import { answerIssues, newQuestionDraft, type OptionItem, type QuestionDraft } from './mappers';

describe('itemId', () => {
  it('pattern ^[a-z]_[a-z0-9_]{1,24}$, prefix by kind, no letters/indexes', () => {
    const ids = Array.from({ length: 200 }, () => itemId('o'));
    for (const id of ids) {
      expect(id).toMatch(ITEM_ID_PATTERN);
      expect(id.startsWith('o_')).toBe(true);
      expect(id).not.toMatch(/^o_[a-d0-9]$/);
    }
    expect(new Set(ids).size).toBeGreaterThan(195);
  });

  it('never returns an id already used in the version', () => {
    let n = 0;
    const seq = [new Uint8Array([0, 0, 0, 1]), new Uint8Array([0, 0, 0, 1]), new Uint8Array([0, 0, 0, 2])];
    const random = () => seq[Math.min(n++, seq.length - 1)]!;
    expect(itemId('b', ['b_0001'], random)).toBe('b_0002');
  });
});

describe('answer_key references (F053-AC-02)', () => {
  it('single_choice: missing answer and an answer pointing to a removed option are both blocking', () => {
    const d = newQuestionDraft('single_choice');
    expect(answerIssues(d)).toContainEqual({ field: 'answer_key.correct_option_id', code: 'ANSWER_MISSING', value: undefined });
    const opts = d.render_payload.options as OptionItem[];
    const ok: QuestionDraft = { ...d, answer_key: { correct_option_id: opts[1]!.id } };
    expect(answerIssues(ok)).toEqual([]);
    const removed: QuestionDraft = { ...ok, render_payload: { ...ok.render_payload, options: opts.filter((_, i) => i !== 1) } };
    expect(answerIssues(removed)).toContainEqual({ field: 'answer_key.correct_option_id', code: 'ANSWER_UNKNOWN_ID', value: opts[1]!.id });
  });

  it('duplicate or malformed ids are reported', () => {
    const d = newQuestionDraft('multiple_choice');
    const opts = d.render_payload.options as OptionItem[];
    const bad: QuestionDraft = { ...d, render_payload: { ...d.render_payload, options: [...opts, { ...opts[0]! }, { id: 'A', content: { blocks: [] } }] }, answer_key: { correct_option_ids: [opts[0]!.id] } };
    const codes = answerIssues(bad).map((i) => i.code);
    expect(codes).toContain('ITEM_ID_DUPLICATE');
    expect(codes).toContain('ITEM_ID_INVALID');
  });
});
