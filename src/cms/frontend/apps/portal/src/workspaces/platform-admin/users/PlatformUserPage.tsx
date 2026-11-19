// Accounts (F059 T034, F059-AC-11…14, FR-AUTH-032, FR-AUTH-010): the cross-tenant list needs `listPlatformUsers`
// (delta owner F012, not in the contract yet — Câu hỏi mở F059-Q1), so this tab offers what the contract allows:
// accounts of one tenant (listTenantUsers with X-Tenant-Id = that tenant; `q` 2–100, type, "SIS" tag; students read
// only, no role button), lookup of any account by id (detail drawer: UserRoles of every tenant, lock/unlock/deactivate)
// and "Tạo nhân sự trường" (createTenantUser in the target tenant). "Nhân sự platform" creation waits for
// `createPlatformUser`.
import { Alert, Button, Form, Input, Select, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useTenantDirectory } from '../api/tenants';
import { PLATFORM_USERS_AVAILABLE, type User } from '../api/users';
import { useTenantStaff } from '../api/userRoles';
import { isUuid, LIMITS } from '../limits';
import { limitInputProps, TenantCell, TenantSelect, useLargeScreen } from '../ui';
import { GrantUserRoleDialog } from '../user-roles/GrantUserRoleDialog';
import { accountKindOf } from '../user-roles/grantableRoles';
import { CreateUserDialog } from './CreateUserDialog';
import { UserDetailDrawer } from './UserDetailDrawer';

type UserType = NonNullable<User['user_type']>;
const USER_TYPES: readonly UserType[] = ['staff', 'student'];

export function PlatformUserPage() {
  const { t } = useTranslation();
  const large = useLargeScreen();
  const directory = useTenantDirectory();
  const [tenantId, setTenantId] = useState<string | undefined>();
  const [q, setQ] = useState('');
  const [userType, setUserType] = useState<UserType>('staff');
  const accounts = useTenantStaff(tenantId, q, userType);
  const [detail, setDetail] = useState<{ userId: string; user?: User } | null>(null);
  const [granting, setGranting] = useState<User | null>(null);
  const [creating, setCreating] = useState(false);
  const [lookupForm] = Form.useForm<{ user_id: string }>();

  const columns: ColumnsType<User> = [
    { key: 'name', title: t('platform.users.displayName'), render: (_, u) => u.display_name },
    { key: 'username', title: t('platform.users.username'), render: (_, u) => <Typography.Text code>{u.username}</Typography.Text> },
    { key: 'email', title: t('platform.users.email'), render: (_, u) => u.email },
    { key: 'home', title: t('platform.users.homeTenant'), render: (_, u) => (u.primary_tenant_id ? <TenantCell tenantId={u.primary_tenant_id} directory={directory} /> : <Tag>{t('platform.users.platformStaff')}</Tag>) },
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
      {PLATFORM_USERS_AVAILABLE ? null : <Alert type="info" showIcon message={t('platform.users.listUnavailable')} />}
      <Space wrap>
        <TenantSelect allowClear ariaLabel={t('platform.users.tenant')} value={tenantId} onChange={(v) => setTenantId(v as string)} />
        <Select aria-label={t('platform.users.userType')} value={userType} onChange={setUserType} options={USER_TYPES.map((x) => ({ value: x, label: t(`platform.users.type.${x}`) }))} />
        <Input.Search
          aria-label={t('platform.users.search')}
          placeholder={t('platform.users.search')}
          allowClear
          onSearch={setQ}
          {...limitInputProps(LIMITS.tenantUserSearch)}
        />
        {large ? (
          <ActionGate actionKey="action.school_lms.access_control.edit_users">
            <Button type="primary" onClick={() => setCreating(true)}>
              {t('platform.users.createStaff')}
            </Button>
          </ActionGate>
        ) : (
          <Alert type="info" showIcon message={t('platform.common.largeScreenReadOnly')} />
        )}
      </Space>
      {q && q.trim().length < (LIMITS.tenantUserSearch.minLength ?? 0) ? <Typography.Text type="secondary">{t('platform.users.searchMin', { min: LIMITS.tenantUserSearch.minLength })}</Typography.Text> : null}
      {accounts.error ? <ProblemMessage error={accounts.error} /> : null}
      {tenantId ? (
        <Table rowKey="user_id" columns={columns} dataSource={accounts.data ?? []} loading={accounts.isFetching} pagination={false} locale={{ emptyText: t('platform.common.empty') }} />
      ) : (
        <Typography.Paragraph type="secondary">{t('platform.users.pickTenant')}</Typography.Paragraph>
      )}
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
      {granting ? <GrantUserRoleDialog preset={{ kind: accountKindOf(granting), userId: granting.user_id, tenantId: granting.primary_tenant_id ?? tenantId }} onClose={() => setGranting(null)} /> : null}
      {creating ? <CreateUserDialog onClose={() => setCreating(false)} /> : null}
    </section>
  );
}

