// Learning paths of a course (F055 T031; F055-AC-14, AC-15): list (template paths read-only), builder of a path —
// title, unlock mode, ordered nodes of published content (content_version_id from the picker; keyboard up/down, no
// drag-only interaction), per-node unlock rule; save = createLearningPath (Idempotency-Key) or updateLearningPath
// (If-Match = path version). Manual unlock of a mastery-gate node for one student: reason + Idempotency-Key.
import { ArrowDownOutlined, ArrowUpOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Flex, Form, Input, List, Modal, Select, Space, Tag, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useParams } from 'react-router-dom';
import { ActionGate } from '../../../../permissions/ActionGate';
import { useClassStudents } from '../../classes/useClasses';
import { ReasonModal } from '../../common/ReasonModal';
import { useFieldRules } from '../../common/ui';
import { useCourses, useLearningPaths, useSaveLearningPath, useUnlockNode, type Course, type LearningPath, type LearningPathInput } from '../api/courses';
import { courseLabel } from '../assignments/wizard/StepTargets';
import { ContentPicker } from '../assignments/wizard/StepContent';
import type { ContentChoice } from '../model/assignmentForm';
import { LMS_ENUMS, LMS_FIELDS, LMS_LIMITS } from '../model/fieldSchemas';
import { LmsProblem } from '../LmsUi';

type Node = LearningPathInput['nodes'][number] & { title?: string };
type Kind = NonNullable<NonNullable<Node['unlock_rule']>['kind']>;
const MASTERY = ['beginning', 'developing', 'proficient', 'mastered'] as const;

/** Node sent to the API: position, content_ref, unlock_rule (no display title; node_id is read-only). */
const toNodeInput = (n: Node): LearningPathInput['nodes'][number] => ({ position: n.position, content_ref: n.content_ref, ...(n.unlock_rule ? { unlock_rule: n.unlock_rule } : {}) });

function UnlockNodeDialog({ course, path, node, onClose }: { course: Course; path: LearningPath; node: Node; onClose: () => void }) {
  const { t } = useTranslation();
  const students = useClassStudents(course.school_id, course.class_id);
  const unlock = useUnlockNode(course.course_id, path.path_id);
  const [student, setStudent] = useState<string | undefined>();
  const [done, setDone] = useState(false);
  return (
    <ReasonModal
      open
      title={t('lms.path.unlockTitle', { position: node.position })}
      okText={t('lms.path.unlock')}
      schema={LMS_FIELDS.reason}
      confirmLoading={unlock.isPending}
      onCancel={onClose}
      onSubmit={(reason) => student && unlock.mutate({ nodeId: node.node_id!, studentId: student, reason }, { onSuccess: () => setDone(true) })}
    >
      <Select
        showSearch
        optionFilterProp="label"
        aria-label={t('lms.submissions.student')}
        placeholder={t('lms.submissions.student')}
        value={student}
        onChange={setStudent}
        options={students.items.map((s) => ({ value: s.student_id, label: s.display_name }))}
      />
      {!student ? <Typography.Text type="secondary">{t('lms.path.pickStudent')}</Typography.Text> : null}
      {done ? <Alert type="success" showIcon role="status" message={t('lms.path.unlocked')} /> : null}
      <LmsProblem error={unlock.error} />
    </ReasonModal>
  );
}

function PathBuilder({ course, path, onDone }: { course: Course; path?: LearningPath; onDone: () => void }) {
  const { t } = useTranslation();
  const field = useFieldRules();
  const readOnly = !!path?.is_template;
  const [title, setTitle] = useState(path?.title ?? '');
  const [mode, setMode] = useState<LearningPathInput['unlock_mode']>(path?.unlock_mode ?? 'sequential');
  const [nodes, setNodes] = useState<Node[]>(path?.nodes ?? []);
  const [picking, setPicking] = useState(false);
  const [unlocking, setUnlocking] = useState<Node | null>(null);
  const save = useSaveLearningPath(course.course_id);
  const renumber = (list: Node[]) => list.map((n, i) => ({ ...n, position: i + 1 }));
  const move = (i: number, d: number) => {
    const next = [...nodes];
    const [n] = next.splice(i, 1);
    next.splice(i + d, 0, n!);
    setNodes(renumber(next));
  };
  const add = (c: ContentChoice) => {
    if (!c.versionId) return;
    setNodes(renumber([...nodes, { position: 0, content_ref: { content_type: c.contentType as Node['content_ref']['content_type'], content_version_id: c.versionId }, title: c.title }]));
    setPicking(false);
  };
  const setRule = (i: number, kind: Kind, min?: string) =>
    setNodes(nodes.map((n, j) => (j === i ? { ...n, unlock_rule: { kind, ...(kind === 'mastery_gate' ? { min_mastery: (min ?? 'proficient') as never } : {}) } } : n)));
  const titleOk = title.trim().length >= LMS_FIELDS.pathTitle.minLength && title.length <= LMS_FIELDS.pathTitle.maxLength;
  const countOk = nodes.length >= LMS_LIMITS.pathNodes.minItems && nodes.length <= LMS_LIMITS.pathNodes.maxItems;
  return (
    <Card size="small" data-testid="path-builder" title={path ? path.title : t('lms.path.new')}>
      <Flex vertical gap="small">
        {readOnly ? <Alert type="info" showIcon message={t('lms.path.templateReadOnly')} data-testid="template-readonly" /> : null}
        <Form layout="vertical" component="div">
          <Form.Item label={t('lms.path.title')} htmlFor="path-title" validateStatus={title && !titleOk ? 'error' : undefined}>
            <Input id="path-title" disabled={readOnly} value={title} onChange={(e) => setTitle(e.target.value)} {...field.input(LMS_FIELDS.pathTitle)} />
          </Form.Item>
          <Form.Item label={t('lms.path.unlockMode')}>
            <Select
              disabled={readOnly}
              aria-label={t('lms.path.unlockMode')}
              value={mode}
              onChange={setMode}
              options={LMS_ENUMS.unlockMode.values.map((m) => ({ value: m, label: t(`lms.unlockMode.${m}`) }))}
            />
          </Form.Item>
        </Form>
        <List
          bordered
          dataSource={nodes}
          locale={{ emptyText: t('lms.path.noNodes') }}
          renderItem={(n, i) => (
            <List.Item
              actions={
                readOnly
                  ? []
                  : [
                      <Button key="up" size="small" disabled={i === 0} aria-label={t('lms.path.moveUp')} icon={<ArrowUpOutlined />} onClick={() => move(i, -1)} />,
                      <Button key="down" size="small" disabled={i === nodes.length - 1} aria-label={t('lms.path.moveDown')} icon={<ArrowDownOutlined />} onClick={() => move(i, 1)} />,
                      <Button key="rm" size="small" onClick={() => setNodes(renumber(nodes.filter((_, j) => j !== i)))}>
                        {t('portal.school_lms.common.remove')}
                      </Button>,
                    ]
              }
            >
              <Space wrap>
                <Typography.Text>{`${n.position}. ${n.title ?? n.content_ref.content_version_id}`}</Typography.Text>
                <Tag>{t(`lms.contentType.${n.content_ref.content_type}`)}</Tag>
                <Select
                  size="small"
                  disabled={readOnly}
                  aria-label={t('lms.path.unlockRule')}
                  value={n.unlock_rule?.kind ?? 'previous_completed'}
                  onChange={(k: Kind) => setRule(i, k)}
                  options={LMS_ENUMS.unlockRule.values.filter((k) => k !== 'date').map((k) => ({ value: k, label: t(`lms.unlockRule.${k}`) }))}
                />
                {n.unlock_rule?.kind === 'mastery_gate' ? (
                  <Select
                    size="small"
                    disabled={readOnly}
                    aria-label={t('lms.path.minMastery')}
                    value={n.unlock_rule.min_mastery ?? 'proficient'}
                    onChange={(m: string) => setRule(i, 'mastery_gate', m)}
                    options={MASTERY.map((m) => ({ value: m, label: t(`lms.mastery.${m}`) }))}
                  />
                ) : null}
                {path && !path.is_template && n.node_id && n.unlock_rule?.kind === 'mastery_gate' ? (
                  <ActionGate actionKey="action.school_lms.courses.unlock_node">
                    <Button size="small" onClick={() => setUnlocking(n)}>
                      {t('lms.path.unlock')}
                    </Button>
                  </ActionGate>
                ) : null}
              </Space>
            </List.Item>
          )}
        />
        {!readOnly && (
          <ActionGate actionKey="action.school_lms.courses.edit_paths">
            <Flex vertical gap="small">
              <Space wrap>
                <Button onClick={() => setPicking(true)} disabled={nodes.length >= LMS_LIMITS.pathNodes.maxItems}>
                  {t('lms.path.addNode')}
                </Button>
                <Button
                  type="primary"
                  disabled={!titleOk || !countOk}
                  loading={save.isPending}
                  onClick={() =>
                    save.mutate(
                      { pathId: path?.path_id, version: path?.version, body: { title: title.trim(), unlock_mode: mode, nodes: nodes.map(toNodeInput) } },
                      { onSuccess: onDone },
                    )
                  }
                >
                  {t('portal.school_lms.common.save')}
                </Button>
              </Space>
              <Typography.Text type="secondary">{t('lms.path.nodeCount', { n: nodes.length, max: LMS_LIMITS.pathNodes.maxItems })}</Typography.Text>
            </Flex>
          </ActionGate>
        )}
        <LmsProblem error={save.error} />
      </Flex>
      <Modal open={picking} title={t('lms.path.addNode')} footer={null} onCancel={() => setPicking(false)} destroyOnHidden>
        <ContentPicker onChange={add} tabs={['lesson', 'activity', 'question_set', 'assessment_blueprint', 'game']} questsOnly />
      </Modal>
      {unlocking && path ? <UnlockNodeDialog course={course} path={path} node={unlocking} onClose={() => setUnlocking(null)} /> : null}
    </Card>
  );
}

export function CourseDetailPage() {
  const { t, i18n } = useTranslation();
  const { courseId } = useParams();
  const courses = useCourses();
  const course = courses.items.find((c) => c.course_id === courseId);
  const paths = useLearningPaths(courseId);
  const [editing, setEditing] = useState<string | 'new' | null>(null);
  return (
    <Flex vertical gap="middle" data-testid="course-detail">
      <Link to="..">{t('portal.school_lms.common.back')}</Link>
      {course ? <Typography.Title level={2}>{courseLabel(course, i18n.language)}</Typography.Title> : null}
      <LmsProblem error={paths.error} />
      <List
        header={<Typography.Text strong>{t('lms.path.list')}</Typography.Text>}
        dataSource={paths.data?.data ?? []}
        locale={{ emptyText: t('lms.path.none') }}
        renderItem={(p) => (
          <List.Item actions={[<Button key="open" size="small" onClick={() => setEditing(p.path_id)}>{p.is_template ? t('lms.path.view') : t('portal.school_lms.common.edit')}</Button>]}>
            <Space>
              <Typography.Text>{p.title}</Typography.Text>
              {p.is_template ? <Tag>{t('lms.path.template')}</Tag> : null}
              <Tag>{t(`lms.unlockMode.${p.unlock_mode}`)}</Tag>
            </Space>
          </List.Item>
        )}
      />
      {course ? (
        <ActionGate actionKey="action.school_lms.courses.edit_paths">
          <Button onClick={() => setEditing('new')}>{t('lms.path.new')}</Button>
        </ActionGate>
      ) : null}
      {course && editing ? (
        <PathBuilder key={editing} course={course} path={editing === 'new' ? undefined : paths.data?.data.find((p) => p.path_id === editing)} onDone={() => setEditing(null)} />
      ) : null}
    </Flex>
  );
}
