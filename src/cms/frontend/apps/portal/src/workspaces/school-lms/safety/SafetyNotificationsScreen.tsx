// Screen of `menu.school_lms.safety_notifications` (F122 US7; F122-AC-18, AC-19, AC-20; ADMIN_PORTAL §1.5.2): the incident
// notifications the Platform Safety Team sent to the schools of the tenant — filter by state, school and period (last 30
// days by default: created_from, filtered by module chat in SQL — F122 T066; "Mọi thời gian" drops it), cursor pages;
// detail panel with the students (name and GRADE only — no class, ADR-0095), role, category, severity, measures, summary,
// time sent and the acknowledgement deadline; there is no chat content area anywhere. "Xác nhận đã liên hệ phụ huynh":
// contact outcome required, note ≤ 500 with a counter, Idempotency-Key; 409 → "Thông báo đã được xác nhận" and reload.
// Below 768 px the list is cards and the detail is its own page with the confirm button.
import { Alert, Button, Card, Descriptions, Flex, Form, Input, List, Radio, Tag, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { openapiInputProps, openapiRules, OPS_UI_NS, SearchSelect, SlaBadge } from '@eduworld/ops-ui';
import dayjs from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Route, Routes, useParams } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { ApiError } from '../../../shared/api/middleware/problem';
import { LIMITS, SafetyError, SeverityTag } from '../../../shared/chatSafety/ui';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ResponsiveTable } from '../../../shell/ResponsiveTable';
import { SchoolSelect } from '../common/SchoolSelect';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { useAcknowledge, useSchoolNotifications, type SchoolNotification } from './api';

const OUTCOMES = ['parent_contacted', 'unable_to_contact', 'not_needed'] as const;
const formatTime = (ts?: string | null) => (ts ? dayjs(ts).format('DD/MM/YYYY HH:mm') : '');

type StateFilter = 'pending' | 'acknowledged' | 'all';
type PeriodFilter = 'last30' | 'all';

/** Default period of the list (F122-AC-18: "khoảng 30 ngày"). */
export const NOTIFICATION_PERIOD_DAYS = 30;

function useNotificationFilters() {
  const [state, setState] = useState<StateFilter>('pending');
  const [schoolId, setSchoolId] = useState<string | undefined>();
  const [period, setPeriod] = useState<PeriodFilter>('last30');
  // fixed when the screen opens, so the query key (and the cursor pages) stay stable
  const [from] = useState(() => new Date(Date.now() - NOTIFICATION_PERIOD_DAYS * 86_400_000).toISOString());
  return { state, setState, schoolId, setSchoolId, period, setPeriod, createdFrom: period === 'last30' ? from : undefined };
}

function AckForm({ n, onDone }: { n: SchoolNotification; onDone: () => void }) {
  const { t } = useTranslation();
  const ops = useTranslation(OPS_UI_NS);
  const ack = useAcknowledge();
  const conflict = ack.error instanceof ApiError && ack.error.status === 409;
  return (
    <ActionGate actionKey="action.school_lms.safety_notifications.acknowledge">
      <Form
        layout="vertical"
        name="acknowledge"
        onFinish={(v: { contact_outcome: (typeof OUTCOMES)[number]; note?: string }) =>
          ack.mutate({ id: n.notification_id, body: { contact_outcome: v.contact_outcome, ...(v.note?.trim() ? { note: v.note.trim() } : {}) } }, { onSuccess: onDone })
        }
      >
        <Form.Item name="contact_outcome" label={t('chatSafety.ack.outcome')} rules={[{ required: true, message: t('chatSafety.ack.outcomeRequired') }]}>
          <Radio.Group>
            <Flex vertical>
              {OUTCOMES.map((o) => (
                <Radio key={o} value={o}>
                  {t(`chatSafety.outcome.${o}`)}
                </Radio>
              ))}
            </Flex>
          </Radio.Group>
        </Form.Item>
        <Form.Item name="note" label={t('chatSafety.ack.note')} rules={openapiRules(LIMITS.ackNote, { t: ops.t })}>
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 5 }} {...openapiInputProps(LIMITS.ackNote)} />
        </Form.Item>
        {conflict ? <Alert type="warning" showIcon role="alert" message={t('chatSafety.ack.already')} /> : ack.error ? <ProblemMessage error={ack.error} /> : null}
        <Button type="primary" htmlType="submit" loading={ack.isPending}>
          {t('chatSafety.ack.confirm')}
        </Button>
      </Form>
    </ActionGate>
  );
}

export function NotificationDetail({ n }: { n: SchoolNotification }) {
  const { t } = useTranslation();
  return (
    <Card size="small" title={t('chatSafety.notifications.detail')} data-testid="notification-detail">
      <Descriptions size="small" column={1} bordered>
        <Descriptions.Item label={t('chatSafety.cases.severity')}>
          <SeverityTag value={n.severity} />
        </Descriptions.Item>
        <Descriptions.Item label={t('chatSafety.cases.category')}>{n.category_code ? t(`chatSafety.category.${n.category_code}`, { defaultValue: n.category_code }) : ''}</Descriptions.Item>
        <Descriptions.Item label={t('chatSafety.notifications.students')}>
          <List
            size="small"
            dataSource={n.students ?? []}
            renderItem={(s) => (
              <List.Item key={s.student_id}>
                {s.display_name ?? s.student_id}
                {s.grade_level ? ` · ${t('chatSafety.notifications.grade', { grade: s.grade_level })}` : ''} · <Tag>{t(`chatSafety.role.${s.role}`)}</Tag>
              </List.Item>
            )}
          />
        </Descriptions.Item>
        <Descriptions.Item label={t('chatSafety.notifications.measures')}>{n.measures_taken ?? ''}</Descriptions.Item>
        <Descriptions.Item label={t('chatSafety.notifications.summary')}>{n.summary ?? ''}</Descriptions.Item>
        <Descriptions.Item label={t('chatSafety.notifications.sentAt')}>{formatTime(n.school_notified_at)}</Descriptions.Item>
        <Descriptions.Item label={t('chatSafety.notifications.ackDue')}>
          <SlaBadge dueAt={n.ack_due_at} done={!!n.school_ack_at} />
        </Descriptions.Item>
        {n.school_ack_at ? (
          <Descriptions.Item label={t('chatSafety.notifications.outcome')}>
            {n.contact_outcome ? t(`chatSafety.outcome.${n.contact_outcome}`) : ''} · {formatTime(n.school_ack_at)}
          </Descriptions.Item>
        ) : null}
      </Descriptions>
    </Card>
  );
}

function NotificationPage() {
  const { t } = useTranslation();
  const { notificationId = '' } = useParams();
  const list = useSchoolNotifications({});
  const n = list.rows.find((x) => x.notification_id === notificationId);
  if (list.error) return <SafetyError error={list.error} />;
  if (!n) {
    return list.isFetching ? <Card loading /> : list.hasNextPage ? <Button onClick={() => list.fetchNextPage()}>{t('chatSafety.common.loadMore')}</Button> : <Alert type="info" showIcon message={t('chatSafety.notifications.notFound')} />;
  }
  return (
    <Flex vertical gap="middle">
      <Link to="..">{t('chatSafety.common.back')}</Link>
      <NotificationDetail n={n} />
      {n.school_ack_at ? <Alert type="success" showIcon message={t('chatSafety.ack.done')} /> : <AckForm n={n} onDone={() => void list.refetch()} />}
    </Flex>
  );
}

function NotificationList() {
  const { t } = useTranslation();
  const f = useNotificationFilters();
  const list = useSchoolNotifications({ acknowledged: f.state === 'all' ? undefined : f.state === 'acknowledged', schoolId: f.schoolId, createdFrom: f.createdFrom });
  const columns: ColumnType<SchoolNotification>[] = [
    { key: 'title', title: t('chatSafety.notifications.incident'), render: (_, r) => <Link to={r.notification_id}>{t('chatSafety.notifications.incidentTitle', { severity: t(`chatSafety.severity.${r.severity}`), at: formatTime(r.school_notified_at) })}</Link> },
    { key: 'students', title: t('chatSafety.notifications.students'), render: (_, r) => (r.students ?? []).map((s) => `${s.display_name ?? s.student_id}${s.grade_level ? ` (${t('chatSafety.notifications.grade', { grade: s.grade_level })})` : ''}`).join(', ') },
    { key: 'severity', title: t('chatSafety.cases.severity'), render: (_, r) => <SeverityTag value={r.severity} /> },
    { key: 'due', title: t('chatSafety.notifications.ackDue'), render: (_, r) => <SlaBadge dueAt={r.ack_due_at} done={!!r.school_ack_at} /> },
    { key: 'state', title: t('chatSafety.notifications.state'), render: (_, r) => (r.school_ack_at ? <Tag color="success">{t('chatSafety.notifications.acknowledged')}</Tag> : <Tag color="warning">{t('chatSafety.notifications.pending')}</Tag>) },
  ];
  return (
    <Flex vertical gap="middle">
      <Form layout="inline" name="school_notification_filters">
        <Form.Item label={t('chatSafety.notifications.state')}>
          <SearchSelect
            aria-label={t('chatSafety.notifications.state')}
            value={f.state}
            onChange={f.setState}
            options={(['pending', 'acknowledged', 'all'] as const).map((v) => ({ value: v, label: t(`chatSafety.notifications.filter.${v}`) }))}
            style={{ minWidth: '10rem' }}
          />
        </Form.Item>
        <Form.Item label={t('chatSafety.notifications.period')}>
          <SearchSelect
            aria-label={t('chatSafety.notifications.period')}
            value={f.period}
            onChange={f.setPeriod}
            options={(['last30', 'all'] as const).map((v) => ({ value: v, label: t(`chatSafety.notifications.periodOption.${v}`) }))}
            style={{ minWidth: '10rem' }}
          />
        </Form.Item>
        <Form.Item label={t('chatSafety.common.school')}>
          <SchoolSelect value={f.schoolId} onChange={f.setSchoolId} allowClear autoSelect={false} />
        </Form.Item>
      </Form>
      {list.error ? <SafetyError error={list.error} /> : null}
      <ResponsiveTable<SchoolNotification>
        rowKey="notification_id"
        columns={columns}
        data={list.rows}
        loading={list.isFetching}
        cardTitle={(r) => <Link to={r.notification_id}>{t('chatSafety.notifications.incidentTitle', { severity: t(`chatSafety.severity.${r.severity}`), at: formatTime(r.school_notified_at) })}</Link>}
        hasMore={list.hasNextPage}
        onLoadMore={() => void list.fetchNextPage()}
        loadMoreLabel={t('chatSafety.common.loadMore')}
      />
      <Typography.Text type="secondary">{t('chatSafety.notifications.noChat')}</Typography.Text>
    </Flex>
  );
}

export function SafetyNotificationsScreen({ title }: { title: string }) {
  return (
    <section>
      <PageTitle title={title} />
      <SingleTenantGate>
        {() => (
          <Routes>
            <Route index element={<NotificationList />} />
            <Route path=":notificationId" element={<NotificationPage />} />
          </Routes>
        )}
      </SingleTenantGate>
    </section>
  );
}
