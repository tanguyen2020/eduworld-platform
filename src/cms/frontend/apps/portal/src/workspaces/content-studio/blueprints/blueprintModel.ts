// blueprintModel (F054-AC-22, F054 T071, assessment-blueprint.schema.json — pure): blueprint state ↔ authoring body. The
// first section is edited either as the LO × difficulty matrix or as a fixed form (F035 efficacy instruments:
// `variants.strategy = fixed`, every rule lists `fixed_question_version_ids`, rules grouped by consecutive points);
// further sections (if any) are kept as they are [Proposed: one section per blueprint in the portal UI]; unknown
// definition keys (variants included in matrix mode) are kept.
import type { EditorBlocker } from '@eduworld/ops-ui/content';
import type { BlueprintAuthoring } from '../builders/useDocument';
import { BP_LIMITS, fromRules, toRules, totalPoints, type BpRule, type Matrix } from './matrixModel';

export type ExamType = 'practice_test' | 'mock' | 'chapter' | 'semester' | 'subject' | 'custom';
export type ReleaseMode = 'immediate' | 'after_close' | 'manual';
/** How the questions of the section are chosen: random draw by the matrix, or the same fixed questions for everyone. */
export type SelectionMode = 'matrix' | 'fixed';

/** One question of a fixed form (published question version, immutable) and its points. */
export interface FixedItem {
  question_version_id: string;
  points: number;
  /** Display only (question type of the picker); not sent. */
  type?: string;
}

export interface BlueprintState {
  title: { vi?: string; en?: string };
  content_language: 'vi' | 'en';
  subject_id?: string;
  grade_levels: number[];
  topic_ids: string[];
  objective_ids: string[];
  exam_type: ExamType;
  duration_min: number;
  total_points: number;
  max_attempts: number;
  score_policy: 'best' | 'last' | 'first';
  result_release: ReleaseMode;
  section: { id: string; title: { vi?: string; en?: string } };
  mode: SelectionMode;
  matrix: Matrix;
  /** Questions of the fixed form, in exam order (F054 T071). */
  fixed: FixedItem[];
  otherSections: unknown[];
  rest: Record<string, unknown>;
  /** F035: efficacy instrument (platform scope only); undefined = never touched, not sent. */
  efficacy_instrument?: boolean;
}

export const EXAM_TYPES: ExamType[] = ['practice_test', 'mock', 'chapter', 'semester', 'subject', 'custom'];

export function emptyBlueprint(sectionTitle: string): BlueprintState {
  return {
    title: {},
    content_language: 'vi',
    grade_levels: [],
    topic_ids: [],
    objective_ids: [],
    exam_type: 'chapter',
    duration_min: 45,
    total_points: 10,
    max_attempts: 1,
    score_policy: 'best',
    result_release: 'after_close',
    section: { id: 'sec_main', title: { vi: sectionTitle } },
    mode: 'matrix',
    matrix: fromRules([]),
    fixed: [],
    otherSections: [],
    rest: {},
  };
}

interface Section {
  id: string;
  title?: { vi?: string; en?: string };
  rules?: BpRule[];
  [k: string]: unknown;
}

/** Fixed items → rules: one rule per run of consecutive questions with the same points (order kept, ≤ BP_LIMITS.count each). */
export function fixedRules(items: readonly FixedItem[]): BpRule[] {
  const out: BpRule[] = [];
  for (const it of items) {
    const last = out[out.length - 1];
    if (last && last.points_each === it.points && last.count < BP_LIMITS.count) {
      last.fixed_question_version_ids!.push(it.question_version_id);
      last.count++;
    } else out.push({ fixed_question_version_ids: [it.question_version_id], count: 1, points_each: it.points });
  }
  return out;
}

/** Rules of the first section in the active mode. */
export function activeRules(s: BlueprintState): BpRule[] {
  return s.mode === 'fixed' ? fixedRules(s.fixed) : toRules(s.matrix);
}

const isFixedRule = (r: BpRule) => (r.fixed_question_version_ids?.length ?? 0) > 0;

export function fromBlueprint(d: BlueprintAuthoring): BlueprintState {
  const { duration_sec, total_points, sections, attempt_policy, result_release, ...rest } = (d.definition ?? {}) as Record<string, unknown>;
  const secs = (sections as Section[] | undefined) ?? [];
  const first = secs[0];
  const ap = (attempt_policy ?? {}) as { max_attempts?: number; score_policy?: BlueprintState['score_policy'] };
  const rr = (result_release ?? {}) as { mode?: ReleaseMode };
  const rules = first?.rules ?? [];
  const fixedForm = (rest.variants as { strategy?: string } | undefined)?.strategy === 'fixed' && rules.length > 0 && rules.every(isFixedRule);
  return {
    title: (d.title ?? {}) as BlueprintState['title'],
    content_language: d.content_language === 'en' ? 'en' : 'vi',
    subject_id: d.subject_id,
    grade_levels: d.grade_levels ?? [],
    topic_ids: d.topic_ids ?? [],
    objective_ids: d.objective_ids ?? [],
    exam_type: (d.exam_type as ExamType) ?? 'chapter',
    duration_min: Math.round(((duration_sec as number) ?? 2700) / 60),
    total_points: (total_points as number) ?? 10,
    max_attempts: ap.max_attempts ?? 1,
    score_policy: ap.score_policy ?? 'best',
    result_release: rr.mode ?? (d.result_release as ReleaseMode) ?? 'after_close',
    section: { id: first?.id ?? 'sec_main', title: first?.title ?? {} },
    mode: fixedForm ? 'fixed' : 'matrix',
    matrix: fromRules(fixedForm ? [] : rules, d.objective_ids ?? []),
    fixed: fixedForm ? rules.flatMap((r) => r.fixed_question_version_ids!.map((id) => ({ question_version_id: id, points: r.points_each }))) : [],
    otherSections: secs.slice(1),
    rest: { ...rest, ...(first ? { __firstSectionExtra: Object.fromEntries(Object.entries(first).filter(([k]) => !['id', 'title', 'rules'].includes(k))) } : {}) },
    ...((d as { efficacy_instrument?: boolean }).efficacy_instrument ? { efficacy_instrument: true } : {}),
  };
}

export function toBlueprintBody(s: BlueprintState): Record<string, unknown> {
  const { __firstSectionExtra, variants, ...rest } = s.rest as { __firstSectionExtra?: Record<string, unknown>; variants?: Record<string, unknown> };
  const rules = activeRules(s);
  // A fixed form is the same questions for everyone: strategy fixed (no draw, no variant count/balance).
  const nextVariants = s.mode === 'fixed' ? { strategy: 'fixed' } : variants;
  const objectives = s.mode === 'fixed' ? s.objective_ids : [...new Set([...s.objective_ids, ...s.matrix.objectives])];
  return {
    title: s.title,
    content_language: s.content_language,
    exam_type: s.exam_type,
    ...(s.subject_id ? { subject_id: s.subject_id } : {}),
    grade_levels: s.grade_levels,
    topic_ids: s.topic_ids,
    objective_ids: objectives.slice(0, 50),
    result_release: s.result_release,
    ...(s.efficacy_instrument === undefined ? {} : { efficacy_instrument: s.efficacy_instrument }),
    definition: {
      ...rest,
      ...(nextVariants ? { variants: nextVariants } : {}),
      duration_sec: s.duration_min * 60,
      total_points: s.total_points,
      sections: [{ ...(__firstSectionExtra ?? {}), id: s.section.id, title: s.section.title, rules }, ...s.otherSections],
      attempt_policy: { max_attempts: s.max_attempts, score_policy: s.score_policy },
      result_release: { mode: s.result_release },
    },
  };
}

export function blueprintIssues(s: BlueprintState, t: (k: string, o?: Record<string, unknown>) => string): EditorBlocker[] {
  const out: EditorBlocker[] = [];
  const rules = activeRules(s);
  const field = s.mode === 'fixed' ? 'fixed' : 'matrix';
  if (!s.title[s.content_language]?.trim()) out.push({ field: 'title', message: t('content_studio.blueprint.issue.title') });
  if (rules.length === 0) out.push({ field, message: t(s.mode === 'fixed' ? 'content_studio.blueprint.issue.fixedEmpty' : 'content_studio.blueprint.issue.empty') });
  if (rules.length > BP_LIMITS.rules) out.push({ field, message: t('content_studio.validation.maxItems', { limit: BP_LIMITS.rules }) });
  if (s.mode === 'fixed' && new Set(s.fixed.map((f) => f.question_version_id)).size !== s.fixed.length) out.push({ field, message: t('content_studio.blueprint.issue.fixedDuplicate') });
  if (s.grade_levels.length === 0) out.push({ field: 'grade_levels', message: t('content_studio.blueprint.issue.grades') });
  if (!s.subject_id) out.push({ field: 'subject_id', message: t('content_studio.blueprint.issue.subject') });
  const pts = totalPoints(rules);
  if (rules.length && Math.abs(pts - s.total_points) > 0.001) out.push({ field, message: t(s.mode === 'fixed' ? 'content_studio.blueprint.fixedPointsMismatch' : 'content_studio.blueprint.pointsMismatch', { points: pts, target: s.total_points }) });
  if (!(s.duration_min >= 1 && s.duration_min <= 240)) out.push({ field: 'duration', message: t('content_studio.blueprint.issue.duration') });
  if (s.efficacy_instrument && s.mode !== 'fixed') out.push({ field: 'efficacy_instrument', message: t('content_studio.blueprint.issue.efficacyFixed') });
  return out;
}
