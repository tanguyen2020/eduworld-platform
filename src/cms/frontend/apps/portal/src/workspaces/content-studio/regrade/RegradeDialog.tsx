// RegradeDialog (F058 T048, F058-AC-22…AC-25): "Chấm lại hệ thống" of a published (or replaced) question / activity
// version — action (accept_all / use_new_key / drop_item; an activity version supports accept_all only), corrected
// published version for use_new_key, reason 10–1000 characters, number of affected attempts from the server before
// confirming, then the request followed to completed / failed. Scope exam_session (F055-AC-23): `versionId` is the
// exam session and one question of its locked papers is picked from `questionOptions`. States plainly that original answers and old
// evaluations are kept and the new score is computed by the system with an audit record; no score field anywhere.
import { Alert, Descriptions, Flex, Input, Modal, Radio, Select, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useCreateRegrade, useRegradePreview, useRegradeStatus, type RegradeCreate, type RegradeScope } from './useRegrade';

/** RegradeRequestCreate.reason bounds (createRegradeRequest, F031-AC-18). */
export const REASON_MIN = 10;
export const REASON_MAX = 1000;

export interface RegradeDialogProps {
  open: boolean;
  scope: RegradeScope;
  versionId: string;
  /** Other published versions of the same question (use_new_key). */
  correctedOptions?: { value: string; label: string }[];
  /** scope exam_session: question versions of the session's papers (listExamSessionPapers). */
  questionOptions?: { value: string; label: string }[];
  onClose: () => void;
}

export function RegradeDialog({ open, scope, versionId, correctedOptions = [], questionOptions = [], onClose }: RegradeDialogProps) {
  const { t } = useTranslation();
  const exam = scope === 'exam_session';
  const actions: string[] = scope === 'activity_version' ? ['accept_all'] : exam && !correctedOptions.length ? ['accept_all', 'drop_item'] : ['use_new_key', 'accept_all', 'drop_item'];
  const [question, setQuestion] = useState<string | undefined>();
  const [action, setAction] = useState<string>(actions[0]!);
  const [corrected, setCorrected] = useState<string | undefined>();
  const [reason, setReason] = useState('');
  const [created, setCreated] = useState<string | undefined>();
  const valid = reason.trim().length >= REASON_MIN && reason.length <= REASON_MAX && (action !== 'use_new_key' || !!corrected) && (!exam || !!question);
  const body = valid
    ? ({
        scope,
        ...(exam ? { exam_session_id: versionId, question_version_id: question } : scope === 'question_version' ? { question_version_id: versionId } : { activity_version_id: versionId }),
        action,
        ...(action === 'use_new_key' ? { corrected_version_id: corrected } : {}),
        reason: reason.trim(),
      } as RegradeCreate)
    : null;
  const impact = useRegradePreview(open && !created ? body : null);
  const create = useCreateRegrade();
  const status = useRegradeStatus(created);
  const tooShort = reason !== '' && reason.trim().length < REASON_MIN;
  return (
    <Modal
      open={open}
      title={t('content_studio.regrade.title')}
      okText={created ? t('content_studio.regrade.close') : t('content_studio.regrade.confirm')}
      okButtonProps={{ disabled: !created && (!valid || !impact.data), loading: create.isPending }}
      cancelButtonProps={created ? { style: { display: 'none' } } : undefined}
      onCancel={onClose}
      onOk={() => {
        if (created) return onClose();
        if (body) create.mutate(body, { onSuccess: (r) => setCreated(r.regrade_request_id) });
      }}
      destroyOnHidden
    >
      <Flex vertical gap="middle" data-testid="regrade-dialog">
        <Alert type="info" showIcon message={t('content_studio.regrade.keeps')} />
        {!created && (
          <>
            {exam && <Select aria-label={t('content_studio.regrade.question')} placeholder={t('content_studio.regrade.question')} value={question} options={questionOptions} onChange={(v: string) => setQuestion(v)} />}
            <Radio.Group aria-label={t('content_studio.regrade.action')} value={action} onChange={(e) => setAction(e.target.value as string)}>
              {actions.map((a) => (
                <Radio key={a} value={a}>
                  {t(`content_studio.regrade.actions.${a}`)}
                </Radio>
              ))}
            </Radio.Group>
            {action === 'use_new_key' && <Select aria-label={t('content_studio.regrade.corrected')} placeholder={t('content_studio.regrade.corrected')} value={corrected} options={correctedOptions} onChange={(v: string) => setCorrected(v)} />}
            <Input.TextArea aria-label={t('content_studio.regrade.reason')} placeholder={t('content_studio.regrade.reason')} rows={3} maxLength={REASON_MAX} showCount value={reason} onChange={(e) => setReason(e.target.value)} status={tooShort ? 'error' : undefined} />
            {tooShort && <Typography.Text type="danger">{t('content_studio.regrade.reasonMin', { min: REASON_MIN })}</Typography.Text>}
            {impact.data && (
              <Alert
                type={impact.data.affected_attempts === 0 ? 'warning' : 'info'}
                showIcon
                data-testid="regrade-impact"
                message={impact.data.affected_attempts === 0 ? t('content_studio.regrade.noneAffected') : t('content_studio.regrade.affected', { n: impact.data.affected_attempts })}
              />
            )}
            {impact.error ? <ProblemMessage error={impact.error} /> : null}
            {create.error ? <ProblemMessage error={create.error} /> : null}
          </>
        )}
        {created && (
          <Descriptions size="small" column={1} data-testid="regrade-status" bordered>
            <Descriptions.Item label={t('content_studio.regrade.status')}>{status.data ? t(`content_studio.regrade.statuses.${status.data.status}`) : t('content_studio.loading')}</Descriptions.Item>
            <Descriptions.Item label={t('content_studio.regrade.affectedLabel')}>{status.data?.affected_attempts ?? '—'}</Descriptions.Item>
            <Descriptions.Item label={t('content_studio.regrade.doneLabel')}>{status.data?.re_evaluated_attempts ?? '—'}</Descriptions.Item>
          </Descriptions>
        )}
        {status.data?.status === 'failed' && <Alert type="error" showIcon message={t('content_studio.regrade.failed')} />}
      </Flex>
    </Modal>
  );
}
