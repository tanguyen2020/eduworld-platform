// "Hỗ trợ thời gian" of an assignment with an exam session (F055 T037, F055-AC-18, FR-EXM-009): personal extra time
// per student of the assignment's audience (students from listAssignmentSubmissions), percent within the
// ExamAccommodation limits, saved with putExamAccommodations (delta F055-D2: reason, Idempotency-Key, If-Match).
// The server recomputes the deadline of each attempt; nothing is computed here.
import { Button, Flex, InputNumber, Table, Typography } from 'antd';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../../permissions/ActionGate';
import { ReasonModal } from '../../common/ReasonModal';
import { useSubmissions, type Assignment } from '../api/assignments';
import { useExamSession, usePutAccommodations, type ExamAccommodation } from '../api/exams';
import { LMS_FIELDS, LMS_LIMITS } from '../model/fieldSchemas';
import { LmsProblem } from '../LmsUi';
import { SearchSelect } from '@eduworld/ops-ui';

export function AccommodationsTab({ assignment }: { assignment: Assignment }) {
  const { t } = useTranslation();
  const id = assignment.exam_session_id!;
  const session = useExamSession(id);
  const students = useSubmissions(assignment.assignment_id);
  const save = usePutAccommodations(id);
  const [rows, setRows] = useState<ExamAccommodation[]>([]);
  const [student, setStudent] = useState<string | undefined>();
  const [percent, setPercent] = useState<number | null>(25);
  const [confirm, setConfirm] = useState(false);
  useEffect(() => setRows(session.data?.data.accommodations ?? []), [session.data]);
  const nameOf = (sid: string) => students.items.find((s) => s.student_id === sid)?.display_name ?? sid;
  const lim = LMS_LIMITS.extraTimePercent;
  const canAdd = !!student && percent != null && percent >= lim.minimum && percent <= lim.maximum && rows.length < LMS_LIMITS.accommodations.maxItems;
  return (
    <Flex vertical gap="middle" data-testid="accommodations">
      <Typography.Paragraph type="secondary">{t('lms.accommodations.help')}</Typography.Paragraph>
      <LmsProblem error={session.error ?? save.error} />
      <Table<ExamAccommodation>
        rowKey="student_id"
        size="small"
        pagination={false}
        dataSource={rows}
        columns={[
          { key: 's', title: t('lms.submissions.student'), render: (_, r) => nameOf(r.student_id) },
          { key: 'p', title: t('lms.accommodations.percent'), render: (_, r) => t('lms.accommodations.percentValue', { n: r.extra_time_percent }) },
          {
            key: 'x',
            title: t('portal.school_lms.common.actions'),
            render: (_, r) => (
              <Button size="small" onClick={() => setRows(rows.filter((x) => x.student_id !== r.student_id))}>
                {t('portal.school_lms.common.remove')}
              </Button>
            ),
          },
        ]}
      />
      <Flex wrap="wrap" gap="small" align="center">
        <SearchSelect
          showSearch
          optionFilterProp="label"
          aria-label={t('lms.submissions.student')}
          placeholder={t('lms.submissions.student')}
          value={student}
          onChange={setStudent}
          options={students.items.filter((s) => !rows.some((r) => r.student_id === s.student_id)).map((s) => ({ value: s.student_id, label: s.display_name ?? s.student_id }))}
        />
        <InputNumber aria-label={t('lms.accommodations.percent')} min={lim.minimum} max={lim.maximum} precision={0} value={percent} onChange={setPercent} />
        <Button
          disabled={!canAdd}
          onClick={() => {
            setRows([...rows, { student_id: student!, extra_time_percent: percent! }]);
            setStudent(undefined);
          }}
        >
          {t('lms.accommodations.add')}
        </Button>
      </Flex>
      <ActionGate actionKey="action.school_lms.assignments.manage_students">
        <Button type="primary" loading={save.isPending} onClick={() => setConfirm(true)}>
          {t('portal.school_lms.common.save')}
        </Button>
      </ActionGate>
      <ReasonModal
        open={confirm}
        title={t('lms.accommodations.saveTitle')}
        schema={LMS_FIELDS.accommodationReason}
        confirmLoading={save.isPending}
        onCancel={() => setConfirm(false)}
        onSubmit={(reason) => {
          setConfirm(false);
          save.mutate({ accommodations: rows, reason, etag: session.data?.etag ?? null });
        }}
      />
    </Flex>
  );
}
