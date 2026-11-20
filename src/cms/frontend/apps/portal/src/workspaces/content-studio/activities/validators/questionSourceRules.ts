// questionSourceRules (F058 T025, F058-AC-03, AC-04, ACTIVITY_ENGINE §3, pure): early checks of the question source
// and grading of a quiz / interactive exercise — a question whose type is outside `allowed_question_types` is refused
// before it is added, list size within maxItems, a selection with a count, strategies within the registry list of the
// type. Module learning validates again (ACTIVITY_CONFIG_INVALID).
export type SourceKind = 'questions' | 'question_set' | 'selection';

export interface QuestionSource {
  kind: SourceKind;
  questions?: { question_version_id: string; type?: string }[];
  question_set_id?: string;
  question_set_version?: number | null;
  selection?: { topic_ids?: string[]; objective_ids?: string[]; difficulty_min?: number; difficulty_max?: number; question_types?: string[]; count: number };
}

/** maxItems of questionSource.questions (activity-base.schema.json). */
export const MAX_QUESTIONS = 100;

/** Auto-graded question types (ADR-0072): the default of `allowed_question_types` of the question-based types. */
export const AUTO_TYPES = ['single_choice', 'multiple_choice', 'true_false', 'fill_in_blank', 'short_answer', 'numeric', 'matching', 'ordering', 'drag_drop', 'image_based', 'interactive'];

/** Whether a picked question may be added: allowed type, not already there, room left. */
export function canAddQuestion(source: QuestionSource, q: { question_version_id: string; type: string }, allowed: readonly string[]): 'ok' | 'type' | 'duplicate' | 'full' {
  const list = source.questions ?? [];
  if (allowed.length > 0 && !allowed.includes(q.type)) return 'type';
  if (list.some((x) => x.question_version_id === q.question_version_id)) return 'duplicate';
  if (list.length >= MAX_QUESTIONS) return 'full';
  return 'ok';
}

export interface SourceIssue {
  pointer: string;
  key: string;
}

export function sourceIssues(source: QuestionSource | undefined, allowed: readonly string[]): SourceIssue[] {
  const out: SourceIssue[] = [];
  const at = (p: string, key: string) => out.push({ pointer: `/config/content/question_source${p}`, key });
  if (!source) return [{ pointer: '/config/content/question_source', key: 'source.missing' }];
  if (source.kind === 'questions') {
    const list = source.questions ?? [];
    if (list.length === 0) at('/questions', 'source.noQuestion');
    list.forEach((q, i) => {
      if (allowed.length > 0 && q.type && !allowed.includes(q.type)) at(`/questions/${i}`, 'source.typeNotAllowed');
    });
  } else if (source.kind === 'question_set') {
    if (!source.question_set_id) at('/question_set_id', 'source.noSet');
  } else {
    const s = source.selection;
    if (!s || !(s.count >= 1 && s.count <= MAX_QUESTIONS)) at('/selection/count', 'source.count');
    if (s?.difficulty_min && s.difficulty_max && s.difficulty_min > s.difficulty_max) at('/selection/difficulty_max', 'source.difficulty');
    (s?.question_types ?? []).forEach((t, i) => {
      if (allowed.length > 0 && !allowed.includes(t)) at(`/selection/question_types/${i}`, 'source.typeNotAllowed');
    });
  }
  return out;
}

/** Strategies offered for a type: the registry list of the type (ActivityTypeInfo.evaluation_strategies). */
export function offeredStrategies(registry: readonly string[] | undefined): string[] {
  return [...(registry ?? [])];
}
