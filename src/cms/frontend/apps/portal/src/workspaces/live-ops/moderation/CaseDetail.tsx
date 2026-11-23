// Case detail (F122 US1/US2; F122-AC-02, AC-03, AC-04, AC-05): getModerationCase + listModerationCaseMessages — evidence
// panel (snapshot messages marked, original content, revisions, recall state), action history, school notifications;
// the only search is inside this case (`q`). Take the case (assignModerationCase), act on it (actOnModerationCase with
// If-Match = case version and X-Tenant-Id = tenant of the case; note mandatory with a counter; `account_lock` needs a second
// confirmation), notify the school (NotifySchoolDrawer; hidden for a tenant `common` case → "Liên hệ trực tiếp").
// Errors are shown by `code` with the trace id; 412 reloads the case; nothing is retried automatically (F122-AC-04).
import { Alert, Button, Card, Checkbox, Descriptions, Flex, Form, Input, InputNumber, List, Modal, Space, Tag, Timeline, Typography } from 'antd';
import { openapiInputProps, openapiRules, OPS_UI_NS, SearchSelect, SlaBadge } from '@eduworld/ops-ui';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useParams } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { useMe } from '../../../shared/api/hooks/useMe';
import { ApiError } from '../../../shared/api/middleware/problem';
import { LIMITS, SafetyError, SeverityTag, TenantName, useTenantNames } from '../../../shared/chatSafety/ui';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useActOnCase, useAssignCase, useCase, useCaseMessages, type CaseMessage, type ModerationAction, type ModerationCaseDetail } from './api';
import { formatTime } from './CasesScreen';
import { NotifySchoolDrawer } from './NotifySchoolDrawer';

const ACTIONS: ModerationAction['action'][] = ['warn', 'mute', 'restrict', 'remove_message', 'escalate', 'dismiss', 'close', 'reopen'];
const RESTRICTIONS: NonNullable<ModerationAction['sanction_type']>[] = ['restrict_text', 'restrict_dm', 'restrict_multiplayer', 'restrict_competition', 'account_lock'];

function EvidencePanel({ caseId }: { caseId: string }) {
  const { t } = useTranslation();
  const [q, setQ] = useState('');
  const list = useCaseMessages(caseId, q);
  return (
    <Card title={t('chatSafety.case.evidence')} size="small">
      <Flex vertical gap="small">
        <Input.Search
          aria-label={t('chatSafety.case.searchInCase')}
          placeholder={t('chatSafety.case.searchInCase')}
          allowClear
          {...openapiInputProps(LIMITS.caseSearch)}
          onSearch={(v) => setQ(v.trim())}
        />
        {list.error ? <SafetyError error={list.error} /> : null}
        <List<CaseMessage>
          loading={list.isFetching}
          dataSource={list.rows}
          locale={{ emptyText: t('chatSafety.common.empty') }}
          renderItem={(m) => (
            <List.Item key={m.message_id} data-testid={`evidence-${m.message_id}`}>
              <Flex vertical gap="small" style={{ width: '100%' }}>
                <Space wrap>
                  <Typography.Text strong>{m.sender_nickname ?? m.sender_id.slice(-8)}</Typography.Text>
                  {m.in_snapshot ? <Tag color="processing">{t('chatSafety.case.inSnapshot')}</Tag> : <Tag>{t('chatSafety.case.context')}</Tag>}
                  <Tag>{t(`chatSafety.messageState.${m.moderation_state}`)}</Tag>
                  {m.recalled_at ? <Tag color="warning">{t('chatSafety.case.recalled', { at: formatTime(m.recalled_at) })}</Tag> : null}
                  <Typography.Text type="secondary">{formatTime(m.created_at)}</Typography.Text>
                </Space>
                <Descriptions size="small" column={1}>
                  <Descriptions.Item label={t('chatSafety.case.original')}>{m.body ?? ''}</Descriptions.Item>
                  {m.body_display != null ? <Descriptions.Item label={t('chatSafety.case.displayed')}>{m.body_display}</Descriptions.Item> : null}
                  <Descriptions.Item label={t('chatSafety.case.revision')}>{m.revision ?? 1}</Descriptions.Item>
                </Descriptions>
                {m.revisions?.length ? (
                  <Timeline
                    items={m.revisions.map((r) => ({
                      key: r.revision,
                      children: t('chatSafety.case.revisionLine', { revision: r.revision, body: r.body ?? '', decision: r.decision ?? '' }),
                    }))}
                  />
                ) : null}
              </Flex>
            </List.Item>
          )}
        />
        {list.hasNextPage ? (
          <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
            {t('chatSafety.common.loadMore')}
          </Button>
        ) : null}
      </Flex>
    </Card>
  );
}

interface ActForm {
  action: ModerationAction['action'];
  target_user_id?: string;
  message_id?: string;
  sanction_type?: ModerationAction['sanction_type'];
  duration_minutes?: number;
  note: string;
}

/** Translated message of a write error (own text for the codes the case screen handles). */
function writeErrorMessage(error: unknown, t: (k: string) => string): string {
  if (!(error instanceof ApiError)) return t('error.generic');
  if (error.code === 'VERSION_MISMATCH' || error.status === 412) return t('chatSafety.case.reloaded');
  return t(error.i18nKey);
}

function CaseActions({ c }: { c: ModerationCaseDetail }) {
  const { t } = useTranslation();
  const ops = useTranslation(OPS_UI_NS);
  const [form] = Form.useForm<ActForm>();
  const act = useActOnCase(c);
  const [confirm, setConfirm] = useState<ModerationAction | null>(null);
  const [understood, setUnderstood] = useState(false);
  const action = Form.useWatch('action', form);
  const sanction = Form.useWatch('sanction_type', form);
  const submit = (v: ActForm) => {
    const body: ModerationAction = {
      action: v.action,
      note: v.note.trim(),
      ...(v.target_user_id ? { target_user_id: v.target_user_id } : {}),
      ...(v.message_id ? { message_id: v.message_id } : {}),
      ...(v.action === 'restrict' && v.sanction_type ? { sanction_type: v.sanction_type } : {}),
      ...(v.duration_minutes ? { duration_minutes: v.duration_minutes } : {}),
    };
    if (body.sanction_type === 'account_lock') {
      setUnderstood(false);
      setConfirm(body);
      return;
    }
    act.mutate(body, { onSuccess: () => form.resetFields() });
  };
  const error = act.error ? (
    <Alert
      type="error"
      showIcon
      role="alert"
      message={writeErrorMessage(act.error, t)}
      description={act.error instanceof ApiError && act.error.traceId ? t('shell.problem.traceId', { traceId: act.error.traceId }) : undefined}
    />
  ) : null;
  return (
    <Card title={t('chatSafety.case.act')} size="small">
      <Form form={form} layout="vertical" name="case_act" onFinish={submit} initialValues={{ action: 'warn' }}>
        <Form.Item name="action" label={t('chatSafety.case.action')} rules={[{ required: true }]}>
          <SearchSelect aria-label={t('chatSafety.case.action')} options={ACTIONS.map((a) => ({ value: a, label: t(`chatSafety.action.${a}`) }))} />
        </Form.Item>
        {c.subject_user_ids?.length ? (
          <Form.Item name="target_user_id" label={t('chatSafety.case.targetUser')}>
            <SearchSelect allowClear aria-label={t('chatSafety.case.targetUser')} options={c.subject_user_ids.map((id) => ({ value: id, label: id }))} />
          </Form.Item>
        ) : null}
        {action === 'remove_message' && c.evidence_message_ids?.length ? (
          <Form.Item name="message_id" label={t('chatSafety.case.message')} rules={[{ required: true, message: t('chatSafety.case.messageRequired') }]}>
            <SearchSelect aria-label={t('chatSafety.case.message')} options={c.evidence_message_ids.map((id) => ({ value: id, label: id }))} />
          </Form.Item>
        ) : null}
        {action === 'restrict' ? (
          <Form.Item name="sanction_type" label={t('chatSafety.case.sanctionType')} rules={[{ required: true, message: t('chatSafety.case.sanctionTypeRequired') }]}>
            <SearchSelect aria-label={t('chatSafety.case.sanctionType')} options={RESTRICTIONS.map((s) => ({ value: s, label: t(`chatSafety.sanctionType.${s}`) }))} />
          </Form.Item>
        ) : null}
        {action === 'mute' || action === 'restrict' ? (
          <Form.Item name="duration_minutes" label={t('chatSafety.case.duration')}>
            <InputNumber min={1} max={43200} aria-label={t('chatSafety.case.duration')} />
          </Form.Item>
        ) : null}
        {sanction === 'account_lock' && action === 'restrict' ? <Alert type="warning" showIcon message={t('chatSafety.case.lockWarning')} /> : null}
        <Form.Item name="note" label={t('chatSafety.case.note')} rules={openapiRules(LIMITS.caseNote, { required: true, t: ops.t })}>
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} {...openapiInputProps(LIMITS.caseNote)} />
        </Form.Item>
        <ActionGate actionKey="action.live_ops.moderation_cases.act">
          <Button type="primary" htmlType="submit" loading={act.isPending}>
            {t('chatSafety.case.apply')}
          </Button>
        </ActionGate>
      </Form>
      {error}
      <Modal
        open={!!confirm}
        title={t('chatSafety.case.lockConfirmTitle')}
        okText={t('chatSafety.case.lockConfirm')}
        okButtonProps={{ danger: true, disabled: !understood }}
        cancelText={t('chatSafety.common.cancel')}
        onCancel={() => setConfirm(null)}
        onOk={() => {
          if (!confirm) return;
          act.mutate(confirm, { onSuccess: () => form.resetFields() });
          setConfirm(null);
        }}
      >
        <Typography.Paragraph>{t('chatSafety.case.lockConfirmText')}</Typography.Paragraph>
        <Checkbox checked={understood} onChange={(e) => setUnderstood(e.target.checked)}>
          {t('chatSafety.case.lockUnderstood')}
        </Checkbox>
      </Modal>
    </Card>
  );
}

export function CaseDetail() {
  const { t } = useTranslation();
  const { caseId = '' } = useParams();
  const q = useCase(caseId);
  const me = useMe();
  const names = useTenantNames();
  const [notify, setNotify] = useState(false);
  const c = q.data;
  const assign = useAssignCase({ case_id: caseId, tenant_id: c?.tenant_id ?? '' });
  if (q.error) return <SafetyError error={q.error} />;
  if (!c) return <Card loading />;
  const common = names.get(c.tenant_id)?.tenant_type === 'common';
  return (
    <Flex vertical gap="middle">
      <Link to="..">{t('chatSafety.case.back')}</Link>
      <Card size="small" title={t('chatSafety.case.title', { id: c.case_id.slice(-8) })}>
        <Descriptions size="small" column={{ xs: 1, md: 2 }}>
          <Descriptions.Item label={t('chatSafety.common.tenant')}>
            <TenantName id={c.tenant_id} names={names} />
          </Descriptions.Item>
          <Descriptions.Item label={t('chatSafety.cases.severity')}>
            <SeverityTag value={c.severity} />
          </Descriptions.Item>
          <Descriptions.Item label={t('chatSafety.cases.status')}>{t(`chatSafety.caseStatus.${c.status}`)}</Descriptions.Item>
          <Descriptions.Item label={t('chatSafety.cases.category')}>{c.category_code ?? ''}</Descriptions.Item>
          <Descriptions.Item label={t('chatSafety.cases.sla')}>
            <SlaBadge dueAt={c.sla_due_at} done={c.status === 'resolved'} />
          </Descriptions.Item>
          <Descriptions.Item label={t('chatSafety.cases.assignee')}>{c.assignee_moderator_id ?? t('chatSafety.cases.unassigned')}</Descriptions.Item>
          <Descriptions.Item label={t('chatSafety.case.reports')}>{c.report_count ?? 0}</Descriptions.Item>
          <Descriptions.Item label={t('chatSafety.case.channel')}>{c.channel_type ? t(`chatSafety.channel.${c.channel_type}`) : ''}</Descriptions.Item>
        </Descriptions>
        <Space wrap>
          <ActionGate actionKey="action.live_ops.moderation_cases.act">
            <Button onClick={() => me.data && assign.mutate(me.data.user_id)} loading={assign.isPending} disabled={!me.data || c.assignee_moderator_id === me.data.user_id}>
              {t('chatSafety.case.take')}
            </Button>
          </ActionGate>
          {common ? (
            <Alert type="info" showIcon message={t('chatSafety.case.directContact')} />
          ) : (
            <ActionGate actionKey="action.live_ops.moderation_cases.notify_school">
              <Button onClick={() => setNotify(true)}>{t('chatSafety.case.notifySchool')}</Button>
            </ActionGate>
          )}
        </Space>
        {assign.error ? <ProblemMessage error={assign.error} /> : null}
      </Card>
      <EvidencePanel caseId={c.case_id} />
      <CaseActions c={c} />
      <Card size="small" title={t('chatSafety.case.history')}>
        <Timeline
          items={(c.actions ?? []).map((a, i) => ({
            key: i,
            children: (
              <span>
                {t(`chatSafety.action.${a.action}`, { defaultValue: a.action })} · {formatTime(a.at)}
                {a.note ? ` · ${a.note}` : ''}
              </span>
            ),
          }))}
        />
      </Card>
      <Card size="small" title={t('chatSafety.case.schoolNotifications')}>
        <List
          dataSource={c.school_notifications ?? []}
          locale={{ emptyText: t('chatSafety.common.empty') }}
          renderItem={(n) => (
            <List.Item key={n.notification_id}>
              {formatTime(n.school_notified_at)} · <SlaBadge dueAt={n.ack_due_at} done={!!n.school_ack_at} />
            </List.Item>
          )}
        />
      </Card>
      {notify ? <NotifySchoolDrawer c={c} onClose={() => setNotify(false)} /> : null}
    </Flex>
  );
}
