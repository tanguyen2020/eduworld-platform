// blueprintModel (F054-AC-22, assessment-blueprint.schema.json — pure): blueprint state ↔ authoring body. The matrix
// edits the first section; further sections (if any) are kept as they are [Proposed: one section per blueprint in the
// portal UI]; unknown definition keys are kept.
import type { EditorBlocker } from '@eduworld/ops-ui/content';
import type { BlueprintAuthoring } from '../builders/useDocument';
import { BP_LIMITS, fromRules, toRules, totalPoints, type BpRule, type Matrix } from './matrixModel';

export type ExamType = 'practice_test' | 'mock' | 'chapter' | 'semester' | 'subject' | 'custom';
export type ReleaseMode = 'immediate' | 'after_close' | 'manual';

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
  matrix: Matrix;
  otherSections: unknown[];
  rest: Record<string, unknown>;
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
    matrix: fromRules([]),
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

export function fromBlueprint(d: BlueprintAuthoring): BlueprintState {
  const { duration_sec, total_points, sections, attempt_policy, result_release, ...rest } = (d.definition ?? {}) as Record<string, unknown>;
  const secs = (sections as Section[] | undefined) ?? [];
  const first = secs[0];
  const ap = (attempt_policy ?? {}) as { max_attempts?: number; score_policy?: BlueprintState['score_policy'] };
  const rr = (result_release ?? {}) as { mode?: ReleaseMode };
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
    matrix: fromRules(first?.rules ?? [], d.objective_ids ?? []),
    otherSections: secs.slice(1),
    rest: { ...rest, ...(first ? { __firstSectionExtra: Object.fromEntries(Object.entries(first).filter(([k]) => !['id', 'title', 'rules'].includes(k))) } : {}) },
  };
}

export function toBlueprintBody(s: BlueprintState): Record<string, unknown> {
  const { __firstSectionExtra, ...rest } = s.rest as { __firstSectionExtra?: Record<string, unknown> };
  const rules = toRules(s.matrix);
  return {
    title: s.title,
    content_language: s.content_language,
    exam_type: s.exam_type,
    ...(s.subject_id ? { subject_id: s.subject_id } : {}),
    grade_levels: s.grade_levels,
    topic_ids: s.topic_ids,
    objective_ids: [...new Set([...s.objective_ids, ...s.matrix.objectives])].slice(0, 50),
    result_release: s.result_release,
    definition: {
      ...rest,
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
  const rules = toRules(s.matrix);
  if (!s.title[s.content_language]?.trim()) out.push({ field: 'title', message: t('content_studio.blueprint.issue.title') });
  if (rules.length === 0) out.push({ field: 'matrix', message: t('content_studio.blueprint.issue.empty') });
  if (rules.length > BP_LIMITS.rules) out.push({ field: 'matrix', message: t('content_studio.validation.maxItems', { limit: BP_LIMITS.rules }) });
  if (s.grade_levels.length === 0) out.push({ field: 'grade_levels', message: t('content_studio.blueprint.issue.grades') });
  if (!s.subject_id) out.push({ field: 'subject_id', message: t('content_studio.blueprint.issue.subject') });
  const pts = totalPoints(rules);
  if (rules.length && Math.abs(pts - s.total_points) > 0.001) out.push({ field: 'matrix', message: t('content_studio.blueprint.pointsMismatch', { points: pts, target: s.total_points }) });
  if (!(s.duration_min >= 1 && s.duration_min <= 240)) out.push({ field: 'duration', message: t('content_studio.blueprint.issue.duration') });
  return out;
}
