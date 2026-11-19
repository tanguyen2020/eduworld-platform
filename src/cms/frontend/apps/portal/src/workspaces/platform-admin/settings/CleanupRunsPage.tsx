// Cleanup runs of every tenant (F057 T037, F057-AC-15, ADR-0032): listStudentCleanupRuns (tenant filter → `tenant_ids[]`,
// cursor) with candidates / cleaned / excluded / skipped; a run `awaiting_approval` gets "Xác nhận" (ActionGate
// approve_cleanup) → confirmation → approveStudentCleanupRun (Idempotency-Key, X-Tenant-Id = tenant of the run).
// No "Hủy" until cancelStudentCleanupRun exists (Câu hỏi mở F057-Q2).
import { App as AntApp, Button, Card, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { TenantFilter } from '../../../shell/TenantControls';
import { CLEANUP_CANCEL_AVAILABLE, useApproveRun, useCleanupRuns, type StudentCleanupRun } from '../api/cleanup';
import { useTenantDirectory } from '../api/tenants';
import { formatTime, TenantCell } from '../ui';

export function CleanupRunsCard() {
  const { t } = useTranslation();
  const { modal, message } = AntApp.useApp();
  const runs = useCleanupRuns();
  const directory = useTenantDirectory();
  const approve = useApproveRun();

  const confirmApprove = (run: StudentCleanupRun) =>
    modal.confirm({
      title: t('platform.settings.approveTitle', { tenant: directory.get(run.tenant_id)?.name ?? run.tenant_id }),
      content: t('platform.settings.approveContent', { count: run.candidates ?? 0 }),
      okText: t('platform.settings.approve'),
      cancelText: t('platform.common.cancel'),
      // a failure stays visible under the card (ProblemMessage of approve.error) after the dialog closes
      onOk: () =>
        approve
          .mutateAsync(run)
          .then(() => message.success(t('platform.settings.approved')))
          .catch(() => undefined),
    });

  const columns: ColumnsType<StudentCleanupRun> = [
    { key: 'tenant', title: t('platform.common.tenant'), render: (_, r) => <TenantCell tenantId={r.tenant_id} directory={directory} /> },
    {
      key: 'status',
      title: t('platform.common.status'),
      render: (_, r) => <Tag color={r.status === 'awaiting_approval' ? 'warning' : 'default'}>{t(`platform.settings.runsStatus.${r.status}`)}</Tag>,
    },
    { key: 'candidates', title: t('platform.settings.candidates'), dataIndex: 'candidates' },
    { key: 'cleaned', title: t('platform.settings.cleaned'), dataIndex: 'cleaned' },
    { key: 'excluded', title: t('platform.settings.excluded'), dataIndex: 'excluded' },
    { key: 'skipped', title: t('platform.settings.skipped'), dataIndex: 'skipped_active' },
    { key: 'started', title: t('platform.settings.startedAt'), render: (_, r) => formatTime(r.started_at) },
    { key: 'finished', title: t('platform.settings.finishedAt'), render: (_, r) => formatTime(r.finished_at) },
    {
      key: 'actions',
      title: t('platform.common.actions'),
      render: (_, r) =>
        r.status === 'awaiting_approval' ? (
          <ActionGate actionKey="action.platform_admin.system_settings.approve_cleanup">
            <Button size="small" type="primary" onClick={() => confirmApprove(r)}>
              {t('platform.settings.approve')}
            </Button>
          </ActionGate>
        ) : null,
    },
  ];

  return (
    <Card title={t('platform.settings.runsTitle')}>
      <Space wrap>
        <TenantFilter />
        {CLEANUP_CANCEL_AVAILABLE ? null : <Typography.Text type="secondary">{t('platform.settings.cancelPending')}</Typography.Text>}
      </Space>
      {runs.error ? <ProblemMessage error={runs.error} /> : null}
      {approve.error ? <ProblemMessage error={approve.error} /> : null}
      <Table rowKey="run_id" columns={columns} dataSource={runs.rows} loading={runs.isFetching} pagination={false} locale={{ emptyText: t('platform.common.empty') }} />
      {runs.hasNextPage ? (
        <Button onClick={() => runs.fetchNextPage()} loading={runs.isFetchingNextPage}>
          {t('platform.common.loadMore')}
        </Button>
      ) : null}
    </Card>
  );
}
