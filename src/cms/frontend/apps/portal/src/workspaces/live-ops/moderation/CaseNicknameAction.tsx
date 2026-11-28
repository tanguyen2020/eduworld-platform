// Nickname of a moderation case (F136 T036, F136-AC-22, F136-AC-19): ReportedNicknames shows the server snapshot of a
// reported nickname (getModerationCase evidence[].reported_nickname) as plain text — React text nodes only, never HTML;
// CaseNicknameAction is the case action "Đặt lại nickname" = actOnModerationCase `action = nickname_reset` (immediate
// sanction; module game sets the nickname to null through sanction.applied) with the target student of the case, a
// mandatory note (3–1000) and the same mechanism as the other case actions: If-Match = case version, X-Tenant-Id = tenant
// of the case, one Idempotency-Key per intent, 412 → reloaded message, no automatic retry. Shown where the other case
// actions are (ActionGate action.live_ops.moderation_cases.act); display only — module chat decides (AGENTS R1, R7, R20).
import { Alert, App as AntApp, Button, Card, Descriptions, Form, Input, Space, Typography } from 'antd';
import { openapiInputProps, openapiRules, OPS_UI_NS, SearchSelect } from '@eduworld/ops-ui';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ApiError } from '../../../shared/api/middleware/problem';
import { LIMITS } from '../../../shared/chatSafety/ui';
import { useActOnCase, type ModerationCaseDetail } from './api';
import { formatTime } from './CasesScreen';

/** Translated message of a case write error (own text for 412 / VERSION_MISMATCH, F122-AC-04). */
export function caseWriteErrorMessage(error: unknown, t: (k: string) => string): string {
  if (!(error instanceof ApiError)) return t('error.generic');
  if (error.code === 'VERSION_MISMATCH' || error.status === 412) return t('chatSafety.case.reloaded');
  return t(error.i18nKey);
}

/** Error of a case write: message by code + trace id. */
export function CaseWriteError({ error }: { error: unknown }) {
  const { t } = useTranslation();
  if (!error) return null;
  return (
    <Alert
      type="error"
      showIcon
      role="alert"
      message={caseWriteErrorMessage(error, t)}
      description={error instanceof ApiError && error.traceId ? t('shell.problem.traceId', { traceId: error.traceId }) : undefined}
    />
  );
}

/** Reported nickname snapshots of the case (plain text), nothing when the case has none. */
export function ReportedNicknames({ c }: { c: ModerationCaseDetail }) {
  const { t } = useTranslation();
  const items = (c.evidence ?? []).filter((e) => !!e.reported_nickname);
  if (!items.length) return null;
  return (
    <Card size="small" data-testid="reported-nicknames">
      <Descriptions size="small" column={1}>
        {items.map((e) => (
          <Descriptions.Item key={e.evidence_id} label={t('chatSafety.case.reportedNickname')}>
            <Space wrap>
              <Typography.Text strong>{e.reported_nickname}</Typography.Text>
              <Typography.Text type="secondary">{formatTime(e.created_at)}</Typography.Text>
            </Space>
          </Descriptions.Item>
        ))}
      </Descriptions>
    </Card>
  );
}

interface NicknameForm {
  target_user_id?: string;
  note: string;
}

function NicknameResetCard({ c }: { c: ModerationCaseDetail }) {
  const { t } = useTranslation();
  const ops = useTranslation(OPS_UI_NS);
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<NicknameForm>();
  const act = useActOnCase(c);
  const subjects = c.subject_user_ids ?? [];
  const submit = (v: NicknameForm) => {
    act.mutate(
      { action: 'nickname_reset', ...(v.target_user_id ? { target_user_id: v.target_user_id } : {}), note: v.note.trim() },
      {
        onSuccess: () => {
          message.success(t('chatSafety.case.nicknameResetDone'));
          form.resetFields();
        },
      },
    );
  };
  return (
    <Card title={t('chatSafety.case.nicknameReset')} size="small" data-testid="case-nickname-action">
      <Form form={form} layout="vertical" name="case_nickname_reset" onFinish={submit} initialValues={subjects.length === 1 ? { target_user_id: subjects[0] } : {}}>
        <Typography.Paragraph type="secondary">{t('chatSafety.case.nicknameResetHint')}</Typography.Paragraph>
        {subjects.length ? (
          <Form.Item name="target_user_id" label={t('chatSafety.case.nicknameTarget')} rules={[{ required: true, message: t('chatSafety.case.nicknameTargetRequired') }]}>
            <SearchSelect aria-label={t('chatSafety.case.nicknameTarget')} options={subjects.map((id) => ({ value: id, label: id }))} />
          </Form.Item>
        ) : null}
        <Form.Item name="note" label={t('chatSafety.case.nicknameNote')} rules={openapiRules(LIMITS.caseNote, { required: true, t: ops.t })}>
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} {...openapiInputProps(LIMITS.caseNote)} />
        </Form.Item>
        <Button danger htmlType="submit" loading={act.isPending}>
          {t('chatSafety.case.nicknameReset')}
        </Button>
      </Form>
      <CaseWriteError error={act.error} />
    </Card>
  );
}

/** The case action "Đặt lại nickname" — hidden without action.live_ops.moderation_cases.act (as the other case actions). */
export function CaseNicknameAction({ c }: { c: ModerationCaseDetail }) {
  return (
    <ActionGate actionKey="action.live_ops.moderation_cases.act">
      <NicknameResetCard c={c} />
    </ActionGate>
  );
}
