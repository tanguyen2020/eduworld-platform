// TenantTrashPage (F054-AC-26, AC-28, FR-CMS-014, ADR-0028, AGENTS R21, ADMIN_PORTAL §3.8): "Thùng rác" of the session
// tenant — listTenantTrash by `resource_type` (cursor; X-Tenant-Id only, a System Admin picks one tenant first) and
// "Khôi phục" (`tenant.trash.restore`): reason 3–1000, restoreTenantTrashItem (Idempotency-Key). `restorable = false`
// or a `409 CONFLICT` shows that a newer record has the same code; a parent still deleted shows the server message.
// There is no permanent delete. The menu (`menu.school_lms.trash`) needs `tenant.trash.read`.
import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, App as AntApp, Button, Flex, Select, Tooltip, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePortalRuntime } from '../../../app/runtime';
import { ActionGate } from '../../../permissions/ActionGate';
import type { components } from '../../../shared/api/schema.gen';
import { ApiError } from '../../../shared/api/middleware/problem';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { ResponsiveTable } from '../../../shell/ResponsiveTable';
import { SingleTenantGate, type SchoolLmsScope } from '../common/scope';
import { formatDateTime } from '../common/format';

export type TrashItem = components['schemas']['TrashItem'];
export type ResourceType = components['schemas']['ResourceType'];

/** Tenant-owned resource types (platform-only types live in the platform Thùng rác, F057). */
export const TENANT_TRASH_TYPES: readonly ResourceType[] = ['question', 'question_set', 'lesson', 'learning_material', 'assessment_blueprint', 'curriculum', 'media', 'assignment', 'challenge', 'student'];

/** RestoreRequest.reason (OpenAPI). */
export const RESTORE_REASON = { min: 3, max: 1000 } as const;

type Page = { data: TrashItem[]; page?: { has_more?: boolean; next_cursor?: string | null } };

export function useTenantTrash(scope: SchoolLmsScope, resourceType: ResourceType) {
  const { api } = usePortalRuntime();
  return useInfiniteQuery({
    queryKey: ['school-lms', 'trash', scope.key, resourceType],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      api.call<Page>(api.client.GET('/tenant/trash', { params: { query: { resource_type: resourceType, limit: 50, ...(pageParam ? { cursor: pageParam } : {}) } }, headers: scope.headers as never })),
    getNextPageParam: (p) => (p.page?.has_more ? (p.page.next_cursor ?? undefined) : undefined),
  });
}

export function useRestoreTenantTrash(scope: SchoolLmsScope) {
  const { api } = usePortalRuntime();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ item, reason }: { item: Pick<TrashItem, 'resource_type' | 'resource_id'>; reason: string }, intent) =>
      api.call(
        api.client.POST('/tenant/trash/{resource_type}/{resource_id}:restore', {
          params: { path: { resource_type: item.resource_type, resource_id: item.resource_id } },
          body: { reason },
          headers: { ...intent, ...scope.headers } as never,
        }),
      ),
    { onSuccess: () => qc.invalidateQueries({ queryKey: ['school-lms', 'trash'] }) },
  );
}

function TrashList({ scope }: { scope: SchoolLmsScope }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const [type, setType] = useState<ResourceType>('question');
  const list = useTenantTrash(scope, type);
  const restore = useRestoreTenantTrash(scope);
  const [restoring, setRestoring] = useState<TrashItem | null>(null);
  const rows = (list.data?.pages ?? []).flatMap((p) => p.data);
  const conflict = restore.error instanceof ApiError && (restore.error.status === 409 || restore.error.code === 'CONFLICT');

  const columns: ColumnType<TrashItem>[] = [
    { key: 'title', title: t('portal.school_lms.trash.col.title'), render: (_, r) => r.title },
    { key: 'type', title: t('portal.school_lms.trash.col.type'), render: (_, r) => t(`portal.school_lms.trash.types.${r.resource_type}`, { defaultValue: r.resource_type }) },
    { key: 'deletedAt', title: t('portal.school_lms.trash.col.deletedAt'), render: (_, r) => formatDateTime(r.deleted_at) },
    { key: 'deletedBy', title: t('portal.school_lms.trash.col.deletedBy'), render: (_, r) => <Typography.Text code>{r.deleted_by.slice(-8)}</Typography.Text> },
    { key: 'reason', title: t('portal.school_lms.trash.col.reason'), render: (_, r) => r.delete_reason ?? '—' },
    {
      key: 'actions',
      title: t('portal.school_lms.common.actions'),
      render: (_, r) => {
        const button = (
          <Button size="small" disabled={r.restorable === false} onClick={() => setRestoring(r)}>
            {t('portal.school_lms.trash.restore')}
          </Button>
        );
        return (
          <ActionGate actionKey="action.school_lms.trash.restore">
            {r.restorable === false ? (
              <Tooltip title={t('portal.school_lms.trash.notRestorable')}>
                <span>
                  {button} <Typography.Text type="secondary">{t('portal.school_lms.trash.notRestorable')}</Typography.Text>
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
    <Flex vertical gap="middle">
      <Select aria-label={t('portal.school_lms.trash.col.type')} value={type} onChange={setType} options={TENANT_TRASH_TYPES.map((x) => ({ value: x, label: t(`portal.school_lms.trash.types.${x}`, { defaultValue: x }) }))} style={{ maxWidth: 320 }} />
      {list.error ? <ProblemMessage error={list.error} /> : null}
      <ResponsiveTable<TrashItem>
        rowKey="resource_id"
        columns={columns}
        data={rows}
        loading={list.isFetching}
        cardTitle={(r) => r.title}
        hasMore={list.hasNextPage}
        onLoadMore={() => void list.fetchNextPage()}
        loadMoreLabel={t('portal.school_lms.common.loadMore')}
      />
      {!list.isFetching && rows.length === 0 && <Typography.Text type="secondary">{t('portal.school_lms.trash.empty')}</Typography.Text>}
      <ReasonDialog
        open={!!restoring}
        title={restoring ? t('portal.school_lms.trash.restoreTitle', { title: restoring.title }) : ''}
        maxLength={RESTORE_REASON.max}
        minLength={RESTORE_REASON.min}
        confirmLoading={restore.isPending}
        error={
          restore.error ? (
            <>
              <ProblemMessage error={restore.error} />
              {conflict ? <Alert type="warning" showIcon message={t('portal.school_lms.trash.notRestorable')} /> : null}
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
                message.success(t('portal.school_lms.trash.restored'));
                setRestoring(null);
              },
            },
          )
        }
      />
    </Flex>
  );
}

export function TenantTrashPage({ title }: { title: string }) {
  const { t } = useTranslation();
  return (
    <section aria-labelledby="ew-sl-trash">
      <Flex vertical gap="middle">
        <Typography.Title level={1} id="ew-sl-trash">
          {title}
        </Typography.Title>
        <Typography.Paragraph type="secondary">{t('portal.school_lms.trash.help')}</Typography.Paragraph>
        <SingleTenantGate>{(scope) => <TrashList scope={scope} />}</SingleTenantGate>
      </Flex>
    </section>
  );
}
