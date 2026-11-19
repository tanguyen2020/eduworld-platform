// questionSetModel (F054-AC-20, specs/schemas/question-set.schema.json, QUESTION_BANK §6 — pure): editor state of a
// question set ↔ QuestionSetAuthoringInput, and the local checks shown as blockers (static needs items, rule based
// needs rules, `maxItems`). The server validates the definition again (422 VALIDATION_FAILED).
import type { EditorBlocker } from '@eduworld/ops-ui/content';
import type { QuestionSetAuthoring } from '../builders/useDocument';

export type SetKind = 'static' | 'rule_based' | 'mixed';
export type SetPurpose = 'exercise' | 'minigame' | 'quest' | 'competition' | 'general';

export interface SetItem {
  question_version_id: string;
  max_score?: number;
  position?: number;
  /** Display only (title of the picked question), never sent. */
  label?: string;
}

export interface RuleFilters {
  subject_id?: string;
  grade_levels?: number[];
  topic_ids?: string[];
  objective_ids?: string[];
  difficulty?: { min?: number; max?: number };
  types?: string[];
}

export interface SelectionRule {
  filters: RuleFilters;
  count: number;
  [k: string]: unknown;
}

export interface QuestionSetState {
  title: { vi?: string; en?: string };
  content_language: 'vi' | 'en';
  subject_id?: string;
  grade_levels: number[];
  topic_ids: string[];
  objective_ids: string[];
  kind: SetKind;
  purpose: SetPurpose;
  items: SetItem[];
  rules: SelectionRule[];
  shuffle_items: boolean;
  shuffle_options: boolean;
  /** Definition keys the editor does not know (kept as they are). */
  rest: Record<string, unknown>;
}

export const SET_LIMITS = { items: 200, rules: 20, count: 100 } as const;

export function emptyQuestionSet(): QuestionSetState {
  return { title: {}, content_language: 'vi', grade_levels: [], topic_ids: [], objective_ids: [], kind: 'static', purpose: 'exercise', items: [], rules: [], shuffle_items: true, shuffle_options: true, rest: {} };
}

export function fromQuestionSet(d: QuestionSetAuthoring): QuestionSetState {
  const { kind, purpose, items, selection_rules, shuffle_items, shuffle_options, ...rest } = (d.definition ?? {}) as Record<string, unknown>;
  return {
    title: (d.title ?? {}) as QuestionSetState['title'],
    content_language: d.content_language === 'en' ? 'en' : 'vi',
    subject_id: d.subject_id,
    grade_levels: d.grade_levels ?? [],
    topic_ids: d.topic_ids ?? [],
    objective_ids: d.objective_ids ?? [],
    kind: (kind as SetKind) ?? 'static',
    purpose: (purpose as SetPurpose) ?? 'exercise',
    items: (items as SetItem[]) ?? [],
    rules: (selection_rules as SelectionRule[]) ?? [],
    shuffle_items: shuffle_items !== false,
    shuffle_options: shuffle_options !== false,
    rest,
  };
}

const clean = (f: RuleFilters): RuleFilters => {
  const out: RuleFilters = {};
  if (f.subject_id) out.subject_id = f.subject_id;
  if (f.grade_levels?.length) out.grade_levels = f.grade_levels;
  if (f.topic_ids?.length) out.topic_ids = f.topic_ids;
  if (f.objective_ids?.length) out.objective_ids = f.objective_ids;
  if (f.types?.length) out.types = f.types;
  if (f.difficulty && (f.difficulty.min || f.difficulty.max)) out.difficulty = { ...(f.difficulty.min ? { min: f.difficulty.min } : {}), ...(f.difficulty.max ? { max: f.difficulty.max } : {}) };
  return out;
};

export function toQuestionSetBody(s: QuestionSetState): Record<string, unknown> {
  const useItems = s.kind !== 'rule_based';
  const useRules = s.kind !== 'static';
  return {
    title: s.title,
    content_language: s.content_language,
    ...(s.subject_id ? { subject_id: s.subject_id } : {}),
    grade_levels: s.grade_levels,
    topic_ids: s.topic_ids,
    objective_ids: s.objective_ids,
    definition: {
      ...s.rest,
      kind: s.kind,
      purpose: s.purpose,
      ...(useItems ? { items: s.items.map((it, i) => ({ question_version_id: it.question_version_id, position: i + 1, ...(it.max_score ? { max_score: it.max_score } : {}) })) } : {}),
      ...(useRules ? { selection_rules: s.rules.map((r) => ({ ...r, filters: clean(r.filters) })) } : {}),
      shuffle_items: s.shuffle_items,
      shuffle_options: s.shuffle_options,
    },
  };
}

/** Local blockers (message keys under content_studio.questionSet.issue.*). */
export function questionSetIssues(s: QuestionSetState, t: (k: string, o?: Record<string, unknown>) => string): EditorBlocker[] {
  const out: EditorBlocker[] = [];
  if (!s.title[s.content_language]?.trim()) out.push({ field: 'title', message: t('content_studio.questionSet.issue.title') });
  if (s.kind !== 'rule_based' && s.items.length === 0) out.push({ field: 'items', message: t('content_studio.questionSet.issue.items') });
  if (s.kind !== 'static' && s.rules.length === 0) out.push({ field: 'rules', message: t('content_studio.questionSet.issue.rules') });
  if (s.items.length > SET_LIMITS.items) out.push({ field: 'items', message: t('content_studio.validation.maxItems', { limit: SET_LIMITS.items }) });
  if (s.rules.length > SET_LIMITS.rules) out.push({ field: 'rules', message: t('content_studio.validation.maxItems', { limit: SET_LIMITS.rules }) });
  s.rules.forEach((r, i) => {
    if (!(r.count >= 1 && r.count <= SET_LIMITS.count)) out.push({ field: `rules.${i}.count`, message: t('content_studio.questionSet.issue.count', { n: i + 1 }) });
    const d = r.filters.difficulty;
    if (d?.min && d.max && d.min > d.max) out.push({ field: `rules.${i}.difficulty`, message: t('content_studio.questionSet.issue.difficulty', { n: i + 1 }) });
  });
  return out;
}
