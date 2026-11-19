// Platform and shared roles (F059 T020, F059-AC-02/04/05, FR-AUTH-032, ADMIN_PORTAL §3.17): listPlatformRoles by
// `role_scope_kind` and the tenant filter (`tenant_ids[]`, no X-Tenant-Id), Tenant column ("Dùng chung" when
// `tenant_id = null`), "Hệ thống" tag, active UserRole count. A system role only offers "Nhân bản thành role tùy
// biến"; a custom role of a tenant is read only here (edited in workspace C, F052). Delete asks for a confirmation and
// a reason (soft delete, AGENTS R21); `409` (role still in use) and `403` are shown by their code.
import { Alert, App as AntApp, Button, Select, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { TenantFilter } from '../../../shell/TenantControls';
import { SCOPE_KINDS, useDeleteRole, useRoleList, type PermissionScopeKind, type Role } from '../api/roles';
import { useTenantDirectory } from '../api/tenants';
import { LIMITS, PRINCIPAL_CACHE_TTL_SECONDS } from '../limits';
import { formatTime, useLargeScreen } from '../ui';
import { PlatformRoleEditor, type EditorTarget } from './PlatformRoleEditor';

export function PlatformRoleListPage() {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const large = useLargeScreen();
  const [scopeKind, setScopeKind] = useState<PermissionScopeKind | undefined>();
  const list = useRoleList(scopeKind);
  const directory = useTenantDirectory();
  const remove = useDeleteRole();
  const [editing, setEditing] = useState<EditorTarget | null>(null);
  const [deleting, setDeleting] = useState<Role | null>(null);
  const inUse = remove.error instanceof ApiError && remove.error.status === 409;

  const columns: ColumnsType<Role> = [
    { key: 'code', title: t('platform.roles.code'), render: (_, r) => <Typography.Text code>{r.code}</Typography.Text> },
    { key: 'name', title: t('platform.roles.name'), render: (_, r) => (i18n.language.startsWith('en') && r.name.en ? r.name.en : r.name.vi) },
    { key: 'scope', title: t('platform.roles.scopeKind'), render: (_, r) => <Tag>{t(`platform.scopeKind.${r.role_scope_kind}`)}</Tag> },
    {
      key: 'tenant',
      title: t('platform.common.tenant'),
      render: (_, r) => (r.tenant_id ? directory.get(r.tenant_id)?.name ?? <Typography.Text code>{r.tenant_id}</Typography.Text> : <Tag>{t('platform.roles.shared')}</Tag>),
    },
    { key: 'system', title: t('platform.roles.kind'), render: (_, r) => (r.is_system ? <Tag color="processing">{t('platform.roles.system')}</Tag> : <Tag>{t('platform.roles.custom')}</Tag>) },
    { key: 'count', title: t('platform.roles.userCount'), render: (_, r) => r.active_user_role_count ?? 0 },
    { key: 'updated', title: t('platform.roles.updatedAt'), render: (_, r) => formatTime(r.updated_at) },
  ];
  if (large) {
    columns.push({
      key: 'actions',
      title: t('platform.common.actions'),
      render: (_, r) => {
        if (r.tenant_id) return <Typography.Text type="secondary">{t('platform.roles.tenantReadOnly')}</Typography.Text>;
        return (
          <ActionGate actionKey="action.platform_admin.access_control.edit_roles">
            {r.is_system ? (
              <Button size="small" onClick={() => setEditing({ mode: 'clone', source: r })}>
                {t('platform.roles.clone')}
              </Button>
            ) : (
              <Space>
                <Button size="small" onClick={() => setEditing({ mode: 'edit', roleId: r.role_id })}>
                  {t('platform.common.edit')}
                </Button>
                <Button size="small" danger onClick={() => setDeleting(r)}>
                  {t('platform.roles.delete')}
                </Button>
              </Space>
            )}
          </ActionGate>
        );
      },
    });
  }

  return (
    <section aria-label={t('platform.access.tabRoles')}>
      <Space wrap>
        <Select
          aria-label={t('platform.roles.scopeKind')}
          allowClear
          placeholder={t('platform.permissions.allScopes')}
          value={scopeKind}
          onChange={setScopeKind}
          options={SCOPE_KINDS.map((x) => ({ value: x, label: t(`platform.scopeKind.${x}`) }))}
        />
        <TenantFilter />
        {large ? (
          <ActionGate actionKey="action.platform_admin.access_control.edit_roles">
            <Button type="primary" onClick={() => setEditing({ mode: 'create' })}>
              {t('platform.roles.create')}
            </Button>
          </ActionGate>
        ) : (
          <Alert type="info" showIcon message={t('platform.common.largeScreenReadOnly')} />
        )}
      </Space>
      {list.error ? <ProblemMessage error={list.error} /> : null}
      <Table rowKey="role_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} locale={{ emptyText: t('platform.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('platform.common.loadMore')}
        </Button>
      ) : null}
      {editing ? <PlatformRoleEditor target={editing} onClose={() => setEditing(null)} /> : null}
      <ReasonDialog
        open={!!deleting}
        title={deleting ? t('platform.roles.deleteTitle', { code: deleting.code }) : ''}
        description={deleting ? t('platform.roles.deleteConsequence', { users: deleting.active_user_role_count ?? 0 }) : undefined}
        maxLength={LIMITS.roleDeleteReason.maxLength}
        minLength={LIMITS.roleDeleteReason.minLength}
        confirmLoading={remove.isPending}
        error={
          remove.error ? (
            <>
              <ProblemMessage error={remove.error} />
              {inUse ? <Alert type="warning" showIcon message={t('platform.roles.inUse')} /> : null}
            </>
          ) : null
        }
        onCancel={() => {
          setDeleting(null);
          remove.reset();
        }}
        onSubmit={(reason) =>
          deleting &&
          remove.mutate(
            { role: deleting, reason },
            {
              onSuccess: () => {
                message.success(t('platform.roles.appliesWithin', { seconds: PRINCIPAL_CACHE_TTL_SECONDS, users: deleting.active_user_role_count ?? 0 }));
                setDeleting(null);
              },
            },
          )
        }
      />
    </section>
  );
}
