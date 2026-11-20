// Submissions of one assignment (F055 T026; F055-AC-08, AC-09): listAssignmentSubmissions (cursor, status filter) —
// student, status, auto-graded score, attempts, submitted at, "làm quá nhanh" when rapid_guess_count > 0. No XP, rating
// or manual comment column (SCHOOL_LMS §8.2, ADR-0072). Per-row extend / excuse inside the catalog ActionGate; the row
// is updated from the AssignmentSubmission the server returns. Cards below 768 px (ResponsiveTable).
import { Button, DatePicker, Flex, Form, Input, Modal, Select, Space, Tag } from 'antd';
import type { ColumnType } from 'antd/es/table';
import dayjs, { type Dayjs } from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../../permissions/ActionGate';
import { formatDateTime } from '../../common/format';
import { ReasonModal } from '../../common/ReasonModal';
import { ResponsiveTable } from '../../common/ResponsiveTable';
import { useFieldRules } from '../../common/ui';
import { useStudentAction, useSubmissions, type Assignment, type AssignmentSubmission } from '../api/assignments';
import { LMS_ENUMS, LMS_FIELDS } from '../model/fieldSchemas';
import { LmsProblem, LmsTag } from '../LmsUi';

type SubStatus = AssignmentSubmission['status'];

function ExtendDialog({ assignment, row, onClose }: { assignment: Assignment; row: AssignmentSubmission; onClose: () => void }) {
  const { t } = useTranslation();
  const field = useFieldRules();
  const [form] = Form.useForm<{ dueAt: Dayjs; reason: string }>();
  const extend = useStudentAction(assignment.assignment_id, 'extend');
  return (
    <Modal
      open
      title={t('lms.submissions.extendTitle', { name: row.display_name ?? '' })}
      okText={t('lms.submissions.extend')}
      cancelText={t('portal.school_lms.common.cancel')}
      confirmLoading={extend.isPending}
      destroyOnHidden
      onCancel={onClose}
      onOk={async () => {
        const v = await form.validateFields().catch(() => null);
        if (v) extend.mutate({ studentId: row.student_id, dueAt: v.dueAt.toISOString(), reason: v.reason.trim() }, { onSuccess: onClose });
      }}
    >
      <Form form={form} layout="vertical" initialValues={{ dueAt: dayjs(row.due_at_override ?? assignment.due_at).add(1, 'day') }}>
        <Form.Item name="dueAt" label={t('lms.submissions.newDueAt')} rules={[{ required: true, message: t('lms.form.required') }]}>
          <DatePicker showTime aria-label={t('lms.submissions.newDueAt')} />
        </Form.Item>
        <Form.Item name="reason" label={t('portal.school_lms.common.reason')} rules={field.rules(LMS_FIELDS.extendReason, true)}>
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} {...field.input(LMS_FIELDS.extendReason)} />
        </Form.Item>
      </Form>
      <LmsProblem error={extend.error} />
    </Modal>
  );
}

function ExcuseDialog({ assignment, row, onClose }: { assignment: Assignment; row: AssignmentSubmission; onClose: () => void }) {
  const { t } = useTranslation();
  const excuse = useStudentAction(assignment.assignment_id, 'excuse');
  return (
    <ReasonModal
      open
      title={t('lms.submissions.excuseTitle', { name: row.display_name ?? '' })}
      okText={t('lms.submissions.excuse')}
      schema={LMS_FIELDS.reason}
      confirmLoading={excuse.isPending}
      onCancel={onClose}
      onSubmit={(reason) => excuse.mutate({ studentId: row.student_id, reason }, { onSuccess: onClose })}
    >
      {excuse.error ? <LmsProblem error={excuse.error} /> : null}
    </ReasonModal>
  );
}

export function SubmissionTable({ assignment }: { assignment: Assignment }) {
  const { t } = useTranslation();
  const [status, setStatus] = useState<SubStatus | undefined>();
  const subs = useSubmissions(assignment.assignment_id, status);
  const [dialog, setDialog] = useState<{ kind: 'extend' | 'excuse'; row: AssignmentSubmission } | null>(null);
  const live = assignment.status === 'open' || assignment.status === 'scheduled' || assignment.status === 'closed';
  const actions = (s: AssignmentSubmission) =>
    live && s.status !== 'excused' && s.status !== 'submitted' ? (
      <ActionGate actionKey="action.school_lms.assignments.manage_students">
        <Space wrap>
          <Button size="small" onClick={() => setDialog({ kind: 'extend', row: s })}>
            {t('lms.submissions.extend')}
          </Button>
          <Button size="small" onClick={() => setDialog({ kind: 'excuse', row: s })}>
            {t('lms.submissions.excuse')}
          </Button>
        </Space>
      </ActionGate>
    ) : null;
  const columns: ColumnType<AssignmentSubmission>[] = [
    { key: 'student', title: t('lms.submissions.student'), render: (_, s) => s.display_name },
    { key: 'status', title: t('lms.assignment.status'), render: (_, s) => <LmsTag group="submissionStatus" value={s.status} /> },
    { key: 'score', title: t('lms.submissions.score'), render: (_, s) => (s.score == null ? '' : `${s.score}${s.max_score == null ? '' : `/${s.max_score}`}`) },
    { key: 'attempts', title: t('lms.submissions.attempts'), render: (_, s) => s.attempts_used ?? 0 },
    { key: 'submitted', title: t('lms.submissions.submittedAt'), render: (_, s) => formatDateTime(s.submitted_at) },
    { key: 'due', title: t('lms.submissions.dueOverride'), render: (_, s) => formatDateTime(s.due_at_override) },
    {
      key: 'flags',
      title: t('lms.submissions.flags'),
      render: (_, s) => ((s.rapid_guess_count ?? 0) > 0 ? <Tag data-testid="rapid-guess">{t('lms.submissions.rapidGuess')}</Tag> : null),
    },
    { key: 'actions', title: t('portal.school_lms.common.actions'), render: (_, s) => actions(s) },
  ];
  return (
    <Flex vertical gap="small" data-testid="submission-table">
      <Select
        allowClear
        aria-label={t('lms.submissions.filter')}
        placeholder={t('lms.submissions.filter')}
        value={status}
        onChange={setStatus}
        options={LMS_ENUMS.submissionStatus.values.map((s) => ({ value: s, label: t(`lms.submissionStatus.${s}`) }))}
      />
      <LmsProblem error={subs.error} />
      <ResponsiveTable<AssignmentSubmission>
        rowKey="student_id"
        columns={columns}
        data={subs.items}
        loading={subs.isFetching}
        cardTitle={(s) => s.display_name}
        hasMore={subs.hasMore}
        onLoadMore={subs.loadMore}
      />
      {dialog?.kind === 'extend' && <ExtendDialog assignment={assignment} row={dialog.row} onClose={() => setDialog(null)} />}
      {dialog?.kind === 'excuse' && <ExcuseDialog assignment={assignment} row={dialog.row} onClose={() => setDialog(null)} />}
    </Flex>
  );
}
