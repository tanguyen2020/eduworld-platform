// F053 T014 — F053-AC-14 (TC-TEN-024): ajv with the generated copy of question.schema.json maps errors to the
// editor field paths (like 422 VALIDATION_FAILED errors[].field) and enforces maxLength / maxItems; the copy stays
// equal to specs/schemas (gen:schemas --check).
import { newQuestionDraft, type OptionItem } from '@eduworld/ops-ui/content';
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { expected } from '../../../../scripts/gen-schemas.mjs';
import { schemaErrorMessage, validateQuestionDraft } from './schemaValidator';

describe('schemaValidator (F053-AC-14)', () => {
  it('generated schema copies are identical to specs/schemas', () => {
    for (const { name, content } of expected()) {
      const copy = fs.readFileSync(path.resolve(__dirname, '../../../shared/api/schemas', name), 'utf8').replace(/\r\n/g, '\n');
      expect(copy, name).toBe(content);
    }
  });

  it('a fresh draft only lacks its answer (correct_option_id required)', () => {
    expect(validateQuestionDraft(newQuestionDraft('single_choice'))).toEqual([{ field: 'answer_key.correct_option_id', keyword: 'required', params: { missingProperty: 'correct_option_id' } }]);
  });

  it('too long option text → error on render_payload.options.1.content.blocks.0.text with maxLength 1000', () => {
    const d = newQuestionDraft('single_choice');
    const opts = d.render_payload.options as OptionItem[];
    opts[1] = { ...opts[1]!, content: { blocks: [{ type: 'paragraph', text: 'x'.repeat(1001) }] } };
    const errs = validateQuestionDraft(d);
    const e = errs.find((x) => x.field.startsWith('render_payload.options.1.content.blocks.0.text'));
    expect(e).toBeDefined();
    expect(schemaErrorMessage({ field: 'f', keyword: 'maxLength', params: { limit: 1000 } }, (k, o) => `${k}:${String(o?.limit)}`)).toBe('content_studio.validation.maxLength:1000');
  });

  it('feedback over 500 characters and misconception_tag over 64 are reported on their fields', () => {
    const d = newQuestionDraft('single_choice');
    const opts = d.render_payload.options as OptionItem[];
    opts[0] = { ...opts[0]!, feedback: 'y'.repeat(501), misconception_tag: 'z'.repeat(65) };
    const fields = validateQuestionDraft(d).map((e) => e.field);
    expect(fields.some((f) => f.startsWith('render_payload.options.0.feedback'))).toBe(true);
    expect(fields).toContain('render_payload.options.0.misconception_tag');
  });

  it('image block without alt → required error on stem.blocks.0.alt; more than 5 hints → hints', () => {
    const d = newQuestionDraft('single_choice');
    d.stem = { blocks: [{ type: 'image', media_ref: '0193a4d2-0000-7000-8000-000000000001' }] };
    d.hints = Array.from({ length: 6 }, () => ({ blocks: [{ type: 'paragraph' as const, text: 'h' }] }));
    const fields = validateQuestionDraft(d).map((e) => e.field);
    expect(fields).toContain('stem.blocks.0.alt');
    expect(fields).toContain('hints');
  });
});
