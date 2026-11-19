// Screen of `menu.school_lms.access_control` (F052 US3; protected menu, never turned off — ADR-0090 mục 2): tabs
// "Người dùng" | "Role" | "Nhật ký phân quyền", each shown only when its catalog action is allowed.
import { Tabs } from 'antd';
import { useTranslation } from 'react-i18next';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { PermissionAuditPage } from '../audit/PermissionAuditPage';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { RoleListPage } from '../roles/RoleListPage';
import { UserListPage } from './UserListPage';

function AccessHome() {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const can = (k: Parameters<typeof actionStatus>[0]) => !!view && actionStatus(k, view) === 'enabled';
  const items = [
    can('action.school_lms.access_control.view_users') ? { key: 'users', label: t('portal.school_lms.users.tab'), children: <UserListPage /> } : null,
    can('action.school_lms.access_control.view_roles') ? { key: 'roles', label: t('portal.school_lms.roles.tab'), children: <RoleListPage /> } : null,
    can('action.school_lms.audit_logs.view') ? { key: 'audit', label: t('portal.school_lms.audit.permissionTab'), children: <PermissionAuditPage /> } : null,
  ].filter((x): x is NonNullable<typeof x> => x !== null);
  return <Tabs destroyOnHidden items={items} />;
}

export function AccessControlScreen({ title }: { title: string }) {
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>{() => <AccessHome />}</SingleTenantGate>
    </section>
  );
}
