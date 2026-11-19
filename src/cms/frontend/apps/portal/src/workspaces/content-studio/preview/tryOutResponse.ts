// tryOutResponse (F054-AC-10, pure): the answer typed in "Thử làm" → a QuestionResponse of the contract for the
// question types the portal can capture (choice, true/false, blanks, short answer, numeric, ordering). Other types
// (matching, drag & drop, image based, interactive widgets) are tried in the LMS / game renderer [Proposed]. Scoring is
// always done by the server (submitPreviewResponse, no attempt stored — ADR-0031).
import type { components } from '../../../shared/api/schema.gen';

export type QuestionResponse = components['schemas']['QuestionResponse'];

export type TryOutAnswer =
  | { kind: 'single'; optionId?: string }
  | { kind: 'multiple'; optionIds: string[] }
  | { kind: 'true_false'; value?: boolean; statements: Record<string, boolean> }
  | { kind: 'blanks'; blanks: Record<string, string> }
  | { kind: 'text'; text: string }
  | { kind: 'numeric'; value: string }
  | { kind: 'ordering'; order: string[] };

export const TRY_OUT_TYPES = ['single_choice', 'multiple_choice', 'true_false', 'fill_in_blank', 'short_answer', 'numeric', 'ordering'] as const;

export function canTryOut(type: string): boolean {
  return (TRY_OUT_TYPES as readonly string[]).includes(type);
}

interface Item {
  id: string;
}

const ids = (rp: Record<string, unknown>, key: string) => (Array.isArray(rp[key]) ? (rp[key] as Item[]).map((x) => x.id) : []);

/** Empty answer for a rendered question. */
export function emptyAnswer(type: string, rp: Record<string, unknown>): TryOutAnswer | null {
  switch (type) {
    case 'single_choice':
      return { kind: 'single' };
    case 'multiple_choice':
      return { kind: 'multiple', optionIds: [] };
    case 'true_false':
      return { kind: 'true_false', statements: {} };
    case 'fill_in_blank':
      return { kind: 'blanks', blanks: Object.fromEntries(ids(rp, 'blanks').map((b) => [b, ''])) };
    case 'short_answer':
      return { kind: 'text', text: '' };
    case 'numeric':
      return { kind: 'numeric', value: '' };
    case 'ordering':
      return { kind: 'ordering', order: ids(rp, 'items') };
    default:
      return null;
  }
}

/** The contract response, or null while the answer is incomplete. */
export function buildResponse(a: TryOutAnswer, rp: Record<string, unknown>): QuestionResponse | null {
  switch (a.kind) {
    case 'single':
      return a.optionId ? { option_id: a.optionId } : null;
    case 'multiple':
      return a.optionIds.length ? { option_ids: [...a.optionIds] } : null;
    case 'true_false': {
      const statements = ids(rp, 'statements');
      if (statements.length === 0) return a.value === undefined ? null : { value: a.value };
      return statements.every((s) => a.statements[s] !== undefined) ? { statement_values: { ...a.statements } } : null;
    }
    case 'blanks': {
      const filled = Object.entries(a.blanks).filter(([, v]) => v.trim());
      return filled.length ? { blanks: Object.fromEntries(filled.map(([k, v]) => [k, v.trim().slice(0, 200)])) } : null;
    }
    case 'text':
      return a.text.trim() ? { text: a.text.trim().slice(0, 500) } : null;
    case 'numeric':
      return a.value.trim() ? { value: a.value.trim().slice(0, 64) } : null;
    case 'ordering':
      return a.order.length >= 3 ? { order: [...a.order] } : null;
  }
}

/** Moves item `i` by `delta` in an ordering answer (keyboard friendly Lên / Xuống). */
export function moveItem(order: readonly string[], i: number, delta: -1 | 1): string[] {
  const j = i + delta;
  if (j < 0 || j >= order.length) return [...order];
  const out = [...order];
  [out[i], out[j]] = [out[j]!, out[i]!];
  return out;
}
