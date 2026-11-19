// Trash of every tenant + platform (F057 T028, F057-AC-09/10, ADMIN_PORTAL §3.8, AGENTS R21): listTrash by
// `resource_type` and the tenant filter (`tenant_ids[]`), columns Tenant / deleted by / deleted at / reason; "Khôi phục"
// asks for a reason (3–1000) then restoreTrashItem (Idempotency-Key, X-Tenant-Id = tenant of the item). A row with
// `restorable = false` or a `409 CONFLICT` answer shows "Đã có bản ghi mới cùng mã". There is no "Xóa vĩnh viễn".
import { Alert, App as AntApp, Button, Select, Space, Table, Tooltip, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { TenantFilter } from '../../../shell/TenantControls';
import type { ResourceType } from '../api/common';
import { useTenantDirectory } from '../api/tenants';
import { TRASH_TYPES, useRestore, useTrash, type TrashItem } from '../api/trash';
import { LIMITS } from '../limits';
import { formatTime, PageTitle, TenantCell } from '../ui';

export function TrashPage({ title }: { title: string }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const [resourceType, setResourceType] = useState<ResourceType>('question');
  const list = useTrash(resourceType);
  const directory = useTenantDirectory();
  const restore = useRestore();
  const [restoring, setRestoring] = useState<TrashItem | null>(null);
  const conflict = restore.error instanceof ApiError && restore.error.code === 'CONFLICT';

  const columns: ColumnsType<TrashItem> = [
    { key: 'title', title: t('platform.trash.title'), dataIndex: 'title' },
    { key: 'type', title: t('platform.trash.resourceType'), render: (_, r) => t(`platform.resourceType.${r.resource_type}`) },
    { key: 'tenant', title: t('platform.common.tenant'), render: (_, r) => <TenantCell tenantId={r.tenant_id} directory={directory} /> },
    { key: 'deleted_by', title: t('platform.trash.deletedBy'), render: (_, r) => <Typography.Text code>{r.deleted_by}</Typography.Text> },
    { key: 'deleted_at', title: t('platform.trash.deletedAt'), render: (_, r) => formatTime(r.deleted_at) },
    { key: 'reason', title: t('platform.trash.deleteReason'), dataIndex: 'delete_reason' },
    {
      key: 'actions',
      title: t('platform.common.actions'),
      render: (_, r) => {
        const blocked = r.restorable === false || r.resource_type === 'user';
        const hint = r.resource_type === 'user' ? t('platform.trash.mergedAccount') : t('platform.trash.notRestorable');
        const button = (
          <Button size="small" disabled={blocked} onClick={() => setRestoring(r)}>
            {t('platform.trash.restore')}
          </Button>
        );
        return (
          <ActionGate actionKey="action.platform_admin.trash.restore">
            {blocked ? (
              <Tooltip title={hint}>
                <span>
                  {button} <Typography.Text type="secondary">{hint}</Typography.Text>
                </span>
              </Tooltip>
            ) : (
              button
            )}
          </ActionGate>
        );
      },
    },
  ];

  return (
    <section>
      <PageTitle title={title} />
      <Space wrap>
        <Select
          aria-label={t('platform.trash.resourceType')}
          value={resourceType}
          onChange={setResourceType}
          options={TRASH_TYPES.map((x) => ({ value: x, label: t(`platform.resourceType.${x}`) }))}
        />
        <TenantFilter />
      </Space>
      {list.error ? <ProblemMessage error={list.error} /> : null}
      <Table rowKey="resource_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} locale={{ emptyText: t('platform.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('platform.common.loadMore')}
        </Button>
      ) : null}
      <ReasonDialog
        open={!!restoring}
        title={restoring ? t('platform.trash.restoreTitle', { title: restoring.title }) : ''}
        maxLength={LIMITS.restoreReason.maxLength}
        minLength={LIMITS.restoreReason.minLength}
        confirmLoading={restore.isPending}
        error={
          restore.error ? (
            <>
              <ProblemMessage error={restore.error} />
              {conflict ? <Alert type="warning" showIcon message={t('platform.trash.notRestorable')} /> : null}
            </>
          ) : null
        }
        onCancel={() => {
          setRestoring(null);
          restore.reset();
        }}
        onSubmit={(reason) =>
          restoring &&
          restore.mutate(
            { item: restoring, reason },
            {
              onSuccess: () => {
                message.success(t('platform.trash.restored'));
                setRestoring(null);
              },
            },
          )
        }
      />
    </section>
  );
}
