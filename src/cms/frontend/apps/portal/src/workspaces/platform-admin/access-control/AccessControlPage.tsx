// Menu "Người dùng & phân quyền (platform)" (F059 US1–US3, ADMIN_PORTAL §3.17): tabs Người dùng, Role, Permission
// catalog and UserRole, each behind its catalog action (ADR-0090) — a tab whose action is not granted or turned off is
// not rendered. The open tab is kept in the URL (`?tab=`) so a link can be shared inside the team.
import { Tabs } from 'antd';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';
import type { ActionKey } from '../../../generated/menu-catalog';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { PermissionCatalogPage } from '../permissions/PermissionCatalogPage';
import { PlatformRoleListPage } from '../roles/PlatformRoleListPage';
import { UserRoleListPage } from '../user-roles/UserRoleListPage';
import { PlatformUserPage } from '../users/PlatformUserPage';
import { PageTitle } from '../ui';

const TABS: { key: string; action: ActionKey; label: string; Screen: () => JSX.Element }[] = [
  { key: 'users', action: 'action.platform_admin.access_control.view_users', label: 'platform.access.tabUsers', Screen: PlatformUserPage },
  { key: 'roles', action: 'action.platform_admin.access_control.view_roles', label: 'platform.access.tabRoles', Screen: PlatformRoleListPage },
  { key: 'permissions', action: 'action.platform_admin.access_control.view_roles', label: 'platform.access.tabPermissions', Screen: PermissionCatalogPage },
  { key: 'user-roles', action: 'action.platform_admin.access_control.view_user_roles', label: 'platform.access.tabUserRoles', Screen: UserRoleListPage },
];

export function AccessControlPage({ title }: { title: string }) {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const [params, setParams] = useSearchParams();
  const items = TABS.filter((x) => view && actionStatus(x.action, view) === 'enabled').map(({ key, label, Screen }) => ({ key, label: t(label), children: <Screen /> }));
  const requested = params.get('tab');
  const active = items.some((x) => x.key === requested) ? requested! : items[0]?.key;
  return (
    <section>
      <PageTitle title={title} />
      <Tabs items={items} activeKey={active} onChange={(k) => setParams({ tab: k }, { replace: true })} destroyOnHidden />
    </section>
  );
}
