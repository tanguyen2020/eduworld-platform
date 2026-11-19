// F053 T019 — F053-AC-01, AC-02, AC-03, AC-20: for each of the 11 types the editor value is a payload valid against
// specs/schemas/question.schema.json (ajv 2020-12), round-trips unchanged, has no answer keys at the root of
// render_payload, and the student view carries no option feedback / misconception tag; type switch rules; formula +
// typed answer warning.
import Ajv2020 from 'ajv/dist/2020';
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import type { RichContent } from '../../rich-content/types';
import { answerIssues, changeType, expressionInputWarning, needsTypeChangeConfirm, newQuestionDraft, QUESTION_TYPES, toStudentView, type OptionItem, type QuestionDraft, type QuestionType } from './index';

const REPO = path.resolve(__dirname, '../../../../../../../..');
const read = (p: string) => JSON.parse(fs.readFileSync(path.join(REPO, p), 'utf8')) as object;
const ajv = new Ajv2020({ strict: false, allErrors: true });
ajv.addFormat('uuid', /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i);
ajv.addFormat('date-time', true);
ajv.addSchema(read('specs/schemas/common/enums.schema.json'));
const validate = ajv.compile(read('specs/schemas/question.schema.json'));

const U = (n: number) => `0193a4d2-0000-7000-8000-${String(n).padStart(12, '0')}`;
const text = (s: string): RichContent => ({ blocks: [{ type: 'paragraph', text: s }] });

/** A draft the author completed (answers chosen, texts typed). */
function filled(type: QuestionType): QuestionDraft {
  const d = newQuestionDraft(type);
  d.stem = text('Đề bài mẫu');
  const rp = d.render_payload;
  const fill = (list: unknown) => (list as OptionItem[]).map((o, i) => ({ ...o, content: text(`Mục ${i + 1}`) }));
  switch (type) {
    case 'single_choice': {
      const opts = fill(rp.options);
      opts[1] = { ...opts[1]!, feedback: { vi: 'Chưa đúng', en: 'Not quite' }, misconception_tag: 'off_by_one' };
      return { ...d, render_payload: { ...rp, options: opts }, answer_key: { correct_option_id: opts[0]!.id }, hints: [text('Gợi ý 1')], explanation: text('Vì…') };
    }
    case 'multiple_choice': {
      const opts = fill(rp.options);
      return { ...d, render_payload: { ...rp, options: opts }, answer_key: { correct_option_ids: [opts[0]!.id, opts[2]!.id] } };
    }
    case 'true_false':
      return { ...d, answer_key: { value: false } };
    case 'fill_in_blank': {
      const b = (rp.blanks as { id: string }[])[0]!.id;
      return { ...d, stem: text(`Thủ đô của Việt Nam là {{blank:${b}}}.`), answer_key: { blanks: { [b]: { accepted: ['Hà Nội'], match: 'normalized' } } } };
    }
    case 'short_answer':
      return { ...d, answer_key: { accepted: ['mèo'], match: 'normalized' } };
    case 'numeric':
      return { ...d, answer_key: { value: 12.5, tolerance: { type: 'absolute', value: 0.01 }, accept_equivalent_forms: true } };
    case 'matching': {
      const left = fill(rp.left);
      const right = fill(rp.right);
      return { ...d, render_payload: { ...rp, left, right }, answer_key: { pairs: [[left[0]!.id, right[1]!.id], [left[1]!.id, right[0]!.id]] } };
    }
    case 'ordering': {
      const items = fill(rp.items);
      return { ...d, render_payload: { ...rp, items }, answer_key: { order: items.map((x) => x.id) } };
    }
    case 'drag_drop': {
      const draggables = fill(rp.draggables);
      const zones = (rp.zones as { id: string }[]).map((z) => ({ ...z, label: 'Số chẵn' }));
      return { ...d, render_payload: { ...rp, draggables, zones }, answer_key: { placements: { [draggables[0]!.id]: zones[0]!.id } } };
    }
    case 'image_based':
      return { ...d, render_payload: { ...rp, image_media_ref: U(9) }, answer_key: { regions: [{ id: 'h_heart', shape: 'circle', cx: 512, cy: 300, r: 60 }], correct_region_ids: ['h_heart'] } };
    case 'interactive':
      return { ...d, render_payload: { widget: 'balance_scale', widget_version: '1.0', config: { max_moves: 6 } }, answer_key: { validator: 'balance_scale.equal', params: { target: 8 } } };
  }
}

const version = (d: QuestionDraft) => ({
  question_id: U(1),
  version: 1,
  ...d,
  max_score: 1,
  meta: { grade_levels: [3], subject_id: U(2), topic_id: U(3), objective_ids: [U(4)], curriculum_version_id: U(5), difficulty: 2, status: 'draft', author_id: U(6), owner_scope: 'platform', content_language: 'vi' },
});

const FORBIDDEN_ROOT = ['answer_key', 'scoring_rule', 'accepted_answers', 'correct_option_id', 'correct_option_ids', 'explanation', 'feedback', 'hints', 'min_response_ms'];

describe('mappers — 11 types (F053-AC-01)', () => {
  it.each(QUESTION_TYPES)('%s: completed draft has no issue and validates against question.schema.json', (type) => {
    const d = filled(type);
    expect(answerIssues(d)).toEqual([]);
    const ok = validate(version(d));
    expect(validate.errors ?? [], type).toEqual([]);
    expect(ok).toBe(true);
    // the form value round-trips unchanged (draft PATCH → GET → form)
    expect(JSON.parse(JSON.stringify(d))).toEqual(d);
    for (const k of FORBIDDEN_ROOT) expect(Object.keys(d.render_payload)).not.toContain(k);
  });

  it.each(QUESTION_TYPES)('%s: new empty draft reports what is missing (save blocked, F053-AC-02)', (type) => {
    if (type === 'ordering') return; // the order is the authoring order: always answered
    expect(answerIssues(newQuestionDraft(type)).length).toBeGreaterThan(0);
  });

  it('student view never contains option feedback or misconception tags (F053-AC-03)', () => {
    const d = filled('single_choice');
    expect(JSON.stringify(d.render_payload)).toContain('off_by_one');
    const view = JSON.stringify(toStudentView(d));
    expect(view).not.toContain('off_by_one');
    expect(view).not.toContain('Chưa đúng');
    expect(view).not.toContain('correct_option_id');
  });

  it('type switch: single ↔ multiple keeps options and converts the answer; other switches drop it after confirmation', () => {
    const d = filled('single_choice');
    expect(needsTypeChangeConfirm(d, 'multiple_choice')).toBe(false);
    const m = changeType(d, 'multiple_choice');
    expect(m.answer_key).toEqual({ correct_option_ids: [d.answer_key.correct_option_id] });
    expect(m.render_payload.options).toEqual(d.render_payload.options);
    expect(needsTypeChangeConfirm(d, 'matching')).toBe(true);
    const x = changeType(d, 'matching');
    expect(x.answer_key).toEqual({ pairs: [] });
    expect(x.stem).toEqual(d.stem);
    expect(needsTypeChangeConfirm(newQuestionDraft('single_choice'), 'numeric')).toBe(false);
  });

  it('F053-AC-20: formula in a typed-answer question → warning; same formula in single choice → none', () => {
    const withMath = (t: QuestionType) => ({ ...filled(t), stem: text(String.raw`Tính \(\frac{3}{4} + \frac{1}{4}\)`) });
    expect(expressionInputWarning(withMath('short_answer'))).toBe(true);
    expect(expressionInputWarning(withMath('fill_in_blank'))).toBe(true);
    expect(expressionInputWarning(withMath('single_choice'))).toBe(false);
    expect(expressionInputWarning(withMath('numeric'))).toBe(false);
    expect(expressionInputWarning(filled('short_answer'))).toBe(false);
  });
});
