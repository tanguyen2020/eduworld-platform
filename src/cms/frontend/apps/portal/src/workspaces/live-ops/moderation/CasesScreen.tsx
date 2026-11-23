// Screen of `menu.live_ops.moderation_cases` (F122 US1, US2; F122-AC-01, AC-06, AC-24): the case queue of the Platform
// Safety Team across tenants — tenant filter of this module only (multi-select, default "all"), severity p1–p3, status,
// assignee (me / unassigned / any), category; Tenant column; SLA deadline from the server (overdue = error token + text).
// Cursor pages ("Tải thêm"), never the whole list. Second tab: school notifications sent by the platform
// (acknowledged / overdue filters). `:caseId` opens the case (CaseDetail). No chat search anywhere (F122-AC-02).
import { Button, Flex, Form, Input, Table, Tabs, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Route, Routes } from 'react-router-dom';
import { SearchSelect, SlaBadge } from '@eduworld/ops-ui';
import { clampTenantSearch, useTenantOptions } from '../../../shared/api/hooks/useTenantOptions';
import { LIMITS, SafetyError, SeverityTag, TenantName, useTenantNames } from '../../../shared/chatSafety/ui';
import { tenantLabel } from '../../../shell/TenantControls';
import { useCases, usePlatformNotifications, type CaseFilters, type ModerationCase, type SchoolNotification } from './api';
import { CaseDetail } from './CaseDetail';

export const formatTime = (ts?: string | null): string => (ts ? dayjs(ts).format('DD/MM/YYYY HH:mm') : '');

/** Tenant filter of the moderation module (F122-AC-01): Safety Moderator and System Admin; empty = every tenant. */
export function ModerationTenantFilter({ value, onChange }: { value: string[]; onChange: (ids: string[]) => void }) {
  const { t } = useTranslation();
  const [search, setSearch] = useState('');
  const options = useTenantOptions(search, true);
  return (
    <SearchSelect
      mode="multiple"
      aria-label={t('chatSafety.common.tenantFilter')}
      placeholder={t('chatSafety.common.allTenants')}
      value={value}
      onChange={onChange}
      showSearch
      filterOption={false}
      searchValue={search}
      onSearch={(v) => setSearch(clampTenantSearch(v))}
      loading={options.isFetching}
      allowClear
      maxTagCount="responsive"
      options={(options.data ?? []).map((x) => ({ value: x.tenant_id, label: tenantLabel(x, t) }))}
      style={{ minWidth: '16rem' }}
    />
  );
}

function CaseQueue() {
  const { t } = useTranslation();
  const [filters, setFilters] = useState<CaseFilters>({ tenantIds: [], assignee: 'any' });
  const list = useCases(filters);
  const names = useTenantNames();
  const set = (patch: Partial<CaseFilters>) => setFilters((f) => ({ ...f, ...patch }));
  const columns: ColumnsType<ModerationCase> = [
    { key: 'tenant', title: t('chatSafety.common.tenant'), render: (_, r) => <TenantName id={r.tenant_id} names={names} /> },
    { key: 'severity', title: t('chatSafety.cases.severity'), render: (_, r) => <SeverityTag value={r.severity} /> },
    { key: 'status', title: t('chatSafety.cases.status'), render: (_, r) => <Tag>{t(`chatSafety.caseStatus.${r.status}`)}</Tag> },
    { key: 'category', title: t('chatSafety.cases.category'), render: (_, r) => r.category_code ?? '' },
    {
      key: 'assignee',
      title: t('chatSafety.cases.assignee'),
      render: (_, r) => (r.assignee_moderator_id ? <Typography.Text code>{r.assignee_moderator_id.slice(-8)}</Typography.Text> : t('chatSafety.cases.unassigned')),
    },
    { key: 'sla', title: t('chatSafety.cases.sla'), render: (_, r) => <SlaBadge dueAt={r.sla_due_at} done={r.status === 'resolved'} /> },
    { key: 'created', title: t('chatSafety.cases.created'), render: (_, r) => formatTime(r.created_at) },
    { key: 'open', title: t('chatSafety.common.actions'), render: (_, r) => <Link to={r.case_id}>{t('chatSafety.cases.open')}</Link> },
  ];
  return (
    <Flex vertical gap="middle">
      <Form layout="inline" name="case_filters">
        <Form.Item label={t('chatSafety.common.tenantFilter')}>
          <ModerationTenantFilter value={filters.tenantIds} onChange={(ids) => set({ tenantIds: ids })} />
        </Form.Item>
        <Form.Item label={t('chatSafety.cases.severity')}>
          <SearchSelect
            aria-label={t('chatSafety.cases.severity')}
            allowClear
            value={filters.severity}
            onChange={(v) => set({ severity: v })}
            options={(['p1', 'p2', 'p3'] as const).map((v) => ({ value: v, label: t(`chatSafety.severity.${v}`) }))}
            style={{ minWidth: '8rem' }}
          />
        </Form.Item>
        <Form.Item label={t('chatSafety.cases.status')}>
          <SearchSelect
            aria-label={t('chatSafety.cases.status')}
            allowClear
            value={filters.status}
            onChange={(v) => set({ status: v })}
            options={(['open', 'in_review', 'escalated', 'resolved', 'reopened'] as const).map((v) => ({ value: v, label: t(`chatSafety.caseStatus.${v}`) }))}
            style={{ minWidth: '10rem' }}
          />
        </Form.Item>
        <Form.Item label={t('chatSafety.cases.assignee')}>
          <SearchSelect
            aria-label={t('chatSafety.cases.assignee')}
            value={filters.assignee}
            onChange={(v) => set({ assignee: v })}
            options={(['any', 'me', 'unassigned'] as const).map((v) => ({ value: v, label: t(`chatSafety.assignee.${v}`) }))}
            style={{ minWidth: '9rem' }}
          />
        </Form.Item>
        <Form.Item label={t('chatSafety.cases.category')}>
          <Input.Search aria-label={t('chatSafety.cases.category')} allowClear maxLength={LIMITS.category.maxLength} onSearch={(v) => set({ category: v.trim() || undefined })} />
        </Form.Item>
      </Form>
      {list.error ? <SafetyError error={list.error} /> : null}
      <Table rowKey="case_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} scroll={{ x: true }} locale={{ emptyText: t('chatSafety.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('chatSafety.common.loadMore')}
        </Button>
      ) : null}
    </Flex>
  );
}

function PlatformNotifications() {
  const { t } = useTranslation();
  const [tenantIds, setTenantIds] = useState<string[]>([]);
  const [state, setState] = useState<'all' | 'pending' | 'acknowledged' | 'overdue'>('all');
  const list = usePlatformNotifications({
    tenantIds,
    acknowledged: state === 'acknowledged' ? true : state === 'pending' ? false : undefined,
    overdue: state === 'overdue',
  });
  const names = useTenantNames();
  const columns: ColumnsType<SchoolNotification> = [
    { key: 'tenant', title: t('chatSafety.common.tenant'), render: (_, r) => <TenantName id={r.tenant_id} names={names} /> },
    { key: 'severity', title: t('chatSafety.cases.severity'), render: (_, r) => <SeverityTag value={r.severity} /> },
    { key: 'sent', title: t('chatSafety.notifications.sentAt'), render: (_, r) => formatTime(r.school_notified_at) },
    { key: 'due', title: t('chatSafety.notifications.ackDue'), render: (_, r) => <SlaBadge dueAt={r.ack_due_at} done={!!r.school_ack_at} /> },
    { key: 'outcome', title: t('chatSafety.notifications.outcome'), render: (_, r) => (r.contact_outcome ? t(`chatSafety.outcome.${r.contact_outcome}`) : t('chatSafety.notifications.pending')) },
    { key: 'case', title: t('chatSafety.notifications.case'), render: (_, r) => <Link to={r.case_id}>{t('chatSafety.cases.open')}</Link> },
  ];
  return (
    <Flex vertical gap="middle">
      <Form layout="inline" name="notification_filters">
        <Form.Item label={t('chatSafety.common.tenantFilter')}>
          <ModerationTenantFilter value={tenantIds} onChange={setTenantIds} />
        </Form.Item>
        <Form.Item label={t('chatSafety.notifications.state')}>
          <SearchSelect
            aria-label={t('chatSafety.notifications.state')}
            value={state}
            onChange={setState}
            options={(['all', 'pending', 'acknowledged', 'overdue'] as const).map((v) => ({ value: v, label: t(`chatSafety.notifications.filter.${v}`) }))}
            style={{ minWidth: '10rem' }}
          />
        </Form.Item>
      </Form>
      {list.error ? <SafetyError error={list.error} /> : null}
      <Table rowKey="notification_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} scroll={{ x: true }} locale={{ emptyText: t('chatSafety.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('chatSafety.common.loadMore')}
        </Button>
      ) : null}
    </Flex>
  );
}

export function CasesScreen({ title }: { title: string }) {
  const { t } = useTranslation();
  return (
    <section>
      <Typography.Title level={1}>{title}</Typography.Title>
      <Routes>
        <Route
          index
          element={
            <Tabs
              items={[
                { key: 'queue', label: t('chatSafety.cases.queue'), children: <CaseQueue /> },
                { key: 'notifications', label: t('chatSafety.notifications.platformTitle'), children: <PlatformNotifications /> },
              ]}
            />
          }
        />
        <Route path=":caseId" element={<CaseDetail />} />
      </Routes>
    </section>
  );
}
