// Cleanup runs of every tenant (F057 T037, F057-AC-15, ADR-0032): listStudentCleanupRuns (tenant filter → `tenant_ids[]`,
// cursor) with candidates / cleaned / excluded / skipped; a run `awaiting_approval` gets "Xác nhận" (ActionGate
// approve_cleanup) → confirmation → approveStudentCleanupRun, and "Hủy" (same action, ADR-0090) → ReasonDialog
// (reason 3..500) → cancelStudentCleanupRun (OQ F057-2). Both send Idempotency-Key (kept when the same request is
// retried) and X-Tenant-Id = tenant of the run. The backend decides whether the run can still be approved/cancelled (409).
import { App as AntApp, Button, Card, Space, Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { TenantFilter } from '../../../shell/TenantControls';
import { useApproveRun, useCancelRun, useCleanupRuns, type StudentCleanupRun } from '../api/cleanup';
import { useTenantDirectory } from '../api/tenants';
import { LIMITS } from '../limits';
import { formatTime, TenantCell } from '../ui';

export function CleanupRunsCard() {
  const { t } = useTranslation();
  const { modal, message } = AntApp.useApp();
  const runs = useCleanupRuns();
  const directory = useTenantDirectory();
  const approve = useApproveRun();
  const cancel = useCancelRun();
  const [cancelling, setCancelling] = useState<StudentCleanupRun | null>(null);
  const tenantName = (run: StudentCleanupRun) => directory.get(run.tenant_id)?.name ?? run.tenant_id;

  const confirmApprove = (run: StudentCleanupRun) =>
    modal.confirm({
      title: t('platform.settings.approveTitle', { tenant: tenantName(run) }),
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
            <Space>
              <Button size="small" type="primary" onClick={() => confirmApprove(r)}>
                {t('platform.settings.approve')}
              </Button>
              <Button size="small" danger onClick={() => setCancelling(r)}>
                {t('platform.settings.cancelRun')}
              </Button>
            </Space>
          </ActionGate>
        ) : null,
    },
  ];

  return (
    <Card title={t('platform.settings.runsTitle')}>
      <Space wrap>
        <TenantFilter />
      </Space>
      {runs.error ? <ProblemMessage error={runs.error} /> : null}
      {approve.error ? <ProblemMessage error={approve.error} /> : null}
      <Table rowKey="run_id" columns={columns} dataSource={runs.rows} loading={runs.isFetching} pagination={false} locale={{ emptyText: t('platform.common.empty') }} />
      {runs.hasNextPage ? (
        <Button onClick={() => runs.fetchNextPage()} loading={runs.isFetchingNextPage}>
          {t('platform.common.loadMore')}
        </Button>
      ) : null}
      <ReasonDialog
        open={!!cancelling}
        title={cancelling ? t('platform.settings.cancelRunTitle', { tenant: tenantName(cancelling) }) : ''}
        description={cancelling ? t('platform.settings.cancelRunContent', { count: cancelling.candidates ?? 0 }) : undefined}
        minLength={LIMITS.cancelRunReason.minLength}
        maxLength={LIMITS.cancelRunReason.maxLength}
        confirmLoading={cancel.isPending}
        error={cancel.error ? <ProblemMessage error={cancel.error} /> : null}
        onCancel={() => {
          setCancelling(null);
          cancel.reset();
        }}
        onSubmit={(reason) =>
          cancelling &&
          cancel.mutate(
            { run: cancelling, reason },
            {
              onSuccess: () => {
                message.success(t('platform.settings.runCancelled'));
                setCancelling(null);
              },
            },
          )
        }
      />
    </Card>
  );
}
