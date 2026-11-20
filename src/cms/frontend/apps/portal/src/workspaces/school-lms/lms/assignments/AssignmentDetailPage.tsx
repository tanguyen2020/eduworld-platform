// Assignment detail (F055 T026, T027; F055-AC-08…AC-12, AC-18, AC-23): summary, actions by status and permission
// (catalog ActionGate) — Sửa (updateAssignment, If-Match; open → due_at/instructions/late_policy only; stale → reload),
// Nhắc (remindAssignment; 429 → "đã nhắc hôm nay", locked until Retry-After), Công bố kết quả (manual + closed,
// lms.result.release), Hủy (reason; 409 / 422 shown), Lưu trữ; tabs Nộp bài, Hỗ trợ thời gian (exam session),
// Chấm lại đề (learning_content.re_evaluate). Every decision stays on the server.
import { Button, Descriptions, Flex, Modal, Space, Tabs, Typography } from 'antd';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useParams } from 'react-router-dom';
import { ActionGate } from '../../../../permissions/ActionGate';
import { ApiError } from '../../../../shared/api/middleware/problem';
import { NotFoundResult, isHiddenResource } from '../../common/errors';
import { formatDateTime } from '../../common/format';
import { ReasonModal } from '../../common/ReasonModal';
import { useAssignment, useAssignmentAction, useRemindAssignment, type Assignment } from '../api/assignments';
import { AccommodationsTab } from '../exams/AccommodationsTab';
import { ExamRegradeButton } from '../exams/ExamRegradeButton';
import { LMS_FIELDS } from '../model/fieldSchemas';
import { LmsProblem, LmsTag, useHasPermission } from '../LmsUi';
import { AssignmentEditDialog } from './AssignmentEditDialog';
import { SubmissionTable } from './SubmissionTable';

/** Remind once a day (assignment_remind_limit): a 429 locks the button until Retry-After (F055-AC-10). */
function RemindButton({ a }: { a: Assignment }) {
  const { t } = useTranslation();
  const remind = useRemindAssignment(a.assignment_id);
  const [lockedUntil, setLockedUntil] = useState(0);
  const [sent, setSent] = useState<number | null>(null);
  useEffect(() => {
    if (!lockedUntil) return;
    const id = setTimeout(() => setLockedUntil(0), Math.max(0, lockedUntil - Date.now()));
    return () => clearTimeout(id);
  }, [lockedUntil]);
  const limited = remind.error instanceof ApiError && remind.error.code === 'RATE_LIMITED';
  return (
    <Flex vertical gap="small">
      <Button
        disabled={lockedUntil > 0}
        loading={remind.isPending}
        onClick={() =>
          remind.mutate(
            { at: a.assignment_id },
            {
              onSuccess: (r) => setSent(r.recipients),
              onError: (e) => {
                if (e instanceof ApiError && e.code === 'RATE_LIMITED') setLockedUntil(Date.now() + (e.retryAfterMs ?? 24 * 3600 * 1000));
              },
            },
          )
        }
      >
        {t('lms.assignment.remind')}
      </Button>
      {limited ? (
        <Typography.Text type="warning" role="status" data-testid="remind-limited">
          {t('lms.assignment.remindLimited')}
        </Typography.Text>
      ) : remind.error ? (
        <LmsProblem error={remind.error} />
      ) : sent != null ? (
        <Typography.Text role="status">{t('lms.assignment.reminded', { n: sent })}</Typography.Text>
      ) : null}
    </Flex>
  );
}

function Actions({ a, onEdit }: { a: Assignment; onEdit: () => void }) {
  const { t } = useTranslation();
  const cancel = useAssignmentAction(a.assignment_id, 'cancel');
  const release = useAssignmentAction(a.assignment_id, 'release-results');
  const archive = useAssignmentAction(a.assignment_id, 'archive');
  const [cancelOpen, setCancelOpen] = useState(false);
  const mayRelease = useHasPermission('lms.result.release');
  const canRelease = a.status === 'closed' && a.result_release === 'manual' && mayRelease;
  const editable = a.status === 'draft' || a.status === 'scheduled' || a.status === 'open';
  return (
    <Flex vertical gap="small">
      <Space wrap>
        {editable && (
          <ActionGate actionKey="action.school_lms.assignments.edit">
            <Button onClick={onEdit}>{t('portal.school_lms.common.edit')}</Button>
          </ActionGate>
        )}
        {canRelease && (
          <ActionGate actionKey="action.school_lms.assignments.release_results">
            <Button
              type="primary"
              loading={release.isPending}
              onClick={() =>
                Modal.confirm({
                  title: t('lms.assignment.releaseConfirm'),
                  okText: t('lms.assignment.release'),
                  cancelText: t('portal.school_lms.common.cancel'),
                  onOk: () => release.mutate({}),
                })
              }
            >
              {t('lms.assignment.release')}
            </Button>
          </ActionGate>
        )}
        {(a.status === 'scheduled' || a.status === 'open') && (
          <ActionGate actionKey="action.school_lms.assignments.edit">
            <Button danger onClick={() => setCancelOpen(true)}>
              {t('lms.assignment.cancel')}
            </Button>
          </ActionGate>
        )}
        {(a.status === 'closed' || a.status === 'results_released') && (
          <ActionGate actionKey="action.school_lms.assignments.edit">
            <Button loading={archive.isPending} onClick={() => archive.mutate({})}>
              {t('lms.assignment.archive')}
            </Button>
          </ActionGate>
        )}
      </Space>
      {a.status === 'open' && (
        <ActionGate actionKey="action.school_lms.assignments.manage_students">
          <RemindButton a={a} />
        </ActionGate>
      )}
      <LmsProblem error={cancel.error ?? release.error ?? archive.error} testId="action-error" />
      <ReasonModal
        open={cancelOpen}
        title={t('lms.assignment.cancelTitle')}
        okText={t('lms.assignment.cancel')}
        danger
        schema={LMS_FIELDS.reason}
        confirmLoading={cancel.isPending}
        onCancel={() => setCancelOpen(false)}
        onSubmit={(reason) => {
          setCancelOpen(false);
          cancel.mutate({ reason });
        }}
      />
    </Flex>
  );
}

export function AssignmentDetailPage() {
  const { t } = useTranslation();
  const { assignmentId } = useParams();
  const q = useAssignment(assignmentId);
  const [editing, setEditing] = useState(false);
  const canWrite = useHasPermission('lms.assignment.write');
  if (q.error) return isHiddenResource(q.error) ? <NotFoundResult /> : <LmsProblem error={q.error} />;
  const a = q.data;
  if (!a) return null;
  const tabs = [{ key: 'submissions', label: t('lms.submissions.tab'), children: <SubmissionTable assignment={a} /> }];
  if (a.exam_session_id && canWrite) tabs.push({ key: 'accommodations', label: t('lms.accommodations.tab'), children: <AccommodationsTab assignment={a} /> });
  return (
    <Flex vertical gap="middle" data-testid="assignment-detail">
      <Link to="..">{t('portal.school_lms.common.back')}</Link>
      <Typography.Title level={2}>{a.title}</Typography.Title>
      <Descriptions bordered size="small" column={{ xs: 1, md: 2 }}>
        <Descriptions.Item label={t('lms.assignment.status')}>
          <LmsTag group="assignmentStatus" value={a.status} />
        </Descriptions.Item>
        <Descriptions.Item label={t('lms.wizard.mode')}>{t(`lms.mode.${a.mode}`)}</Descriptions.Item>
        <Descriptions.Item label={t('lms.wizard.openAt')}>{formatDateTime(a.open_at)}</Descriptions.Item>
        <Descriptions.Item label={t('lms.wizard.dueAt')}>{formatDateTime(a.due_at)}</Descriptions.Item>
        <Descriptions.Item label={t('lms.wizard.resultRelease')}>{a.result_release ? t(`lms.resultRelease.${a.result_release}`) : ''}</Descriptions.Item>
        <Descriptions.Item label={t('lms.assignment.completion')}>
          {a.completion ? t('lms.assignment.completionText', { submitted: a.completion.submitted ?? 0, assigned: a.completion.assigned ?? 0, missed: a.completion.missed ?? 0 }) : ''}
        </Descriptions.Item>
      </Descriptions>
      <Actions a={a} onEdit={() => setEditing(true)} />
      {a.exam_session_id && (a.status === 'closed' || a.status === 'results_released') ? <ExamRegradeButton examSessionId={a.exam_session_id} /> : null}
      <Tabs items={tabs} />
      {editing && <AssignmentEditDialog assignment={a} onClose={() => setEditing(false)} onStale={() => void q.refetch()} />}
    </Flex>
  );
}
