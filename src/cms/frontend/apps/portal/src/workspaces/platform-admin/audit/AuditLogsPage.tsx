// Menu "Nhật ký toàn hệ thống" (F059 US4, F059-AC-15…17, FR-AUD-001/002, ADMIN_PORTAL §7): tab "Toàn hệ thống" =
// listPlatformAuditLogs with mandatory `from`/`to` (default last 24 h), tenant filter (`tenant_ids[]`), actor, action,
// resource type/id; tab "Phân quyền" = the same query restricted to the `role.*` / `user_role.*` actions. Filters live
// in the URL query string (shareable link). A `422` (range over `audit_query_max_range_days`) is shown with the limit
// and the filters are kept. Read only: no edit/delete of an audit entry anywhere.
import { Alert, Button, DatePicker, Form, Input, Select, Space, Table, Tabs, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import dayjs, { type Dayjs } from 'dayjs';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { TenantFilter } from '../../../shell/TenantControls';
import { filterFromParams, PERMISSION_AUDIT_ACTIONS, useAuditLogs, type AuditLogEntry, type AuditMode } from '../api/audit';
import { useTenantDirectory } from '../api/tenants';
import { AUDIT_MAX_RANGE_DAYS, isUuid, LIMITS } from '../limits';
import { formatTime, limitInputProps, PageTitle, TenantCell } from '../ui';
import { AuditDetailDrawer } from './AuditDetailDrawer';

type Mode = AuditMode;

export function AuditLogsPage({ title }: { title: string }) {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const tab: Mode = params.get('tab') === 'permissions' ? 'permissions' : 'system';
  return (
    <section>
      <PageTitle title={title} />
      <Tabs
        activeKey={tab}
        onChange={(k) => setParams({ tab: k }, { replace: true })}
        destroyOnHidden
        items={[
          { key: 'system', label: t('platform.audit.tabSystem'), children: <AuditLogView mode="system" /> },
          { key: 'permissions', label: t('platform.audit.tabPermissions'), children: <AuditLogView mode="permissions" /> },
        ]}
      />
    </section>
  );
}

interface FormValues {
  range: [Dayjs, Dayjs];
  actor_id?: string;
  action?: string;
  resource_type?: string;
  resource_id?: string;
}

function AuditLogView({ mode }: { mode: Mode }) {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const [initialNow] = useState(() => dayjs());
  const filter = useMemo(() => filterFromParams(params, mode, initialNow), [params, mode, initialNow]);
  const list = useAuditLogs(filter);
  const directory = useTenantDirectory();
  const [form] = Form.useForm<FormValues>();
  const [detail, setDetail] = useState<AuditLogEntry | null>(null);
  const rangeError = list.error instanceof ApiError && list.error.code === 'VALIDATION_FAILED';

  const apply = (v: FormValues) => {
    const next: Record<string, string> = { tab: mode, from: v.range[0].toISOString(), to: v.range[1].toISOString() };
    if (v.actor_id?.trim()) next.actor_id = v.actor_id.trim();
    if (v.action?.trim()) next.action = v.action.trim();
    if (v.resource_type?.trim()) next.resource_type = v.resource_type.trim();
    if (v.resource_id?.trim()) next.resource_id = v.resource_id.trim();
    setParams(next, { replace: true });
  };

  const uuidRule = { validator: (_: unknown, v?: string) => (!v?.trim() || isUuid(v.trim()) ? Promise.resolve() : Promise.reject(new Error(t('platform.common.uuidInvalid')))) };
  const columns: ColumnsType<AuditLogEntry> = [
    { key: 'at', title: t('platform.audit.occurredAt'), render: (_, r) => formatTime(r.occurred_at) },
    { key: 'actor', title: t('platform.audit.actor'), render: (_, r) => (r.actor_id ? <Typography.Text code>{r.actor_id}</Typography.Text> : t('platform.audit.systemActor')) },
    { key: 'actor_type', title: t('platform.audit.actorType'), render: (_, r) => <Tag>{t(`platform.audit.actorKind.${r.actor_type}`)}</Tag> },
    { key: 'tenant', title: t('platform.common.tenant'), render: (_, r) => <TenantCell tenantId={r.tenant_id} directory={directory} /> },
    { key: 'action', title: t('platform.audit.action'), render: (_, r) => <Typography.Text code>{r.action}</Typography.Text> },
    { key: 'resource', title: t('platform.audit.resource'), render: (_, r) => `${r.target_type} · ${r.target_id}` },
    { key: 'result', title: t('platform.audit.result'), render: (_, r) => <Tag color={r.result === 'success' ? 'success' : 'error'}>{t(`platform.audit.resultKind.${r.result}`)}</Tag> },
    { key: 'reason', title: t('platform.common.reason'), render: (_, r) => r.reason },
    { key: 'trace', title: t('platform.audit.traceId'), render: (_, r) => (r.trace_id ? <Typography.Text code copyable>{r.trace_id}</Typography.Text> : null) },
    {
      key: 'detail',
      title: t('platform.audit.changes'),
      render: (_, r) => (
        <Button size="small" onClick={() => setDetail(r)}>
          {t('platform.audit.viewDetail')}
        </Button>
      ),
    },
  ];

  return (
    <section aria-label={t(mode === 'system' ? 'platform.audit.tabSystem' : 'platform.audit.tabPermissions')}>
      <Form
        form={form}
        name={`audit_${mode}`}
        layout="inline"
        onFinish={apply}
        initialValues={{ range: [dayjs(filter.from), dayjs(filter.to)], actor_id: filter.actorId, action: filter.action, resource_type: filter.resourceType, resource_id: filter.resourceId }}
      >
        <Form.Item name="range" label={t('platform.audit.range')} rules={[{ required: true, message: t('platform.audit.rangeRequired') }]}>
          <DatePicker.RangePicker showTime format="DD/MM/YYYY HH:mm" allowClear={false} />
        </Form.Item>
        <Form.Item name="actor_id" label={t('platform.audit.actorId')} rules={[uuidRule]}>
          <Input allowClear maxLength={36} />
        </Form.Item>
        <Form.Item name="action" label={t('platform.audit.action')}>
          {mode === 'permissions' ? (
            <Select options={PERMISSION_AUDIT_ACTIONS.map((a) => ({ value: a, label: a }))} />
          ) : (
            <Input allowClear {...limitInputProps(LIMITS.auditAction)} />
          )}
        </Form.Item>
        {mode === 'system' ? (
          <Form.Item name="resource_type" label={t('platform.audit.resourceType')}>
            <Input allowClear {...limitInputProps(LIMITS.auditResourceType)} />
          </Form.Item>
        ) : null}
        <Form.Item name="resource_id" label={t('platform.audit.resourceId')} rules={[uuidRule]}>
          <Input allowClear maxLength={36} />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit">
            {t('platform.audit.apply')}
          </Button>
        </Form.Item>
      </Form>
      <Space wrap>
        <TenantFilter />
        <Typography.Text type="secondary">{t('platform.audit.tenantHint')}</Typography.Text>
      </Space>
      {mode === 'permissions' ? <Typography.Paragraph type="secondary">{t('platform.audit.permissionsHint')}</Typography.Paragraph> : null}
      {list.error ? <ProblemMessage error={list.error} /> : null}
      {rangeError ? <Alert type="warning" showIcon message={t('platform.audit.rangeLimit', { days: AUDIT_MAX_RANGE_DAYS })} /> : null}
      <Table rowKey="audit_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} locale={{ emptyText: t('platform.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('platform.common.loadMore')}
        </Button>
      ) : null}
      {detail ? <AuditDetailDrawer entry={detail} directory={directory} onClose={() => setDetail(null)} /> : null}
    </section>
  );
}
