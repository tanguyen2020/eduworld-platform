// Curriculum tree model (F053-AC-26, AC-27; CURRICULUM_MODEL §2): Môn → môn × khối of the version → chủ đề (nested)
// → LO, built from the flat lists of the API; and the drag-and-drop rule of topics: a drop becomes an `updateTopic`
// patch (`parent_topic_id`, `sort_order`), only inside the same subject × grade and never under its own subtree.
// Pure functions (unit tested).
import type { LearningObjective, Topic } from './useCurriculum';

export interface TreeNode {
  key: string;
  kind: 'subject' | 'grade' | 'topic' | 'objective';
  title: string;
  topic?: Topic;
  objective?: LearningObjective;
  subjectId: string;
  gradeLevel?: number;
  children?: TreeNode[];
}

export interface SubjectGradeRef {
  subject_id: string;
  grade_level: number;
}

const byOrder = (a: { sort_order?: number; code?: string }, b: { sort_order?: number; code?: string }) => (a.sort_order ?? 0) - (b.sort_order ?? 0) || (a.code ?? '').localeCompare(b.code ?? '');

export function buildTree(
  subjectGrades: readonly SubjectGradeRef[],
  subjectName: (id: string) => string,
  gradeLabel: (grade: number) => string,
  topics: readonly Topic[],
  objectives: readonly LearningObjective[],
): TreeNode[] {
  const childrenOf = (parent: string | undefined, sg: SubjectGradeRef): TreeNode[] =>
    topics
      .filter((t) => (t.parent_topic_id ?? undefined) === parent && t.subject_id === sg.subject_id && t.grade_level === sg.grade_level)
      .sort(byOrder)
      .map((t) => ({
        key: `topic:${t.topic_id}`,
        kind: 'topic' as const,
        title: `${t.code ?? ''} ${t.name}`.trim(),
        topic: t,
        subjectId: sg.subject_id,
        gradeLevel: sg.grade_level,
        children: [
          ...childrenOf(t.topic_id, sg),
          ...objectives
            .filter((o) => o.topic_id === t.topic_id)
            .sort(byOrder)
            .map((o) => ({ key: `lo:${o.objective_id}`, kind: 'objective' as const, title: `${o.code} — ${o.description}`, objective: o, subjectId: sg.subject_id, gradeLevel: sg.grade_level })),
        ],
      }));
  const subjects = [...new Set(subjectGrades.map((s) => s.subject_id))];
  return subjects.map((sid) => ({
    key: `subject:${sid}`,
    kind: 'subject' as const,
    title: subjectName(sid),
    subjectId: sid,
    children: subjectGrades
      .filter((s) => s.subject_id === sid)
      .sort((a, b) => a.grade_level - b.grade_level)
      .map((sg) => ({ key: `grade:${sid}:${sg.grade_level}`, kind: 'grade' as const, title: gradeLabel(sg.grade_level), subjectId: sid, gradeLevel: sg.grade_level, children: childrenOf(undefined, sg) })),
  }));
}

/** True when `candidate` is `ancestor` or below it. */
export function isWithin(topics: readonly Topic[], candidate: string | undefined, ancestor: string): boolean {
  let cur = candidate;
  const seen = new Set<string>();
  while (cur && !seen.has(cur)) {
    if (cur === ancestor) return true;
    seen.add(cur);
    cur = topics.find((t) => t.topic_id === cur)?.parent_topic_id ?? undefined;
  }
  return false;
}

export interface DropInfo {
  /** Dragged topic. */
  drag: Topic;
  /** Target node: a topic (inside / before / after) or a grade node (top level, last). */
  target: { kind: 'topic'; topic: Topic } | { kind: 'grade'; subjectId: string; gradeLevel: number };
  /** -1 before the target, 1 after it, 0 inside it. */
  position: -1 | 0 | 1;
}

/** `updateTopic` patch of a drop, or null when the drop is not allowed (other subject × grade, own subtree, no-op). */
export function dropToPatch(topics: readonly Topic[], d: DropInfo): { parent_topic_id: string | null; sort_order: number } | null {
  const siblings = (parent: string | null) =>
    topics.filter((t) => (t.parent_topic_id ?? null) === parent && t.subject_id === d.drag.subject_id && t.grade_level === d.drag.grade_level && t.topic_id !== d.drag.topic_id).sort(byOrder);
  let parent: string | null;
  let index: number;
  if (d.target.kind === 'grade') {
    if (d.target.subjectId !== d.drag.subject_id || d.target.gradeLevel !== d.drag.grade_level) return null;
    parent = null;
    index = siblings(null).length;
  } else {
    const target = d.target.topic;
    if (target.subject_id !== d.drag.subject_id || target.grade_level !== d.drag.grade_level) return null;
    if (target.topic_id === d.drag.topic_id) return null;
    if (d.position === 0) {
      if (isWithin(topics, target.topic_id, d.drag.topic_id)) return null;
      parent = target.topic_id;
      index = siblings(parent).length;
    } else {
      parent = target.parent_topic_id ?? null;
      if (parent && isWithin(topics, parent, d.drag.topic_id)) return null;
      const sib = siblings(parent);
      const at = sib.findIndex((t) => t.topic_id === target.topic_id);
      index = d.position < 0 ? at : at + 1;
    }
  }
  const sortOrder = index + 1;
  if ((d.drag.parent_topic_id ?? null) === parent && d.drag.sort_order === sortOrder) return null;
  return { parent_topic_id: parent, sort_order: sortOrder };
}

/** Local optimistic move (shown until the server answers; reverted on error). */
export function applyMove(topics: readonly Topic[], id: string, patch: { parent_topic_id: string | null; sort_order: number }): Topic[] {
  const moved = topics.find((t) => t.topic_id === id);
  if (!moved) return [...topics];
  const others = topics.filter((t) => t.topic_id !== id);
  const sib = others.filter((t) => (t.parent_topic_id ?? null) === patch.parent_topic_id && t.subject_id === moved.subject_id && t.grade_level === moved.grade_level).sort(byOrder);
  sib.splice(patch.sort_order - 1, 0, { ...moved, parent_topic_id: patch.parent_topic_id ?? undefined });
  const renumbered = new Map(sib.map((t, i) => [t.topic_id, { ...t, sort_order: i + 1 }]));
  return [...others.filter((t) => !renumbered.has(t.topic_id)), ...renumbered.values()];
}
