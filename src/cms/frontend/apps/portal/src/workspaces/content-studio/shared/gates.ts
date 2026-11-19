// Submit / publish gates of the question editor (pure; the server decides again):
// - F053-AC-04 (CONTENT_MODEL §8.1, TC-CNT-010): grade_levels ∩ {1,2,3} → stem.audio_ref required unless an
//   audio_exempt_reason is given;
// - F053-AC-23 / AC-24 (CONTENT_MODEL §8, ADR-0056): the content_language text of required fields must exist (blocks
//   Gửi duyệt / Xuất bản); a missing translation only warns (translation_status = partial);
// - F053-AC-05 edge case: media still in the pipeline blocks submit (422 MEDIA_NOT_READY otherwise);
// - server Problems with `errors[].field` (AUDIO_REQUIRED, TRANSLATION_INCOMPLETE, MEDIA_NOT_READY,
//   MATH_RENDER_FAILED, VALIDATION_FAILED) → blockers on the same fields.
import { fieldStatus, type ContentLang, type I18nTextValue, type OptionItem, type QuestionDraft, type RichContent } from '@eduworld/ops-ui/content';
import type { ApiError } from '../../../shared/api/middleware/problem';
import { SPOKEN_STEM_GRADES } from './config';

export interface Gate {
  field: string;
  /** i18n key under content_studio.* (with params). */
  key: string;
  params?: Record<string, unknown>;
}

export function needsSpokenStem(gradeLevels: readonly number[] | undefined): boolean {
  return (gradeLevels ?? []).some((g) => SPOKEN_STEM_GRADES.includes(g));
}

/** F053-AC-04. */
export function audioGate(d: Pick<QuestionDraft, 'stem'>, gradeLevels: readonly number[] | undefined, exemptReason: string | undefined): Gate[] {
  if (!needsSpokenStem(gradeLevels) || d.stem.audio_ref || exemptReason?.trim()) return [];
  return [{ field: 'stem.audio_ref', key: 'content_studio.gate.audioRequired' }];
}

const blockHasOriginal = (c: RichContent | undefined, lang: ContentLang) =>
  !!c?.blocks.some((b) => b.type === 'math' || b.type === 'image' || b.type === 'audio' || b.type === 'video' || b.type === 'table' || (b.text !== undefined && fieldStatus(b.text, lang) !== 'missing_original' && fieldStatus(b.text, lang) !== 'empty'));

const OPTION_LISTS = ['options', 'statements', 'left', 'right', 'items', 'draggables', 'labels'] as const;

/** Every bilingual text of a draft with its field path. */
export function i18nTexts(d: QuestionDraft): { field: string; value: I18nTextValue }[] {
  const out: { field: string; value: I18nTextValue }[] = [];
  const rich = (c: RichContent | undefined, path: string) =>
    c?.blocks.forEach((b, i) => {
      if (b.text !== undefined) out.push({ field: `${path}.blocks.${i}.text`, value: b.text });
      b.items?.forEach((it, k) => out.push({ field: `${path}.blocks.${i}.items.${k}`, value: it }));
    });
  rich(d.stem, 'stem');
  rich(d.explanation, 'explanation');
  d.hints?.forEach((h, i) => rich(h, `hints.${i}`));
  for (const k of OPTION_LISTS) {
    const list = d.render_payload[k];
    if (!Array.isArray(list)) continue;
    (list as OptionItem[]).forEach((o, i) => {
      rich(o.content, `render_payload.${k}.${i}.content`);
      if (o.feedback !== undefined) out.push({ field: `render_payload.${k}.${i}.feedback`, value: o.feedback });
    });
  }
  return out;
}

/** F053-AC-23: required fields without their content_language text. */
export function originalMissing(d: QuestionDraft, lang: ContentLang): Gate[] {
  const out: Gate[] = [];
  if (!blockHasOriginal(d.stem, lang)) out.push({ field: 'stem', key: 'content_studio.gate.originalMissing', params: { field: 'stem' } });
  for (const k of OPTION_LISTS) {
    const list = d.render_payload[k];
    if (!Array.isArray(list)) continue;
    (list as OptionItem[]).forEach((o, i) => {
      if (!blockHasOriginal(o.content, lang)) out.push({ field: `render_payload.${k}.${i}.content`, key: 'content_studio.gate.originalMissingItem', params: { n: i + 1 } });
    });
  }
  for (const x of i18nTexts(d)) if (fieldStatus(x.value, lang) === 'missing_original') out.push({ field: x.field, key: 'content_studio.gate.originalMissingText' });
  return out;
}

/** F053-AC-24: some text has no translation → warn only (publish allowed with translation_status = partial). */
export function translationPartial(d: QuestionDraft, lang: ContentLang): boolean {
  return i18nTexts(d).some((x) => fieldStatus(x.value, lang) === 'partial');
}

/** Ids of every media asset referenced by a draft (stem audio, image/audio blocks, background, image_based). */
export function mediaRefs(d: QuestionDraft): string[] {
  const ids = new Set<string>();
  const walk = (n: unknown, key = '') => {
    if (typeof n === 'string') {
      if (['media_ref', 'audio_ref', 'image_media_ref', 'background_media_ref'].includes(key)) ids.add(n);
      return;
    }
    if (Array.isArray(n)) n.forEach((x) => walk(x));
    else if (n && typeof n === 'object') for (const [k, v] of Object.entries(n)) walk(v, k);
  };
  walk(d);
  return [...ids];
}

/** F053-AC-05: an asset not `ready` yet (or rejected) blocks submit. */
export function mediaGate(d: QuestionDraft, statuses: Readonly<Record<string, string>>): Gate[] {
  return mediaRefs(d)
    .filter((id) => statuses[id] && statuses[id] !== 'ready')
    .map((id) => ({ field: id, key: statuses[id] === 'rejected' ? 'content_studio.gate.mediaRejected' : 'content_studio.gate.mediaNotReady' }));
}

/** JSON pointer (`/stem/audio_ref`, `/render_payload/options/1/content`) → dotted editor path. */
export function pointerToField(pointer: string): string {
  return pointer.replace(/^\//, '').split('/').map((p) => p.replace(/~1/g, '/').replace(/~0/g, '~')).join('.');
}

/** Problem codes whose `errors[]` become blockers of the editor (F053-AC-04, AC-18, AC-23, AC-05, AC-14). */
export const CONTENT_PROBLEMS = ['AUDIO_REQUIRED', 'TRANSLATION_INCOMPLETE', 'MEDIA_NOT_READY', 'MATH_RENDER_FAILED', 'VALIDATION_FAILED'] as const;

/** Server errors → gates by field (pointer fields converted). */
export function serverGates(err: ApiError): Gate[] {
  if (!(CONTENT_PROBLEMS as readonly string[]).includes(err.code)) return [];
  const fields = Object.entries(err.fieldErrors);
  if (fields.length === 0) return [{ field: '', key: `content_studio.problem.${err.code}` }];
  return fields.map(([field, fe]) => ({ field: field.startsWith('/') ? pointerToField(field) : field, key: `content_studio.problem.${err.code}`, params: { code: fe.code } }));
}
