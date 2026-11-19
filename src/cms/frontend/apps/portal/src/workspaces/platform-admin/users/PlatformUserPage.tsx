// Accounts (F059 T034, F059-AC-11…14, FR-AUTH-032, FR-AUTH-010): listPlatformUsers (OQ F059-1). With no tenant in the
// tenant filter the list holds the platform staff only (the backend never lists every account without `tenant_ids`,
// AGENTS R8); with tenants selected it holds the members of those tenants, with their memberships (`tenant_ids`).
// Filters: `q` 2–100, account type, status; columns: name, username, email, home tenant, memberships, status, "SIS"
// tag. Students are read only (no role button). Lookup of any account by id (detail drawer: UserRoles of every
// tenant, lock/unlock/deactivate), "Tạo nhân sự" (platform staff or school staff, CreateUserDialog). Display only:
// the backend authorizes every call (AGENTS R7, R20).
import { Alert, Button, Form, Input, Select, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { TenantFilter } from '../../../shell/TenantControls';
import { useTenantDirectory } from '../api/tenants';
import { USER_STATUSES, USER_TYPES, usePlatformUsers, type PlatformUserFilter, type User, type UserStatus, type UserType } from '../api/users';
import { isUuid, LIMITS } from '../limits';
import { limitInputProps, TenantCell, useLargeScreen } from '../ui';
import { GrantUserRoleDialog, type GrantPreset } from '../user-roles/GrantUserRoleDialog';
import { accountKindOf, memberTenantsOf } from '../user-roles/grantableRoles';
import { CreateUserDialog, type CreateKind } from './CreateUserDialog';
import { UserDetailDrawer } from './UserDetailDrawer';

const grantPreset = (u: User): GrantPreset => {
  const members = memberTenantsOf(u);
  return { kind: accountKindOf(u), userId: u.user_id, tenantIds: members, tenantId: members.length === 1 ? members[0] : (u.primary_tenant_id ?? undefined) };
};

export function PlatformUserPage() {
  const { t } = useTranslation();
  const large = useLargeScreen();
  const { view } = usePermissions();
  const directory = useTenantDirectory();
  const [filter, setFilter] = useState<PlatformUserFilter>({});
  const [search, setSearch] = useState('');
  const accounts = usePlatformUsers(filter);
  const [detail, setDetail] = useState<{ userId: string; user?: User } | null>(null);
  const [granting, setGranting] = useState<User | null>(null);
  const [creating, setCreating] = useState(false);
  const [lookupForm] = Form.useForm<{ user_id: string }>();
  const createKinds: CreateKind[] = [
    ...(view && actionStatus('action.platform_admin.access_control.edit_users', view) === 'enabled' ? (['platform'] as const) : []),
    ...(view && actionStatus('action.school_lms.access_control.edit_users', view) === 'enabled' ? (['tenant'] as const) : []),
  ];

  const onSearch = (v: string) => {
    setSearch(v);
    const q = v.trim();
    setFilter((f) => ({ ...f, q: q.length >= (LIMITS.platformUserSearch.minLength ?? 0) ? q : undefined }));
  };

  const columns: ColumnsType<User> = [
    { key: 'name', title: t('platform.users.displayName'), render: (_, u) => u.display_name },
    { key: 'username', title: t('platform.users.username'), render: (_, u) => <Typography.Text code>{u.username}</Typography.Text> },
    { key: 'email', title: t('platform.users.email'), render: (_, u) => u.email },
    { key: 'home', title: t('platform.users.homeTenant'), render: (_, u) => (u.primary_tenant_id ? <TenantCell tenantId={u.primary_tenant_id} directory={directory} /> : <Tag>{t('platform.users.platformStaff')}</Tag>) },
    ...(accounts.platformOnly
      ? []
      : [
          {
            key: 'members',
            title: t('platform.users.memberTenants'),
            render: (_: unknown, u: User) => (
              <Space size="small" wrap>
                {(u.tenant_ids ?? []).map((id) => (
                  <TenantCell key={id} tenantId={id} directory={directory} />
                ))}
              </Space>
            ),
          },
        ]),
    {
      key: 'status',
      title: t('platform.common.status'),
      render: (_, u) => (
        <Space size="small">
          <Tag>{t(`platform.users.status.${u.status}`)}</Tag>
          {u.source === 'sis' ? <Tag color="processing">{t('platform.users.sis')}</Tag> : null}
        </Space>
      ),
    },
    {
      key: 'actions',
      title: t('platform.common.actions'),
      render: (_, u) => (
        <Space>
          <Button size="small" onClick={() => setDetail({ userId: u.user_id, user: u })}>
            {t('platform.users.details')}
          </Button>
          {large && u.user_type !== 'student' ? (
            <ActionGate actionKey="action.platform_admin.access_control.assign_roles">
              <Button size="small" onClick={() => setGranting(u)}>
                {t('platform.userRoles.grant')}
              </Button>
            </ActionGate>
          ) : null}
        </Space>
      ),
    },
  ];

  return (
    <section aria-label={t('platform.access.tabUsers')}>
      <Space wrap>
        <TenantFilter />
        <Select
          aria-label={t('platform.users.userType')}
          placeholder={t('platform.users.allTypes')}
          allowClear
          value={filter.userType}
          onChange={(userType?: UserType) => setFilter((f) => ({ ...f, userType }))}
          options={USER_TYPES.map((x) => ({ value: x, label: t(`platform.users.type.${x}`) }))}
        />
        <Select
          aria-label={t('platform.common.status')}
          placeholder={t('platform.users.allStatuses')}
          allowClear
          value={filter.status}
          onChange={(status?: UserStatus) => setFilter((f) => ({ ...f, status }))}
          options={USER_STATUSES.map((x) => ({ value: x, label: t(`platform.users.status.${x}`) }))}
        />
        <Input.Search aria-label={t('platform.users.search')} placeholder={t('platform.users.search')} allowClear onSearch={onSearch} {...limitInputProps(LIMITS.platformUserSearch)} />
        {large ? (
          createKinds.length ? (
            <Button type="primary" onClick={() => setCreating(true)}>
              {t('platform.users.createStaff')}
            </Button>
          ) : null
        ) : (
          <Alert type="info" showIcon message={t('platform.common.largeScreenReadOnly')} />
        )}
      </Space>
      <Typography.Paragraph type="secondary">{t(accounts.platformOnly ? 'platform.users.platformOnlyHint' : 'platform.users.membersHint')}</Typography.Paragraph>
      {search && search.trim().length < (LIMITS.platformUserSearch.minLength ?? 0) ? (
        <Typography.Text type="secondary">{t('platform.users.searchMin', { min: LIMITS.platformUserSearch.minLength })}</Typography.Text>
      ) : null}
      {accounts.error ? <ProblemMessage error={accounts.error} /> : null}
      <Table rowKey="user_id" columns={columns} dataSource={accounts.rows} loading={accounts.isFetching} pagination={false} locale={{ emptyText: t('platform.common.empty') }} />
      {accounts.hasNextPage ? (
        <Button onClick={() => accounts.fetchNextPage()} loading={accounts.isFetchingNextPage}>
          {t('platform.common.loadMore')}
        </Button>
      ) : null}
      <Form form={lookupForm} name="user_lookup" layout="inline" onFinish={(v) => setDetail({ userId: v.user_id.trim() })}>
        <Form.Item
          name="user_id"
          label={t('platform.users.lookup')}
          rules={[{ required: true, whitespace: true, message: t('platform.common.uuidInvalid') }, { validator: (_, v?: string) => (!v || isUuid(v.trim()) ? Promise.resolve() : Promise.reject(new Error(t('platform.common.uuidInvalid')))) }]}
        >
          <Input maxLength={36} />
        </Form.Item>
        <Form.Item>
          <Button htmlType="submit">{t('platform.users.open')}</Button>
        </Form.Item>
      </Form>
      {detail ? <UserDetailDrawer userId={detail.userId} user={detail.user} onClose={() => setDetail(null)} /> : null}
      {granting ? <GrantUserRoleDialog preset={grantPreset(granting)} onClose={() => setGranting(null)} /> : null}
      {creating ? <CreateUserDialog kinds={createKinds} onClose={() => setCreating(false)} /> : null}
    </section>
  );
}
