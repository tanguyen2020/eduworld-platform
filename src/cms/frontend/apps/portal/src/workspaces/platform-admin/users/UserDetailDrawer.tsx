// Account detail (F059 T034, F059-AC-13/14, AGENTS R14, ADR-0031): profile fields of the account when known (from
// listTenantUsers), its UserRoles across tenants (listAllUserRoles?user_id=) grouped by tenant, last login, and the
// account actions Khóa / Mở khóa / Vô hiệu hóa (reason, Idempotency-Key, X-Tenant-Id = home tenant when known).
// Never shows a password, token or IP (none is in the contract either). Students get no "Gán role" button.
import { App as AntApp, Button, Descriptions, Drawer, Space, Table, Tag, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { useTenantDirectory } from '../api/tenants';
import { groupByTenant, useUserRolesOf } from '../api/userRoles';
import { ACCOUNT_ACTIONS, useAccountAction, type AccountAction, type User } from '../api/users';
import { LIMITS, type FieldLimit } from '../limits';
import { formatTime, TenantCell, useLargeScreen } from '../ui';
import { GrantUserRoleDialog } from '../user-roles/GrantUserRoleDialog';
import { accountKindOf, canHavePortalRole } from '../user-roles/grantableRoles';
import { validityText } from '../user-roles/UserRoleListPage';

const REASON_LIMIT: Record<AccountAction, FieldLimit> = { lock: LIMITS.lockReason, unlock: LIMITS.unlockReason, deactivate: LIMITS.deactivateReason };

export function UserDetailDrawer({ userId, user, onClose }: { userId: string; user?: User; onClose: () => void }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const large = useLargeScreen();
  const roles = useUserRolesOf(userId);
  const directory = useTenantDirectory();
  const action = useAccountAction();
  const [pending, setPending] = useState<AccountAction | null>(null);
  const [granting, setGranting] = useState(false);
  const student = user?.user_type === 'student';

  return (
    <Drawer open title={t('platform.users.detailTitle')} width={720} onClose={onClose} destroyOnHidden>
      <Descriptions column={1} size="small" bordered>
        <Descriptions.Item label={t('platform.users.userId')}>
          <Typography.Text code>{userId}</Typography.Text>
        </Descriptions.Item>
        {user ? (
          <>
            <Descriptions.Item label={t('platform.users.displayName')}>{user.display_name}</Descriptions.Item>
            <Descriptions.Item label={t('platform.users.username')}>{user.username}</Descriptions.Item>
            <Descriptions.Item label={t('platform.users.email')}>{user.email}</Descriptions.Item>
            <Descriptions.Item label={t('platform.users.userType')}>{t(`platform.users.type.${user.user_type}`)}</Descriptions.Item>
            <Descriptions.Item label={t('platform.users.homeTenant')}>
              {user.primary_tenant_id ? <TenantCell tenantId={user.primary_tenant_id} directory={directory} /> : <Tag>{t('platform.users.platformStaff')}</Tag>}
            </Descriptions.Item>
            <Descriptions.Item label={t('platform.common.status')}>{t(`platform.users.status.${user.status}`)}</Descriptions.Item>
            <Descriptions.Item label={t('platform.users.source')}>{user.source === 'sis' ? <Tag color="processing">{t('platform.users.sis')}</Tag> : t(`platform.users.sourceKind.${user.source ?? 'manual'}`)}</Descriptions.Item>
            <Descriptions.Item label={t('platform.users.lastLogin')}>{user.last_login_at ? formatTime(user.last_login_at) : t('platform.users.never')}</Descriptions.Item>
          </>
        ) : null}
      </Descriptions>
      {large ? (
        <Space wrap>
          <ActionGate actionKey="action.platform_admin.access_control.lock_user">
            <Space>
              {ACCOUNT_ACTIONS.map((a) => (
                <Button key={a} size="small" danger={a !== 'unlock'} onClick={() => setPending(a)}>
                  {t(`platform.users.action.${a}`)}
                </Button>
              ))}
            </Space>
          </ActionGate>
          {!student && canHavePortalRole(user) ? (
            <ActionGate actionKey="action.platform_admin.access_control.assign_roles">
              <Button size="small" type="primary" onClick={() => setGranting(true)}>
                {t('platform.userRoles.grant')}
              </Button>
            </ActionGate>
          ) : null}
        </Space>
      ) : null}
      <Typography.Title level={2}>{t('platform.users.userRoles')}</Typography.Title>
      {roles.error ? <ProblemMessage error={roles.error} /> : null}
      {groupByTenant(roles.data ?? []).map(([tenantId, rows]) => (
        <section key={tenantId ?? 'platform'}>
          <TenantCell tenantId={tenantId} directory={directory} />
          <Table
            rowKey="user_role_id"
            size="small"
            pagination={false}
            dataSource={rows}
            columns={[
              { key: 'role', title: t('platform.userRoles.role'), render: (_, r) => <Typography.Text code>{r.role_code}</Typography.Text> },
              { key: 'scope', title: t('platform.userRoles.scope'), render: (_, r) => t(`platform.scopeType.${r.scope_type}`) },
              { key: 'valid', title: t('platform.userRoles.validity'), render: (_, r) => validityText(r, t) },
              { key: 'source', title: t('platform.userRoles.source'), render: (_, r) => t(`platform.userRoles.sourceKind.${r.source}`) },
            ]}
          />
        </section>
      ))}
      {!roles.isFetching && !roles.data?.length ? <Typography.Text type="secondary">{t('platform.users.noUserRoles')}</Typography.Text> : null}
      <ReasonDialog
        open={!!pending}
        title={pending ? t(`platform.users.actionTitle.${pending}`) : ''}
        description={pending ? t(`platform.users.actionConsequence.${pending}`) : undefined}
        maxLength={(pending ? REASON_LIMIT[pending] : LIMITS.lockReason).maxLength ?? LIMITS.lockReason.maxLength}
        minLength={pending ? REASON_LIMIT[pending].minLength : undefined}
        confirmLoading={action.isPending}
        error={action.error ? <ProblemMessage error={action.error} /> : null}
        onCancel={() => {
          setPending(null);
          action.reset();
        }}
        onSubmit={(reason) =>
          pending &&
          action.mutate(
            { action: pending, userId, homeTenantId: user?.primary_tenant_id, reason },
            {
              onSuccess: () => {
                message.success(t(`platform.users.actionDone.${pending}`));
                setPending(null);
              },
            },
          )
        }
      />
      {granting ? (
        <GrantUserRoleDialog
          preset={user ? { kind: accountKindOf(user), userId, tenantId: user.primary_tenant_id ?? undefined } : { userId }}
          onClose={() => setGranting(false)}
        />
      ) : null}
    </Drawer>
  );
}
