// TopicTree (F053-AC-26, AC-27): Môn → môn × khối → chủ đề (nested) → LO. With `curriculum.write` on an editable
// (draft) version, topics are dragged to a new place: the move is shown at once and sent as `updateTopic`
// (`parent_topic_id`, `sort_order`, If-Match); if the server refuses, the tree goes back and the error is shown.
// Ant Design Tree drag-and-drop [Proposed instead of @dnd-kit]: same result, no extra dependency.
import { Tree, Typography, type TreeDataNode, type TreeProps } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { applyMove, buildTree, dropToPatch, type SubjectGradeRef, type TreeNode } from './treeModel';
import type { LearningObjective, Topic, TopicPatch } from './useCurriculum';

export interface TopicTreeProps {
  subjectGrades: readonly SubjectGradeRef[];
  subjectName: (id: string) => string;
  topics: readonly Topic[];
  objectives: readonly LearningObjective[];
  editable: boolean;
  /** Sends `updateTopic` (the page passes the API call). */
  onMove: (topic: Topic, patch: TopicPatch) => Promise<unknown>;
  onSelect?: (node: TreeNode) => void;
}

const toData = (nodes: TreeNode[]): (TreeDataNode & { node: TreeNode })[] =>
  nodes.map((n) => ({ key: n.key, title: n.title, node: n, isLeaf: n.kind === 'objective', children: n.children ? toData(n.children) : undefined }));

/** Optimistic topic moves: shown at once, reverted with the error when the server refuses (F053-AC-27). */
export function useOptimisticTopics(topics: readonly Topic[], onMove: TopicTreeProps['onMove']) {
  const [local, setLocal] = useState<readonly Topic[]>(topics);
  const [error, setError] = useState<unknown>(null);
  useEffect(() => setLocal(topics), [topics]);
  const move = (topic: Topic, patch: { parent_topic_id: string | null; sort_order: number }) => {
    const before = local;
    setError(null);
    setLocal(applyMove(local, topic.topic_id, patch));
    return onMove(topic, patch).catch((e: unknown) => {
      setLocal(before); // the server refused: back to the previous tree
      setError(e);
    });
  };
  return { local, error, move };
}

export function TopicTree({ subjectGrades, subjectName, topics, objectives, editable, onMove, onSelect }: TopicTreeProps) {
  const { t } = useTranslation();
  const { local, error, move } = useOptimisticTopics(topics, onMove);
  const tree = useMemo(() => buildTree(subjectGrades, subjectName, (g) => t('content_studio.curriculum.grade', { grade: g }), local, objectives), [subjectGrades, subjectName, local, objectives, t]);
  const data = useMemo(() => toData(tree), [tree]);
  // every node expanded, including nodes that arrive after the first render (topics / LOs load after subjects)
  const allKeys = useMemo(() => {
    const out: string[] = [];
    const walk = (ns: TreeNode[]) => ns.forEach((n) => (out.push(n.key), n.children && walk(n.children)));
    walk(tree);
    return out;
  }, [tree]);
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set());

  const onDrop: TreeProps['onDrop'] = (info) => {
    const drag = (info.dragNode as unknown as { node: TreeNode }).node;
    const target = (info.node as unknown as { node: TreeNode }).node;
    if (drag.kind !== 'topic' || !drag.topic) return;
    const rel = info.dropPosition - Number(String(info.node.pos).split('-').pop());
    const position: -1 | 0 | 1 = !info.dropToGap ? 0 : rel < 0 ? -1 : 1;
    const patch =
      target.kind === 'topic' && target.topic
        ? dropToPatch(local, { drag: drag.topic, target: { kind: 'topic', topic: target.topic }, position })
        : target.kind === 'grade' && target.gradeLevel !== undefined
          ? dropToPatch(local, { drag: drag.topic, target: { kind: 'grade', subjectId: target.subjectId, gradeLevel: target.gradeLevel }, position: 0 })
          : null;
    if (!patch) return;
    void move(drag.topic, patch);
  };

  return (
    <div>
      {error ? <ProblemMessage error={error} /> : null}
      {data.length === 0 ? (
        <Typography.Text type="secondary">{t('content_studio.curriculum.empty')}</Typography.Text>
      ) : (
        <Tree
          aria-label={t('content_studio.curriculum.tree')}
          treeData={data}
          expandedKeys={allKeys.filter((k) => !collapsed.has(k))}
          onExpand={(keys) => setCollapsed(new Set(allKeys.filter((k) => !keys.includes(k))))}
          blockNode
          draggable={editable ? { icon: false, nodeDraggable: (n) => (n as unknown as { node: TreeNode }).node.kind === 'topic' } : false}
          allowDrop={({ dropNode }) => (dropNode as unknown as { node: TreeNode }).node.kind !== 'objective' && (dropNode as unknown as { node: TreeNode }).node.kind !== 'subject'}
          onDrop={onDrop}
          onSelect={(_, e) => onSelect?.((e.node as unknown as { node: TreeNode }).node)}
        />
      )}
    </div>
  );
}
