// F053 T046 (model part) — F053-AC-26, AC-27: tree Môn → môn × khối → chủ đề → LO, and drops → updateTopic patches
// (parent_topic_id, sort_order) only inside the same subject × grade and never under the dragged subtree.
import { describe, expect, it } from 'vitest';
import { applyMove, buildTree, dropToPatch } from './treeModel';
import type { LearningObjective, Topic } from './useCurriculum';

const S = 'subj-math';
const tp = (id: string, sort: number, parent?: string, grade = 3): Topic => ({ topic_id: id, name: id, code: id.toUpperCase(), subject_id: S, grade_level: grade, sort_order: sort, parent_topic_id: parent });
const topics: Topic[] = [tp('a', 1), tp('b', 2), tp('a1', 1, 'a'), tp('a2', 2, 'a'), tp('x', 1, undefined, 4)];
const los: LearningObjective[] = [{ objective_id: 'lo1', code: 'MATH.G3.NUM.ADD', description: 'Cộng', topic_id: 'a1' }];

describe('buildTree (F053-AC-26)', () => {
  it('subject → grade → nested topics → LO leaves, ordered by sort_order', () => {
    const tree = buildTree([{ subject_id: S, grade_level: 3 }, { subject_id: S, grade_level: 4 }], () => 'Toán', (g) => `Khối ${g}`, topics, los);
    expect(tree).toHaveLength(1);
    expect(tree[0]!.title).toBe('Toán');
    expect(tree[0]!.children!.map((g) => g.title)).toEqual(['Khối 3', 'Khối 4']);
    const g3 = tree[0]!.children![0]!;
    expect(g3.children!.map((n) => n.topic!.topic_id)).toEqual(['a', 'b']);
    const a = g3.children![0]!;
    expect(a.children!.map((n) => n.key)).toEqual(['topic:a1', 'topic:a2']);
    expect(a.children![0]!.children!.map((n) => n.kind)).toEqual(['objective']);
  });
});

describe('dropToPatch (F053-AC-27)', () => {
  it('before / after a sibling, inside a topic, at the end of a grade', () => {
    expect(dropToPatch(topics, { drag: topics[1]!, target: { kind: 'topic', topic: topics[0]! }, position: -1 })).toEqual({ parent_topic_id: null, sort_order: 1 });
    expect(dropToPatch(topics, { drag: topics[3]!, target: { kind: 'topic', topic: topics[1]! }, position: 0 })).toEqual({ parent_topic_id: 'b', sort_order: 1 });
    expect(dropToPatch(topics, { drag: topics[2]!, target: { kind: 'topic', topic: topics[3]! }, position: 1 })).toEqual({ parent_topic_id: 'a', sort_order: 2 });
    expect(dropToPatch(topics, { drag: topics[2]!, target: { kind: 'grade', subjectId: S, gradeLevel: 3 }, position: 0 })).toEqual({ parent_topic_id: null, sort_order: 3 });
  });

  it('refused: other grade, into its own subtree, no-op', () => {
    expect(dropToPatch(topics, { drag: topics[0]!, target: { kind: 'topic', topic: topics[4]! }, position: 0 })).toBeNull();
    expect(dropToPatch(topics, { drag: topics[0]!, target: { kind: 'topic', topic: topics[2]! }, position: 0 })).toBeNull();
    expect(dropToPatch(topics, { drag: topics[0]!, target: { kind: 'topic', topic: topics[1]! }, position: -1 })).toBeNull();
  });

  it('applyMove renumbers the siblings (optimistic display)', () => {
    const moved = applyMove(topics, 'b', { parent_topic_id: null, sort_order: 1 });
    const top = moved.filter((t) => !t.parent_topic_id && t.grade_level === 3).sort((x, y) => (x.sort_order ?? 0) - (y.sort_order ?? 0));
    expect(top.map((t) => t.topic_id)).toEqual(['b', 'a']);
  });
});
