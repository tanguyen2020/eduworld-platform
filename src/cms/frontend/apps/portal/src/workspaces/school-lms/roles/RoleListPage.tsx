// "Role" (F052 T034, F052-AC-14, F052-AC-15, F052-AC-27): system roles (label "Hệ thống", read-only) and custom roles
// [Proposed, FR-AUTH-032]; "Tạo role" behind ActionGate edit_roles; the role form needs a large screen.
import { Button, Flex, Space, Tag } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { LargeScreenRequired } from '../../../shell/StatusPages';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { QueryError } from '../common/errors';
import { formatDate, localizedName } from '../common/format';
import { MOBILE_QUERY, ResponsiveTable } from '../common/ResponsiveTable';
import { RoleEditor, type RoleEditorTarget } from './RoleEditor';
import { useRoleList, type Role } from './useRoles';

export function RoleListPage() {
  const { t, i18n } = useTranslation();
  const desktop = useMediaQuery(MOBILE_QUERY);
  const roles = useRoleList();
  const [target, setTarget] = useState<RoleEditorTarget | null>(null);
  const open = (r: Role) => setTarget({ kind: 'existing', roleId: r.role_id });
  const columns: ColumnType<Role>[] = [
    { key: 'title', title: t('portal.school_lms.roles.code'), render: (_, r) => <Button type="link" onClick={() => open(r)}>{r.code}</Button> },
    { key: 'name', title: t('portal.school_lms.roles.name'), render: (_, r) => localizedName(r.name, i18n.language) },
    { key: 'kind', title: t('portal.school_lms.roles.kind'), render: (_, r) => <Tag>{t(r.is_system ? 'portal.school_lms.roles.system' : 'portal.school_lms.roles.custom')}</Tag> },
    { key: 'users', title: t('portal.school_lms.roles.userCount'), render: (_, r) => r.active_user_role_count ?? 0 },
    { key: 'updated', title: t('portal.school_lms.roles.updatedAt'), render: (_, r) => (r.is_system ? '' : formatDate(r.updated_at)) },
  ];
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Flex justify="end">
        <ActionGate actionKey="action.school_lms.access_control.edit_roles">
          <Button type="primary" onClick={() => setTarget({ kind: 'new' })}>
            {t('portal.school_lms.roles.create')}
          </Button>
        </ActionGate>
      </Flex>
      {roles.error ? <QueryError error={roles.error} /> : null}
      <ResponsiveTable<Role> rowKey="role_id" columns={columns} data={roles.items} loading={roles.isFetching} cardTitle={(r) => r.code} hasMore={roles.hasMore} onLoadMore={roles.loadMore} />
      {target && !desktop ? <LargeScreenRequired /> : null}
      {target && desktop ? <RoleEditor key={target.kind === 'existing' ? target.roleId : 'new'} target={target} onClose={() => setTarget(null)} onClone={(draft) => setTarget({ kind: 'new', draft })} /> : null}
    </Space>
  );
}
