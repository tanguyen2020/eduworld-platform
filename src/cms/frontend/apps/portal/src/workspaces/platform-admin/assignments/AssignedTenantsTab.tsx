// Tab "Tenant được gán" (F057 T021, F057-AC-05/06): listTenantAssignments of one platform record (cursor), button
// "Gán cho tenant" (AssignToTenantsDialog) and per-tenant "Gỡ gán" after a confirmation stating that learning history
// and assigned work are kept (ADMIN_PORTAL §9), then unassignFromTenants (optional reason ≤ 500, Idempotency-Key); the
// checkbox "Bỏ luôn quy tắc gán mặc định" sends `remove_default: true` (OQ F045-9, F057 T054).
// Exported for the record pages of workspaces A/B.
import { App as AntApp, Button, Checkbox, Space, Table, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { useAssignments, useUnassign, type TenantAssignment } from '../api/assignments';
import type { ResourceType } from '../api/common';
import { LIMITS } from '../limits';
import { formatTime, useLargeScreen } from '../ui';
import { AssignToTenantsDialog } from './AssignToTenantsDialog';

export function AssignedTenantsTab({ resourceType, resourceId }: { resourceType: ResourceType; resourceId: string }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const large = useLargeScreen();
  const list = useAssignments(resourceType, resourceId);
  const unassign = useUnassign();
  const [assignOpen, setAssignOpen] = useState(false);
  const [removing, setRemoving] = useState<TenantAssignment | null>(null);
  const [removeDefault, setRemoveDefault] = useState(false);

  const columns: ColumnsType<TenantAssignment> = [
    { key: 'tenant', title: t('platform.common.tenant'), render: (_, r) => r.tenant_name ?? r.tenant_id },
    { key: 'assigned_at', title: t('platform.assign.assignedAt'), render: (_, r) => formatTime(r.assigned_at) },
  ];
  if (large) {
    columns.push({
      key: 'actions',
      title: t('platform.common.actions'),
      render: (_, r) => (
        <ActionGate actionKey="action.platform_admin.tenant_data.assign">
          <Button size="small" danger onClick={() => setRemoving(r)}>
            {t('platform.assign.unassign')}
          </Button>
        </ActionGate>
      ),
    });
  }

  return (
    <section aria-label={t('platform.assign.assignedTab')}>
      <Space wrap>
        {large ? (
          <ActionGate actionKey="action.platform_admin.tenant_data.assign">
            <Button type="primary" onClick={() => setAssignOpen(true)}>
              {t('platform.assign.open')}
            </Button>
          </ActionGate>
        ) : null}
      </Space>
      {list.error ? <ProblemMessage error={list.error} /> : null}
      <Table rowKey="tenant_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} locale={{ emptyText: t('platform.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('platform.common.loadMore')}
        </Button>
      ) : null}
      <AssignToTenantsDialog resourceType={resourceType} resourceId={resourceId} open={assignOpen} onClose={() => setAssignOpen(false)} />
      <ReasonDialog
        open={!!removing}
        title={removing ? t('platform.assign.unassignTitle', { name: removing.tenant_name ?? removing.tenant_id }) : ''}
        description={
          <Space direction="vertical" size="small">
            <span>{t('platform.assign.unassignContent')}</span>
            <Checkbox checked={removeDefault} onChange={(e) => setRemoveDefault(e.target.checked)}>
              {t('platform.assign.removeDefault')}
            </Checkbox>
            <Typography.Text type="secondary">{t('platform.assign.removeDefaultHint')}</Typography.Text>
          </Space>
        }
        maxLength={LIMITS.unassignReason.maxLength}
        confirmLoading={unassign.isPending}
        error={unassign.error ? <ProblemMessage error={unassign.error} /> : null}
        onCancel={() => {
          setRemoving(null);
          setRemoveDefault(false);
          unassign.reset();
        }}
        onSubmit={(reason) =>
          removing &&
          unassign.mutate(
            { resource_type: resourceType, resource_id: resourceId, tenant_ids: [removing.tenant_id], reason, ...(removeDefault ? { remove_default: true } : {}) },
            {
              onSuccess: () => {
                message.success(t('platform.assign.unassigned'));
                setRemoving(null);
                setRemoveDefault(false);
              },
            },
          )
        }
      />
    </section>
  );
}
