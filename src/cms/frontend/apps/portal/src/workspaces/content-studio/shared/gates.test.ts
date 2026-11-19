// F053 T021 (audioGate) — F053-AC-04 (TC-CNT-010): grades 1–3 need stem.audio_ref unless audio_exempt_reason; a
// server 422 AUDIO_REQUIRED lists the missing fields. F053 T040 (translationGate) — F053-AC-23, AC-24 (TC-I18N-002):
// missing original text blocks, TRANSLATION_INCOMPLETE maps to fields, a missing translation only warns (partial).
// Media pipeline gate (F053-AC-05 edge case).
import { newQuestionDraft, type OptionItem, type QuestionDraft } from '@eduworld/ops-ui/content';
import { describe, expect, it } from 'vitest';
import { toApiError } from '../../../shared/api/middleware/problem';
import { audioGate, mediaGate, mediaRefs, originalMissing, pointerToField, serverGates, translationPartial } from './gates';

const has = () => true;
const filled = (): QuestionDraft => {
  const d = newQuestionDraft('single_choice');
  d.stem = { blocks: [{ type: 'paragraph', text: 'Con mèo có mấy chân?' }] };
  d.render_payload = { ...d.render_payload, options: (d.render_payload.options as OptionItem[]).map((o, i) => ({ ...o, content: { blocks: [{ type: 'paragraph', text: String(i + 2) }] } })) };
  return d;
};

describe('audio gate (F053-AC-04)', () => {
  it('grade 3 without stem audio → blocker on stem.audio_ref; grade 4 → none', () => {
    expect(audioGate(filled(), [3, 4], undefined)).toEqual([{ field: 'stem.audio_ref', key: 'content_studio.gate.audioRequired' }]);
    expect(audioGate(filled(), [4, 5], undefined)).toEqual([]);
  });

  it('audio present or exemption reason → open', () => {
    const d = filled();
    d.stem = { ...d.stem, audio_ref: '0193a4d2-0000-7000-8000-000000000009' };
    expect(audioGate(d, [1], undefined)).toEqual([]);
    expect(audioGate(filled(), [2], 'Câu chính tả: đọc to làm lộ đáp án')).toEqual([]);
    expect(audioGate(filled(), [2], '   ')).toHaveLength(1);
  });

  it('server 422 AUDIO_REQUIRED (JSON pointer) → gate on the same field', () => {
    const err = toApiError(422, { code: 'AUDIO_REQUIRED', errors: [{ field: '/stem/audio_ref', code: 'REQUIRED' }] }, has);
    expect(serverGates(err)).toEqual([{ field: 'stem.audio_ref', key: 'content_studio.problem.AUDIO_REQUIRED', params: { code: 'REQUIRED' } }]);
    expect(pointerToField('/render_payload/options/1/content')).toBe('render_payload.options.1.content');
  });
});

describe('translation gate (F053-AC-23, AC-24)', () => {
  it('original vi present, en missing → no blocker, partial warning only', () => {
    expect(originalMissing(filled(), 'vi')).toEqual([]);
    expect(translationPartial(filled(), 'vi')).toBe(true);
  });

  it('original missing (only en typed in a vi question) → blockers listing the fields', () => {
    const d = filled();
    d.stem = { blocks: [{ type: 'paragraph', text: { en: 'How many legs?' } }] };
    const g = originalMissing(d, 'vi');
    expect(g.map((x) => x.field)).toEqual(['stem', 'stem.blocks.0.text']);
  });

  it('both languages → complete, no warning; TRANSLATION_INCOMPLETE maps to fields', () => {
    const d = filled();
    d.stem = { blocks: [{ type: 'paragraph', text: { vi: 'Mèo?', en: 'Cat?' } }] };
    d.render_payload = { ...d.render_payload, options: (d.render_payload.options as OptionItem[]).map((o) => ({ ...o, content: { blocks: [{ type: 'paragraph', text: { vi: '4', en: '4' } }] } })) };
    expect(translationPartial(d, 'vi')).toBe(false);
    const err = toApiError(422, { code: 'TRANSLATION_INCOMPLETE', errors: [{ field: '/stem/blocks/0/text', code: 'REQUIRED' }] }, has);
    expect(serverGates(err)[0]!.field).toBe('stem.blocks.0.text');
  });
});

describe('media gate (F053-AC-05)', () => {
  it('assets not ready (or rejected) block submit', () => {
    const d = filled();
    d.stem = { blocks: [{ type: 'image', media_ref: 'A', alt: 'x' }], audio_ref: 'B' };
    expect(mediaRefs(d).sort()).toEqual(['A', 'B']);
    expect(mediaGate(d, { A: 'processing', B: 'ready' })).toEqual([{ field: 'A', key: 'content_studio.gate.mediaNotReady' }]);
    expect(mediaGate(d, { A: 'rejected' })).toEqual([{ field: 'A', key: 'content_studio.gate.mediaRejected' }]);
    expect(mediaGate(d, { A: 'ready', B: 'ready' })).toEqual([]);
  });
});
