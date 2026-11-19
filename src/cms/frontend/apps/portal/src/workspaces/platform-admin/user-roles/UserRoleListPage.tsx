// UserRoles of every tenant (F059 T028, F059-AC-06/09, FR-AUTH-032, FR-AUTH-016): listAllUserRoles filtered by
// `user_id`, `role_id` and the tenant filter (`tenant_ids[]`), columns account, role, scope, tenant, validity, source,
// granted by. "Thu hồi" asks for a confirmation and a reason, then revokePlatformUserRole (X-Tenant-Id = tenant of the
// UserRole when it has one). The account id opens the detail drawer.
import { Alert, App as AntApp, Button, Form, Input, Select, Space, Table, Tag, Typography, theme } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { TenantFilter } from '../../../shell/TenantControls';
import { useRoleOptions } from '../api/roles';
import { useTenantDirectory } from '../api/tenants';
import { useRevokeUserRole, useUserRoleList, type UserRole } from '../api/userRoles';
import { isUuid, LIMITS, PRINCIPAL_CACHE_TTL_SECONDS } from '../limits';
import { formatTime, TenantCell, useLargeScreen } from '../ui';
import { UserDetailDrawer } from '../users/UserDetailDrawer';
import { GrantUserRoleDialog } from './GrantUserRoleDialog';

export function validityText(r: Pick<UserRole, 'valid_from' | 'valid_to'>, t: (k: string, o?: Record<string, unknown>) => string): string {
  return r.valid_to ? t('platform.userRoles.validRange', { from: formatTime(r.valid_from), to: formatTime(r.valid_to) }) : t('platform.userRoles.validFrom', { from: formatTime(r.valid_from) });
}

export function UserRoleListPage() {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const large = useLargeScreen();
  const { token } = theme.useToken();
  const [filterForm] = Form.useForm<{ user_id?: string; role_id?: string }>();
  const [filter, setFilter] = useState<{ userId?: string; roleId?: string }>({});
  const list = useUserRoleList(filter);
  const roles = useRoleOptions(undefined);
  const directory = useTenantDirectory();
  const revoke = useRevokeUserRole();
  const [revoking, setRevoking] = useState<UserRole | null>(null);
  const [granting, setGranting] = useState(false);
  const [detail, setDetail] = useState<string | null>(null);

  const columns: ColumnsType<UserRole> = [
    {
      key: 'user',
      title: t('platform.userRoles.user'),
      render: (_, r) => (
        <Button type="link" size="small" onClick={() => setDetail(r.user_id)}>
          <Typography.Text code>{r.user_id}</Typography.Text>
        </Button>
      ),
    },
    { key: 'role', title: t('platform.userRoles.role'), render: (_, r) => <Typography.Text code>{r.role_code}</Typography.Text> },
    {
      key: 'scope',
      title: t('platform.userRoles.scope'),
      render: (_, r) => (
        <span>
          <Tag>{t(`platform.scopeType.${r.scope_type}`)}</Tag>
          {r.scope_id ? <Typography.Text code>{r.scope_id}</Typography.Text> : null}
        </span>
      ),
    },
    { key: 'tenant', title: t('platform.common.tenant'), render: (_, r) => <TenantCell tenantId={r.tenant_id} directory={directory} /> },
    { key: 'valid', title: t('platform.userRoles.validity'), render: (_, r) => validityText(r, t) },
    { key: 'source', title: t('platform.userRoles.source'), render: (_, r) => <Tag>{t(`platform.userRoles.sourceKind.${r.source}`)}</Tag> },
    { key: 'granted_by', title: t('platform.userRoles.grantedBy'), render: (_, r) => (r.granted_by ? <Typography.Text code>{r.granted_by}</Typography.Text> : t('platform.userRoles.system')) },
  ];
  if (large) {
    columns.push({
      key: 'actions',
      title: t('platform.common.actions'),
      render: (_, r) => (
        <ActionGate actionKey="action.platform_admin.access_control.assign_roles">
          <Button size="small" danger onClick={() => setRevoking(r)}>
            {t('platform.userRoles.revoke')}
          </Button>
        </ActionGate>
      ),
    });
  }

  const apply = (v: { user_id?: string; role_id?: string }) => setFilter({ userId: v.user_id?.trim() || undefined, roleId: v.role_id || undefined });

  return (
    <section aria-label={t('platform.access.tabUserRoles')}>
      <Form form={filterForm} name="user_role_filter" layout="inline" onFinish={apply}>
        <Form.Item name="user_id" label={t('platform.userRoles.userId')} rules={[{ validator: (_, v?: string) => (!v?.trim() || isUuid(v.trim()) ? Promise.resolve() : Promise.reject(new Error(t('platform.common.uuidInvalid')))) }]}>
          <Input allowClear maxLength={36} />
        </Form.Item>
        <Form.Item name="role_id" label={t('platform.userRoles.role')}>
          <Select allowClear showSearch optionFilterProp="label" style={{ minWidth: token.controlHeightLG * 5 }} options={(roles.data ?? []).map((r) => ({ value: r.role_id, label: r.code }))} />
        </Form.Item>
        <Form.Item>
          <Button htmlType="submit">{t('platform.common.search')}</Button>
        </Form.Item>
      </Form>
      <Space wrap>
        <TenantFilter />
        {large ? (
          <ActionGate actionKey="action.platform_admin.access_control.assign_roles">
            <Button type="primary" onClick={() => setGranting(true)}>
              {t('platform.userRoles.grant')}
            </Button>
          </ActionGate>
        ) : (
          <Alert type="info" showIcon message={t('platform.common.largeScreenReadOnly')} />
        )}
      </Space>
      {list.error ? <ProblemMessage error={list.error} /> : null}
      <Table rowKey="user_role_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} locale={{ emptyText: t('platform.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('platform.common.loadMore')}
        </Button>
      ) : null}
      {granting ? <GrantUserRoleDialog onClose={() => setGranting(false)} /> : null}
      {detail ? <UserDetailDrawer userId={detail} onClose={() => setDetail(null)} /> : null}
      <ReasonDialog
        open={!!revoking}
        title={revoking ? t('platform.userRoles.revokeTitle', { role: revoking.role_code }) : ''}
        description={t('platform.userRoles.revokeConsequence', { seconds: PRINCIPAL_CACHE_TTL_SECONDS })}
        maxLength={LIMITS.revokeReason.maxLength}
        minLength={LIMITS.revokeReason.minLength}
        confirmLoading={revoke.isPending}
        error={revoke.error ? <ProblemMessage error={revoke.error} /> : null}
        onCancel={() => {
          setRevoking(null);
          revoke.reset();
        }}
        onSubmit={(reason) =>
          revoking &&
          revoke.mutate(
            { userRole: revoking, reason },
            {
              onSuccess: () => {
                message.success(t('platform.userRoles.revoked', { seconds: PRINCIPAL_CACHE_TTL_SECONDS }));
                setRevoking(null);
              },
            },
          )
        }
      />
    </section>
  );
}
