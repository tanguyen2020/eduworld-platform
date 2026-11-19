// SIS connections of every tenant, read only (F059 T046, F059-AC-22/23, FR-SIS-007, F011-AC-27): listSisConnections
// (tenant filter `tenant_ids[]`, status, cursor) with tenant, provider, contract version, status, school count, sync
// schedule (`sis_sync_schedule` default when empty), whether a secret is configured (never its value) and the last sync
// run; a row expands to its schools (listSisConnectionSchools) and sync runs (listSisSyncRuns: status, time, rejected
// rows). Columns read named schema fields only, so a field outside the schema is never shown.
import { Button, Select, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { TenantFilter } from '../../../shell/TenantControls';
import { useSisConnections, useSisConnectionSchools, useSisSyncRuns, type SisConnection, type SisConnectionStatus, type SyncRun } from '../api/integrations';
import { useTenantDirectory } from '../api/tenants';
import { formatTime, TenantCell } from '../ui';

const STATUSES: readonly SisConnectionStatus[] = ['active', 'auth_error', 'contract_error', 'paused'];
/** `SyncRun.stats` key of the rejected rows (module identity SIS import). */
const REJECTED_STAT = 'rows_rejected';

function runText(run: SyncRun | null | undefined, t: (k: string, o?: Record<string, unknown>) => string): string {
  if (!run) return t('platform.integrations.noRun');
  return t('platform.integrations.runSummary', {
    status: t(`platform.integrations.runStatus.${run.status}`),
    at: formatTime(run.finished_at ?? run.started_at),
    rejected: run.stats?.[REJECTED_STAT] ?? 0,
  });
}

export function SisConnectionsTab() {
  const { t } = useTranslation();
  const [status, setStatus] = useState<SisConnectionStatus | undefined>();
  const list = useSisConnections(status);
  const directory = useTenantDirectory();

  const columns: ColumnsType<SisConnection> = [
    { key: 'tenant', title: t('platform.common.tenant'), render: (_, c) => <TenantCell tenantId={c.tenant_id} directory={directory} /> },
    { key: 'provider', title: t('platform.integrations.provider'), render: (_, c) => t(`platform.integrations.providerKind.${c.provider_kind}`) },
    { key: 'contract', title: t('platform.integrations.contractVersion'), render: (_, c) => c.contract_version },
    { key: 'status', title: t('platform.common.status'), render: (_, c) => <Tag>{t(`platform.integrations.status.${c.status}`)}</Tag> },
    { key: 'schools', title: t('platform.integrations.schoolCount'), render: (_, c) => c.school_count ?? 0 },
    { key: 'schedule', title: t('platform.integrations.schedule'), render: (_, c) => c.schedule ?? t('platform.integrations.defaultSchedule') },
    { key: 'secret', title: t('platform.integrations.secret'), render: (_, c) => t(c.has_secret ? 'platform.integrations.secretSet' : 'platform.integrations.secretNone') },
    { key: 'last', title: t('platform.integrations.lastRun'), render: (_, c) => runText(c.last_sync_run, t) },
  ];

  return (
    <section aria-label={t('platform.integrations.tabSis')}>
      <Space wrap>
        <Select
          aria-label={t('platform.common.status')}
          allowClear
          placeholder={t('platform.integrations.allStatuses')}
          value={status}
          onChange={setStatus}
          options={STATUSES.map((x) => ({ value: x, label: t(`platform.integrations.status.${x}`) }))}
        />
        <TenantFilter />
      </Space>
      {list.error ? <ProblemMessage error={list.error} /> : null}
      <Table
        rowKey="sis_connection_id"
        columns={columns}
        dataSource={list.rows}
        loading={list.isFetching}
        pagination={false}
        locale={{ emptyText: t('platform.common.empty') }}
        expandable={{ expandedRowRender: (c) => <ConnectionDetail connectionId={c.sis_connection_id} /> }}
      />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('platform.common.loadMore')}
        </Button>
      ) : null}
    </section>
  );
}

function ConnectionDetail({ connectionId }: { connectionId: string }) {
  const { t } = useTranslation();
  const schools = useSisConnectionSchools(connectionId);
  const runs = useSisSyncRuns(connectionId);
  return (
    <Space direction="vertical">
      <Typography.Text strong>{t('platform.integrations.schools')}</Typography.Text>
      {schools.error ? <ProblemMessage error={schools.error} /> : null}
      <Table
        rowKey="school_id"
        size="small"
        pagination={false}
        dataSource={schools.data ?? []}
        loading={schools.isFetching}
        columns={[
          { key: 'name', title: t('platform.integrations.schoolName'), render: (_, s) => s.school_name ?? s.school_id },
          { key: 'active', title: t('platform.common.status'), render: (_, s) => t(s.active ? 'platform.integrations.schoolActive' : 'platform.integrations.schoolInactive') },
          { key: 'assigned', title: t('platform.integrations.assignedAt'), render: (_, s) => formatTime(s.assigned_at) },
        ]}
      />
      <Typography.Text strong>{t('platform.integrations.runs')}</Typography.Text>
      {runs.error ? <ProblemMessage error={runs.error} /> : null}
      <Table
        rowKey="sync_run_id"
        size="small"
        pagination={false}
        dataSource={runs.data ?? []}
        loading={runs.isFetching}
        columns={[
          { key: 'kind', title: t('platform.integrations.runKind'), render: (_, r) => r.kind },
          { key: 'status', title: t('platform.common.status'), render: (_, r) => t(`platform.integrations.runStatus.${r.status}`) },
          { key: 'started', title: t('platform.integrations.startedAt'), render: (_, r) => formatTime(r.started_at) },
          { key: 'finished', title: t('platform.integrations.finishedAt'), render: (_, r) => formatTime(r.finished_at) },
          { key: 'rejected', title: t('platform.integrations.rejectedRows'), render: (_, r) => r.stats?.[REJECTED_STAT] ?? 0 },
        ]}
      />
    </Space>
  );
}
