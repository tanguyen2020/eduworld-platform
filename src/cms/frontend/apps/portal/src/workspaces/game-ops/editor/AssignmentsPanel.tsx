// AssignmentsPanel (F108-AC-10): the tenants a tournament or challenge template is assigned to — read only here
// (listTenantAssignments, cursor pages "Tải thêm"). Assigning and unassigning live in workspace D (F057, saga F045); the
// list shows only to users who may read tenant data (ActionGate action.platform_admin.tenant_data.view — the backend
// authorizes again).
import { Button, Card, Empty, Skeleton, Table, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { usePortalRuntime } from '../../../app/runtime';
import { ActionGate } from '../../../permissions/ActionGate';
import { useSession } from '../../../session/SessionProvider';
import type { components } from '../../../shared/api/schema.gen';
import { formatIct } from '../../../shared/release/ict';
import { useCursorPages } from '../../../shared/api/useCursorPages';
import { GameOpsError } from '../shared/errors';

type TenantAssignment = components['schemas']['TenantAssignment'];
type PageInfo = components['schemas']['PageInfo'];
type ResourceType = components['schemas']['ResourceType'];

function useTemplateAssignments(resourceType: ResourceType, resourceId: string) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useCursorPages<TenantAssignment, { data: TenantAssignment[]; page: PageInfo }>(['game-ops', 'assignments', scopeKey, resourceType, resourceId], (cursor) =>
    api.call(api.client.GET('/admin/tenant-assignments', { params: { query: { resource_type: resourceType, resource_id: resourceId, ...(cursor ? { cursor } : {}) } } })),
  );
}

function AssignmentList({ resourceType, resourceId }: { resourceType: ResourceType; resourceId: string }) {
  const { t } = useTranslation();
  const q = useTemplateAssignments(resourceType, resourceId);
  if (q.isPending) return <Skeleton active />;
  if (q.error) return <GameOpsError error={q.error} />;
  if (q.rows.length === 0) return <Empty description={t('gameOps.assignments.none')} />;
  const columns: ColumnsType<TenantAssignment> = [
    { key: 'tenant', title: t('gameOps.assignments.tenant'), render: (_, a) => a.tenant_name ?? <Typography.Text code>{a.tenant_id.slice(-8)}</Typography.Text> },
    { key: 'at', title: t('gameOps.assignments.assignedAt'), render: (_, a) => formatIct(a.assigned_at) },
  ];
  return (
    <Table<TenantAssignment>
      rowKey="tenant_id"
      size="small"
      columns={columns}
      dataSource={q.rows}
      pagination={false}
      footer={
        q.hasNextPage
          ? () => (
              <Button onClick={() => void q.fetchNextPage()} loading={q.isFetchingNextPage}>
                {t('gameOps.common.loadMore')}
              </Button>
            )
          : undefined
      }
    />
  );
}

export function AssignmentsPanel({ resourceType, resourceId }: { resourceType: ResourceType; resourceId: string }) {
  const { t } = useTranslation();
  return (
    <ActionGate actionKey="action.platform_admin.tenant_data.view">
      <Card size="small" title={t('gameOps.assignments.title')} data-testid="template-assignments">
        <Typography.Paragraph type="secondary">{t('gameOps.assignments.readOnly')}</Typography.Paragraph>
        <AssignmentList resourceType={resourceType} resourceId={resourceId} />
      </Card>
    </ActionGate>
  );
}
