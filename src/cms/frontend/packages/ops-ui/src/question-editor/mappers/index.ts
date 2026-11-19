// Question form ↔ authoring payload (F053-AC-01, AC-02, AC-03, AC-20; QUESTION_BANK §2–§3, question.schema.json).
// The form value IS the authoring shape of QuestionAuthoringInput (type, stem, render_payload, answer_key,
// scoring_rule, explanation, hints) so a draft round-trips unchanged; per type this module gives the empty skeleton
// (with random item ids), the answer-key checks run before saving, the type switch, the scoring methods and the
// student view (render payload without feedback / misconception — what the game receives). Option `feedback` and
// `misconception_tag` travel inside `render_payload.options[]` as the contract says; the server strips them from the
// student DTO (ADR-0016). Pure functions: the server stays the judge (validators F022/F023).
import type { I18nTextValue } from '../../i18n-field/translationStatus';
import { hasMath } from '../../math/delimiters';
import { emptyRichContent, type RichContent } from '../../rich-content/types';
import { itemId, isItemId, type ItemPrefix } from '../itemId';

export const QUESTION_TYPES = ['single_choice', 'multiple_choice', 'true_false', 'fill_in_blank', 'short_answer', 'numeric', 'matching', 'ordering', 'drag_drop', 'image_based', 'interactive'] as const;
export type QuestionType = (typeof QUESTION_TYPES)[number];
/** Types validated by F023 (server answers 422 TYPE_NOT_ENABLED until it is released). */
export const ADVANCED_TYPES: readonly QuestionType[] = ['short_answer', 'image_based', 'interactive'];

export type PartialMethod = 'right_minus_wrong' | 'proportional' | 'position' | 'adjacent_pairs';
export interface ScoringRule {
  mode: 'all_or_nothing' | 'partial';
  partial_method?: PartialMethod;
  min_score?: number;
  require_action_log?: boolean;
}

export interface OptionItem {
  id: string;
  content: RichContent;
  misconception_tag?: string;
  feedback?: I18nTextValue;
}

export interface TextMatch {
  accepted: string[];
  match?: 'exact' | 'normalized';
  allow_diacritics_missing?: boolean;
  case_sensitive?: boolean;
  fuzzy?: { enabled?: boolean; max_edit_distance?: number; min_length?: number };
}

export interface QuestionDraft {
  type: QuestionType;
  stem: RichContent;
  render_payload: Record<string, unknown>;
  answer_key: Record<string, unknown>;
  scoring_rule: ScoringRule;
  explanation?: RichContent;
  hints?: RichContent[];
}

/** question.schema.json limits used by the editors. */
export const LIMITS = {
  hints: 5,
  misconceptionTag: 64,
  feedback: 500,
  acceptedAnswer: 255,
  label: 255,
  wordBank: 128,
  unitLabel: 128,
  options: { single_choice: [2, 8], multiple_choice: [3, 10], matchingLeft: [2, 8], matchingRight: [2, 10], ordering: [3, 10], draggables: [1, 16], zones: [1, 8], statements: [1, 8], blanks: [1, 10] },
} as const;

const option = (prefix: ItemPrefix, taken: string[]): OptionItem => {
  const id = itemId(prefix, taken);
  taken.push(id);
  return { id, content: emptyRichContent() };
};

const options = (prefix: ItemPrefix, n: number, taken: string[] = []) => Array.from({ length: n }, () => option(prefix, taken));

/** Empty skeleton of a type (random ids). */
export function defaultsFor(type: QuestionType): Pick<QuestionDraft, 'render_payload' | 'answer_key' | 'scoring_rule'> {
  switch (type) {
    case 'single_choice':
      return { render_payload: { options: options('o', 4), shuffle_options: true }, answer_key: {}, scoring_rule: { mode: 'all_or_nothing' } };
    case 'multiple_choice':
      return { render_payload: { options: options('o', 4), shuffle_options: true, show_correct_count: false }, answer_key: { correct_option_ids: [] }, scoring_rule: { mode: 'partial', partial_method: 'right_minus_wrong', min_score: 0 } };
    case 'true_false':
      return { render_payload: {}, answer_key: {}, scoring_rule: { mode: 'all_or_nothing' } };
    case 'fill_in_blank': {
      const id = itemId('b');
      return { render_payload: { blanks: [{ id, input: 'text', max_length: 30 }] }, answer_key: { blanks: { [id]: { accepted: [], match: 'normalized' } } }, scoring_rule: { mode: 'partial', partial_method: 'proportional' } };
    }
    case 'short_answer':
      return { render_payload: { max_length: 100, keyboard: 'text' }, answer_key: { accepted: [], match: 'normalized' }, scoring_rule: { mode: 'all_or_nothing' } };
    case 'numeric':
      return { render_payload: { keyboard: 'numeric', allow_fraction_input: false, decimal_separator: ',' }, answer_key: { tolerance: { type: 'exact' }, accept_equivalent_forms: true }, scoring_rule: { mode: 'all_or_nothing' } };
    case 'matching':
      return { render_payload: { left: options('l', 2), right: options('r', 2), shuffle: true }, answer_key: { pairs: [] }, scoring_rule: { mode: 'partial', partial_method: 'proportional' } };
    case 'ordering': {
      const items = options('i', 3);
      return { render_payload: { items, shuffle: true }, answer_key: { order: items.map((x) => x.id) }, scoring_rule: { mode: 'partial', partial_method: 'adjacent_pairs' } };
    }
    case 'drag_drop': {
      const zone = itemId('z');
      return { render_payload: { background_media_ref: null, draggables: options('d', 2), zones: [{ id: zone, label: '', capacity: null }], reusable_draggables: false }, answer_key: { placements: {} }, scoring_rule: { mode: 'partial', partial_method: 'proportional' } };
    }
    case 'image_based':
      return { render_payload: { interaction: 'hotspot', image_size: { w: 1024, h: 768 }, max_selections: 1 }, answer_key: { regions: [], correct_region_ids: [] }, scoring_rule: { mode: 'all_or_nothing' } };
    case 'interactive':
      return { render_payload: { widget: '', widget_version: '1.0', config: {} }, answer_key: { validator: '', params: {} }, scoring_rule: { mode: 'all_or_nothing', require_action_log: true } };
  }
}

export function newQuestionDraft(type: QuestionType): QuestionDraft {
  return { type, stem: emptyRichContent(), ...defaultsFor(type), hints: [] };
}

/** `partial_method` values allowed per type (QUESTION_BANK §3). */
export function scoringMethods(type: QuestionType): PartialMethod[] {
  switch (type) {
    case 'multiple_choice':
      return ['right_minus_wrong', 'proportional'];
    case 'true_false':
    case 'fill_in_blank':
    case 'matching':
    case 'drag_drop':
      return ['proportional'];
    case 'ordering':
      return ['position', 'adjacent_pairs'];
    default:
      return [];
  }
}

const arr = <T>(v: unknown): T[] => (Array.isArray(v) ? (v as T[]) : []);
const obj = (v: unknown): Record<string, unknown> => (v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : {});

/** True when the draft holds an answer worth confirming before a type switch. */
export function hasAnswer(d: QuestionDraft): boolean {
  const ak = d.answer_key;
  return Object.values(ak).some((v) => (Array.isArray(v) ? v.length > 0 : v && typeof v === 'object' ? Object.keys(v).length > 0 : v !== undefined && v !== ''));
}

/** Switches the type: single ↔ multiple keep the options (answer converted); other switches drop the incompatible answer. */
export function changeType(d: QuestionDraft, type: QuestionType): QuestionDraft {
  if (d.type === type) return d;
  const choice = (t: QuestionType) => t === 'single_choice' || t === 'multiple_choice';
  if (choice(d.type) && choice(type)) {
    const opts = arr<OptionItem>(d.render_payload.options);
    const base = defaultsFor(type);
    const correct = type === 'single_choice' ? arr<string>(d.answer_key.correct_option_ids)[0] : d.answer_key.correct_option_id;
    const answer_key = type === 'single_choice' ? (correct ? { correct_option_id: correct } : {}) : { correct_option_ids: correct ? [correct as string] : [] };
    const extra = type === 'multiple_choice' && opts.length < LIMITS.options.multiple_choice[0] ? options('o', LIMITS.options.multiple_choice[0] - opts.length, opts.map((o) => o.id)) : [];
    return { ...d, type, render_payload: { ...base.render_payload, options: [...opts, ...extra] }, answer_key, scoring_rule: base.scoring_rule };
  }
  return { ...d, type, ...defaultsFor(type) };
}

export function needsTypeChangeConfirm(d: QuestionDraft, type: QuestionType): boolean {
  if (d.type === type || !hasAnswer(d)) return false;
  const choice = (t: QuestionType) => t === 'single_choice' || t === 'multiple_choice';
  return !(choice(d.type) && choice(type));
}

export type AnswerIssueCode = 'ANSWER_MISSING' | 'ANSWER_UNKNOWN_ID' | 'ITEMS_MIN' | 'ITEMS_MAX' | 'ITEM_ID_INVALID' | 'ITEM_ID_DUPLICATE' | 'BLANK_NOT_IN_STEM' | 'ACCEPTED_MISSING' | 'HINTS_MAX';

export interface AnswerIssue {
  /** Field id (dotted), e.g. `answer_key.correct_option_id`, `render_payload.options`. */
  field: string;
  code: AnswerIssueCode;
  /** Offending id or count. */
  value?: string | number;
}

function idsIssues(list: { id: string }[], field: string, min: number, max: number): AnswerIssue[] {
  const out: AnswerIssue[] = [];
  if (list.length < min) out.push({ field, code: 'ITEMS_MIN', value: min });
  if (list.length > max) out.push({ field, code: 'ITEMS_MAX', value: max });
  const seen = new Set<string>();
  for (const x of list) {
    if (!isItemId(x.id)) out.push({ field, code: 'ITEM_ID_INVALID', value: x.id });
    if (seen.has(x.id)) out.push({ field, code: 'ITEM_ID_DUPLICATE', value: x.id });
    seen.add(x.id);
  }
  return out;
}

const refIssue = (field: string, ids: Set<string>, ref: unknown): AnswerIssue[] => (typeof ref === 'string' && ids.has(ref) ? [] : [{ field, code: ref === undefined || ref === '' ? 'ANSWER_MISSING' : 'ANSWER_UNKNOWN_ID', value: typeof ref === 'string' ? ref : undefined }]);

function stemText(stem: RichContent): string {
  return JSON.stringify(stem.blocks.map((b) => [b.text, b.items, b.rows]));
}

/**
 * F053-AC-02: answer-key checks per type — ids exist, answer present, item counts within the schema. Blocking: the
 * draft is not saved while one remains (local changes stay in memory).
 */
export function answerIssues(d: QuestionDraft): AnswerIssue[] {
  const rp = d.render_payload;
  const ak = d.answer_key;
  const out: AnswerIssue[] = [];
  if ((d.hints?.length ?? 0) > LIMITS.hints) out.push({ field: 'hints', code: 'HINTS_MAX', value: LIMITS.hints });
  switch (d.type) {
    case 'single_choice':
    case 'multiple_choice': {
      const opts = arr<OptionItem>(rp.options);
      const [min, max] = LIMITS.options[d.type];
      out.push(...idsIssues(opts, 'render_payload.options', min, max));
      const ids = new Set(opts.map((o) => o.id));
      if (d.type === 'single_choice') out.push(...refIssue('answer_key.correct_option_id', ids, ak.correct_option_id));
      else {
        const sel = arr<string>(ak.correct_option_ids);
        if (sel.length === 0) out.push({ field: 'answer_key.correct_option_ids', code: 'ANSWER_MISSING' });
        for (const s of sel) if (!ids.has(s)) out.push({ field: 'answer_key.correct_option_ids', code: 'ANSWER_UNKNOWN_ID', value: s });
      }
      break;
    }
    case 'true_false': {
      const st = arr<{ id: string }>(rp.statements);
      if (st.length === 0) {
        if (typeof ak.value !== 'boolean') out.push({ field: 'answer_key.value', code: 'ANSWER_MISSING' });
      } else {
        out.push(...idsIssues(st, 'render_payload.statements', ...LIMITS.options.statements));
        const values = obj(ak.statement_values);
        for (const s of st) if (typeof values[s.id] !== 'boolean') out.push({ field: `answer_key.statement_values.${s.id}`, code: 'ANSWER_MISSING', value: s.id });
        for (const k of Object.keys(values)) if (!st.some((s) => s.id === k)) out.push({ field: 'answer_key.statement_values', code: 'ANSWER_UNKNOWN_ID', value: k });
      }
      break;
    }
    case 'fill_in_blank': {
      const blanks = arr<{ id: string }>(rp.blanks);
      out.push(...idsIssues(blanks, 'render_payload.blanks', ...LIMITS.options.blanks));
      const keys = obj(ak.blanks);
      const text = stemText(d.stem);
      for (const b of blanks) {
        const m = obj(keys[b.id]) as Partial<TextMatch>;
        if (!m.accepted?.some((a) => a.trim())) out.push({ field: `answer_key.blanks.${b.id}`, code: 'ACCEPTED_MISSING', value: b.id });
        if (!text.includes(`{{blank:${b.id}}}`)) out.push({ field: 'stem', code: 'BLANK_NOT_IN_STEM', value: b.id });
      }
      for (const k of Object.keys(keys)) if (!blanks.some((b) => b.id === k)) out.push({ field: 'answer_key.blanks', code: 'ANSWER_UNKNOWN_ID', value: k });
      break;
    }
    case 'short_answer':
      if (!arr<string>(ak.accepted).some((a) => a.trim())) out.push({ field: 'answer_key.accepted', code: 'ACCEPTED_MISSING' });
      break;
    case 'numeric':
      if (typeof ak.value !== 'number' && !(typeof ak.value === 'string' && ak.value.trim())) out.push({ field: 'answer_key.value', code: 'ANSWER_MISSING' });
      break;
    case 'matching': {
      const left = arr<OptionItem>(rp.left);
      const right = arr<OptionItem>(rp.right);
      out.push(...idsIssues(left, 'render_payload.left', ...LIMITS.options.matchingLeft), ...idsIssues(right, 'render_payload.right', ...LIMITS.options.matchingRight));
      const pairs = arr<[string, string]>(ak.pairs);
      if (pairs.length === 0) out.push({ field: 'answer_key.pairs', code: 'ANSWER_MISSING' });
      const l = new Set(left.map((x) => x.id));
      const r = new Set(right.map((x) => x.id));
      for (const [a, b] of pairs) {
        if (!l.has(a)) out.push({ field: 'answer_key.pairs', code: 'ANSWER_UNKNOWN_ID', value: a });
        if (!r.has(b)) out.push({ field: 'answer_key.pairs', code: 'ANSWER_UNKNOWN_ID', value: b });
      }
      break;
    }
    case 'ordering': {
      const items = arr<OptionItem>(rp.items);
      out.push(...idsIssues(items, 'render_payload.items', ...LIMITS.options.ordering));
      const order = arr<string>(ak.order);
      const ids = new Set(items.map((x) => x.id));
      if (order.length !== items.length || order.some((o) => !ids.has(o))) out.push({ field: 'answer_key.order', code: order.length === 0 ? 'ANSWER_MISSING' : 'ANSWER_UNKNOWN_ID' });
      break;
    }
    case 'drag_drop': {
      const drags = arr<OptionItem>(rp.draggables);
      const zones = arr<{ id: string }>(rp.zones);
      out.push(...idsIssues(drags, 'render_payload.draggables', ...LIMITS.options.draggables), ...idsIssues(zones, 'render_payload.zones', ...LIMITS.options.zones));
      const placements = obj(ak.placements);
      if (Object.keys(placements).length === 0) out.push({ field: 'answer_key.placements', code: 'ANSWER_MISSING' });
      const z = new Set(zones.map((x) => x.id));
      const dIds = new Set(drags.map((x) => x.id));
      for (const [k, v] of Object.entries(placements)) {
        if (!dIds.has(k)) out.push({ field: 'answer_key.placements', code: 'ANSWER_UNKNOWN_ID', value: k });
        if (typeof v !== 'string' || !z.has(v)) out.push({ field: `answer_key.placements.${k}`, code: 'ANSWER_UNKNOWN_ID', value: String(v) });
      }
      break;
    }
    case 'image_based': {
      if (rp.interaction === 'label') {
        const labels = arr<OptionItem>(rp.labels);
        const points = arr<{ id: string }>(rp.drop_points);
        const placements = obj(ak.placements);
        if (Object.keys(placements).length === 0) out.push({ field: 'answer_key.placements', code: 'ANSWER_MISSING' });
        const p = new Set(points.map((x) => x.id));
        const l = new Set(labels.map((x) => x.id));
        for (const [k, v] of Object.entries(placements)) if (!l.has(k) || typeof v !== 'string' || !p.has(v)) out.push({ field: 'answer_key.placements', code: 'ANSWER_UNKNOWN_ID', value: k });
      } else {
        const regions = arr<{ id: string }>(ak.regions);
        const correct = arr<string>(ak.correct_region_ids);
        if (correct.length === 0) out.push({ field: 'answer_key.correct_region_ids', code: 'ANSWER_MISSING' });
        const ids = new Set(regions.map((x) => x.id));
        for (const c of correct) if (!ids.has(c)) out.push({ field: 'answer_key.correct_region_ids', code: 'ANSWER_UNKNOWN_ID', value: c });
      }
      if (!rp.image_media_ref) out.push({ field: 'render_payload.image_media_ref', code: 'ANSWER_MISSING' });
      break;
    }
    case 'interactive':
      if (!ak.validator) out.push({ field: 'answer_key.validator', code: 'ANSWER_MISSING' });
      break;
  }
  return out;
}

const SERVER_ONLY_OPTION_KEYS = ['feedback', 'misconception_tag'] as const;
const OPTION_LISTS = ['options', 'left', 'right', 'items', 'draggables', 'labels'] as const;

/** What the student receives: stem + render payload without option feedback / misconception tags (F053-AC-03). */
export function toStudentView(d: Pick<QuestionDraft, 'type' | 'stem' | 'render_payload'>): { type: QuestionType; stem: RichContent; render_payload: Record<string, unknown> } {
  const rp: Record<string, unknown> = { ...d.render_payload };
  for (const k of OPTION_LISTS) {
    if (Array.isArray(rp[k])) {
      rp[k] = (rp[k] as Record<string, unknown>[]).map((o) => Object.fromEntries(Object.entries(o).filter(([key]) => !(SERVER_ONLY_OPTION_KEYS as readonly string[]).includes(key))));
    }
  }
  return { type: d.type, stem: d.stem, render_payload: rp };
}

/** Types where the student would type an answer (text): a formula in such a question triggers the ADR-0055 §6 notice. */
const TYPED_ANSWER: readonly QuestionType[] = ['short_answer', 'fill_in_blank'];

/** F053-AC-20: formula present + the student types the answer → warn (students never type expressions in the game). */
export function expressionInputWarning(d: QuestionDraft): boolean {
  if (!TYPED_ANSWER.includes(d.type)) return false;
  if (d.type === 'fill_in_blank' && !arr<{ input?: string }>(d.render_payload.blanks).some((b) => b.input === 'text')) return false;
  const anyMath = JSON.stringify([d.stem, d.render_payload]).includes('\\\\(') || JSON.stringify([d.stem, d.render_payload]).includes('\\\\[') || d.stem.blocks.some((b) => b.type === 'math');
  return anyMath || hasMath(JSON.stringify(d.answer_key));
}
