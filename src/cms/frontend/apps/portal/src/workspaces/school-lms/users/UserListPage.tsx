// "Người dùng" (F052 T032, F052-AC-12, F052-AC-13, F052-AC-16): listTenantUsers filtered by q / type / school / role;
// students are read-only with the "SIS" tag and never get a portal-role action (ADR-0066); staff open their UserRole
// panel and, with edit_users, "Đổi email" (OQ F012-1). "Thêm nhân sự" behind ActionGate edit_users. Roles offered come from grantableRoles (display only).
import { Button, Flex, Form, Input, Select, Space } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { usePermissions } from '../../../permissions/usePermissions';
import { useMyPermissions } from '../../../shared/api/hooks/useMyPermissions';
import { QueryError } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { ResponsiveTable } from '../common/ResponsiveTable';
import { SchoolSelect } from '../common/SchoolSelect';
import { SourceTag, StatusTag } from '../common/ui';
import { useRoleList } from '../roles/useRoles';
import { grantableRoles, grantMode } from './canGrant';
import { ChangeEmailDialog } from './ChangeEmailDialog';
import { CreateStaffForm } from './CreateStaffForm';
import { UserRolesPanel } from './UserRolesPanel';
import { useUserList, type User, type UserFilters } from './useUsers';

export function UserListPage() {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const [filters, setFilters] = useState<UserFilters>({ userType: 'staff' });
  const users = useUserList(filters);
  const roles = useRoleList();
  const grantable = grantableRoles(roles.items, view?.permissions ?? []);
  const mode = grantMode(useMyPermissions().data?.roles);
  const [adding, setAdding] = useState(false);
  const [selected, setSelected] = useState<User | null>(null);
  const [emailOf, setEmailOf] = useState<User | null>(null);
  const columns: ColumnType<User>[] = [
    { key: 'title', title: t('portal.school_lms.users.displayName'), render: (_, u) => u.display_name ?? u.username },
    { key: 'username', title: t('portal.school_lms.users.username'), dataIndex: 'username' },
    { key: 'email', title: t('portal.school_lms.users.email'), render: (_, u) => (u.user_type === 'staff' ? (u.email ?? '') : '') },
    { key: 'type', title: t('portal.school_lms.users.type'), render: (_, u) => t(`portal.school_lms.users.types.${u.user_type}`) },
    { key: 'roles', title: t('portal.school_lms.users.roles'), render: (_, u) => (u.user_roles ?? []).map((r) => `${r.role_code} (${t(`portal.school_lms.userRoles.scopes.${r.scope_type}`)})`).join(', ') },
    { key: 'status', title: t('portal.school_lms.users.status'), render: (_, u) => <StatusTag group="user" value={u.status} /> },
    { key: 'source', title: t('portal.school_lms.common.source'), render: (_, u) => <SourceTag source={u.source} /> },
    {
      key: 'actions',
      title: t('portal.school_lms.common.actions'),
      render: (_, u) =>
        u.user_type === 'staff' ? (
          <Space size="small" wrap>
            <ActionGate actionKey="action.school_lms.access_control.view_user_roles">
              <Button size="small" onClick={() => setSelected(u)}>
                {t('portal.school_lms.userRoles.manage')}
              </Button>
            </ActionGate>
            <ActionGate actionKey="action.school_lms.access_control.edit_users">
              <Button size="small" onClick={() => setEmailOf(u)}>
                {t('portal.school_lms.users.changeEmail')}
              </Button>
            </ActionGate>
          </Space>
        ) : null,
    },
  ];
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Flex wrap="wrap" gap="middle" align="end">
        <Form.Item label={t('portal.school_lms.common.search')}>
          <Input.Search allowClear placeholder={t('portal.school_lms.users.searchHint')} maxLength={FIELDS.userSearch.maxLength} onSearch={(q) => setFilters((f) => ({ ...f, q }))} />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.users.type')}>
          <Select
            allowClear
            aria-label={t('portal.school_lms.users.type')}
            value={filters.userType}
            onChange={(userType) => setFilters((f) => ({ ...f, userType }))}
            options={(['staff', 'student'] as const).map((x) => ({ value: x, label: t(`portal.school_lms.users.types.${x}`) }))}
          />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.common.school')}>
          <SchoolSelect value={filters.schoolId} onChange={(schoolId) => setFilters((f) => ({ ...f, schoolId }))} allowClear autoSelect={false} />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.users.role')}>
          <Select allowClear aria-label={t('portal.school_lms.users.role')} value={filters.roleCode} onChange={(roleCode) => setFilters((f) => ({ ...f, roleCode }))} options={roles.items.map((r) => ({ value: r.code, label: r.code }))} />
        </Form.Item>
        <ActionGate actionKey="action.school_lms.access_control.edit_users">
          <Button type="primary" onClick={() => setAdding(true)}>
            {t('portal.school_lms.users.addStaff')}
          </Button>
        </ActionGate>
      </Flex>
      {users.error ? <QueryError error={users.error} /> : null}
      <ResponsiveTable<User>
        rowKey="user_id"
        columns={columns}
        data={users.items}
        loading={users.isFetching}
        cardTitle={(u) => u.display_name ?? u.username}
        hasMore={users.hasMore}
        onLoadMore={users.loadMore}
      />
      {adding ? <CreateStaffForm open roles={grantable} mode={mode} onClose={() => setAdding(false)} /> : null}
      {emailOf ? <ChangeEmailDialog user={emailOf} onClose={() => setEmailOf(null)} /> : null}
      {selected ? <UserRolesPanel user={selected} roles={grantable} mode={mode} onClose={() => setSelected(null)} /> : null}
    </Space>
  );
}
