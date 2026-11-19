// lessonModel (F054-AC-21, specs/schemas/lesson.schema.json, LessonAuthoringInput — pure): lesson state ↔ authoring
// body, block list operations (add ≤ maxItems, move for drag & drop / keyboard, remove), local blockers, and the
// formulas of the rich_text / worked_example blocks sent as `math_assets` (ADR-0055).
import { newWorkedExample, type EditorBlocker, type RichContent, type WorkedExample } from '@eduworld/ops-ui/content';
import type { LessonAuthoring } from '../builders/useDocument';

export type BlockKind = 'rich_text' | 'media' | 'worked_example' | 'activity';
export const BLOCK_KINDS: BlockKind[] = ['rich_text', 'media', 'worked_example', 'activity'];
export const LESSON_LIMITS = { blocks: 100, mathAssets: 50 } as const;
export type CompletionRule = 'all_required_activities' | 'any_activity' | 'view_only';

export interface LessonBlock {
  kind: BlockKind;
  /** Client key for React lists (never sent unless `worked_example`, whose id is the contract id). */
  key: string;
  id?: string;
  required?: boolean;
  content?: RichContent;
  media_ref?: string;
  caption?: { vi?: string; en?: string };
  activity_id?: string;
  worked?: WorkedExample;
  title?: { vi?: string; en?: string };
}

export interface LessonState {
  title: { vi?: string; en?: string };
  summary?: { vi?: string; en?: string };
  content_language: 'vi' | 'en';
  subject_id?: string;
  grade_levels: number[];
  topic_ids: string[];
  objective_ids: string[];
  estimated_minutes?: number;
  completion_rule: CompletionRule;
  blocks: LessonBlock[];
}

let seq = 0;
const nextKey = () => `b${Date.now().toString(36)}${(seq++).toString(36)}`;

export function newBlock(kind: BlockKind, existingIds: readonly string[] = []): LessonBlock {
  switch (kind) {
    case 'rich_text':
      return { kind, key: nextKey(), content: { blocks: [{ type: 'paragraph' }] } };
    case 'media':
      return { kind, key: nextKey() };
    case 'worked_example': {
      const worked = newWorkedExample(existingIds);
      return { kind, key: nextKey(), id: worked.id, worked };
    }
    case 'activity':
      return { kind, key: nextKey(), required: true };
  }
}

export function emptyLesson(): LessonState {
  return { title: {}, content_language: 'vi', grade_levels: [], topic_ids: [], objective_ids: [], completion_rule: 'all_required_activities', blocks: [newBlock('rich_text')] };
}

type RawBlock = NonNullable<LessonAuthoring['blocks']>[number];

export function fromLesson(d: LessonAuthoring): LessonState {
  return {
    title: (d.title ?? {}) as LessonState['title'],
    summary: d.summary as LessonState['summary'],
    content_language: d.content_language === 'en' ? 'en' : 'vi',
    subject_id: d.subject_id,
    grade_levels: d.grade_level ? [d.grade_level] : [],
    topic_ids: d.topic_id ? [d.topic_id] : [],
    objective_ids: d.objective_ids ?? [],
    estimated_minutes: d.estimated_minutes,
    completion_rule: d.completion_rule ?? 'all_required_activities',
    blocks: (d.blocks ?? []).map((b: RawBlock) => {
      const base = { kind: b.kind, key: nextKey(), required: b.required, id: b.id } as LessonBlock;
      if (b.kind === 'rich_text') return { ...base, content: b.content as unknown as RichContent };
      if (b.kind === 'media') return { ...base, media_ref: b.media_ref, caption: b.caption as LessonBlock['caption'] };
      if (b.kind === 'activity') return { ...base, activity_id: b.activity_id };
      return {
        ...base,
        title: b.title as LessonBlock['title'],
        worked: { type: 'worked_example', id: b.id ?? '', objective_ids: b.objective_ids, problem: b.problem as unknown as RichContent, steps: (b.steps ?? []) as unknown as WorkedExample['steps'], self_explain_prompt: b.self_explain_prompt as unknown as RichContent | undefined },
      };
    }),
  };
}

/** Block → contract item. */
export function toRawBlock(b: LessonBlock): Record<string, unknown> {
  const req = b.required ? { required: true } : {};
  switch (b.kind) {
    case 'rich_text':
      return { kind: b.kind, content: b.content, ...req };
    case 'media':
      return { kind: b.kind, media_ref: b.media_ref, ...(b.caption && (b.caption.vi || b.caption.en) ? { caption: b.caption } : {}), ...req };
    case 'activity':
      return { kind: b.kind, activity_id: b.activity_id, ...req };
    case 'worked_example': {
      const w = b.worked!;
      return {
        kind: b.kind,
        id: w.id,
        problem: w.problem,
        steps: w.steps.map((st) => ({ content: st.content, ...(st.audio_ref ? { audio_ref: st.audio_ref } : {}) })),
        ...(w.objective_ids?.length ? { objective_ids: w.objective_ids } : {}),
        ...(w.self_explain_prompt ? { self_explain_prompt: w.self_explain_prompt } : {}),
        ...(b.title && (b.title.vi || b.title.en) ? { title: b.title } : {}),
        ...req,
      };
    }
  }
}

/** Payload holding every formula of the lesson (input of collectMath). */
export function mathPayload(s: LessonState): unknown {
  return s.blocks.map((b) => (b.kind === 'rich_text' ? { content: b.content } : b.kind === 'worked_example' ? { problem: b.worked?.problem, steps: b.worked?.steps, self_explain_prompt: b.worked?.self_explain_prompt } : {}));
}

export function toLessonBody(s: LessonState, mathAssets: readonly unknown[]): Record<string, unknown> {
  return {
    title: s.title,
    ...(s.summary && (s.summary.vi || s.summary.en) ? { summary: s.summary } : {}),
    content_language: s.content_language,
    ...(s.subject_id ? { subject_id: s.subject_id } : {}),
    ...(s.grade_levels[0] ? { grade_level: s.grade_levels[0] } : {}),
    ...(s.topic_ids[0] ? { topic_id: s.topic_ids[0] } : {}),
    objective_ids: s.objective_ids,
    ...(s.estimated_minutes ? { estimated_minutes: s.estimated_minutes } : {}),
    completion_rule: s.completion_rule,
    blocks: s.blocks.map(toRawBlock),
    math_assets: [...mathAssets],
  };
}

export function addBlock(blocks: readonly LessonBlock[], kind: BlockKind): LessonBlock[] {
  if (blocks.length >= LESSON_LIMITS.blocks) return [...blocks];
  const ids = blocks.filter((b) => b.kind === 'worked_example').map((b) => b.id ?? '');
  return [...blocks, newBlock(kind, ids)];
}

/** Moves block `from` to position `to` (drag & drop, or ±1 with the keyboard buttons). */
export function moveBlock(blocks: readonly LessonBlock[], from: number, to: number): LessonBlock[] {
  if (from === to || from < 0 || to < 0 || from >= blocks.length || to >= blocks.length) return [...blocks];
  const out = [...blocks];
  const [b] = out.splice(from, 1);
  out.splice(to, 0, b!);
  return out;
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function lessonIssues(s: LessonState, t: (k: string, o?: Record<string, unknown>) => string): EditorBlocker[] {
  const out: EditorBlocker[] = [];
  if (!s.title[s.content_language]?.trim()) out.push({ field: 'title', message: t('content_studio.lesson.issue.title') });
  if (s.blocks.length === 0) out.push({ field: 'blocks', message: t('content_studio.lesson.issue.blocks') });
  if (s.blocks.length > LESSON_LIMITS.blocks) out.push({ field: 'blocks', message: t('content_studio.validation.maxItems', { limit: LESSON_LIMITS.blocks }) });
  s.blocks.forEach((b, i) => {
    if (b.kind === 'media' && !b.media_ref) out.push({ field: `blocks.${i}`, message: t('content_studio.lesson.issue.media', { n: i + 1 }) });
    if (b.kind === 'activity' && !UUID.test(b.activity_id ?? '')) out.push({ field: `blocks.${i}`, message: t('content_studio.lesson.issue.activity', { n: i + 1 }) });
  });
  if (s.completion_rule === 'all_required_activities' && !s.blocks.some((b) => b.kind === 'activity' && b.required)) out.push({ field: 'completion_rule', message: t('content_studio.lesson.issue.requiredActivity') });
  return out;
}
