// PortalLayout (F051 T031, T035, F051-AC-09/16/17/18/19, ADMIN_PORTAL §1.3, §1.5): topbar (product, workspace
// switcher, working tenant or System Admin tenant filter, user menu), banner slot, sidebar of the current workspace
// built from the generated catalog filtered by getMyPermissions, breadcrumb, main content. Desktop ≥ 1280 px: fixed
// sidebar; < 1280 px: sidebar in a drawer (☰); < 768 px: tenant controls move into the drawer too; touch targets
// ≥ 44 px below desktop. Skip link, landmarks and labels for keyboard/screen readers. No business logic (AGENTS R1).
import { MenuOutlined, UserOutlined } from '@ant-design/icons';
import { Breadcrumb, Button, Drawer, Flex, Layout, Menu, Popover, Segmented, Select, Switch, Typography, theme } from 'antd';
import { useEffect, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { usePreferences } from '../app/PreferencesProvider';
import { LanguageSelect } from '../auth/AuthLayout';
import type { WorkspaceDef } from '../generated/menu-catalog';
import { menuForPath, visibleMenus, visibleWorkspaces, workspaceForPath } from '../permissions/menuRegistry';
import { Loading } from '../permissions/RequirePermission';
import { usePermissions } from '../permissions/usePermissions';
import { INTERNAL_WORKSPACES } from '../permissions/workspaceRegistry';
import { useSession } from '../session/SessionProvider';
import { useMe } from '../shared/api/hooks/useMe';
import { readItem, STORAGE_KEYS, writeItem } from '../shared/storage';
import { ProblemMessage } from './ProblemMessage';
import { NoAccessPage, VpnRequiredNotice } from './StatusPages';
import { TenantFilter, TenantSwitcher } from './TenantControls';
import { useMediaQuery } from './useMediaQuery';
import './shell.css';

/** ADMIN_PORTAL §1.5 [Proposed]: desktop layout from 1280 px; touch targets ≥ 44 px below it (no admin token yet). */
export const DESKTOP_MIN_WIDTH_PX = 1280;
export const TOUCH_TARGET_PX = 44;
export const MAIN_ID = 'ew-main';

/** Banner slot (ADMIN_PORTAL §1.3): visible on every breakpoint; F122 adds "Đang tắt kiểm duyệt" here. */
export function BannerSlot({ children }: { children?: ReactNode }) {
  const { t } = useTranslation();
  if (!children) return null;
  return (
    <section aria-label={t('shell.layout.banner')} data-testid="banner-slot">
      {children}
    </section>
  );
}

export function WorkspaceSwitcher({ workspaces, current }: { workspaces: WorkspaceDef[]; current?: WorkspaceDef }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { userKey } = useSession();
  return (
    <Select
      aria-label={t('shell.layout.workspace')}
      value={current?.key}
      placeholder={t('shell.layout.workspace')}
      popupMatchSelectWidth={false}
      onChange={(key) => {
        const ws = workspaces.find((w) => w.key === key);
        if (!ws) return;
        if (userKey) writeItem('local', STORAGE_KEYS.lastWorkspace(userKey), ws.key);
        navigate(ws.path);
      }}
      options={workspaces.map((w) => ({ value: w.key, label: t(w.labelKey) }))}
    />
  );
}

/** User menu (ADMIN_PORTAL §1.3): language, light/dark, compact, sign out — applied without reload (AC-16/17). */
export function UserMenu() {
  const { t } = useTranslation();
  const session = useSession();
  const navigate = useNavigate();
  const me = useMe();
  const { preferences, update } = usePreferences();
  const [open, setOpen] = useState(false);
  const name = me.data?.display_name ?? t('shell.userMenu.label');
  const content = (
    <Flex vertical gap="middle">
      <Typography.Text strong>{name}</Typography.Text>
      <LanguageSelect />
      <Segmented
        aria-label={t('shell.userMenu.theme')}
        value={preferences.theme}
        onChange={(v) => update({ theme: v as 'light' | 'dark' })}
        options={[
          { value: 'light', label: t('shell.userMenu.light') },
          { value: 'dark', label: t('shell.userMenu.dark') },
        ]}
      />
      <Flex gap="small" align="center" justify="space-between">
        <span id="ew-compact-label">{t('shell.userMenu.compact')}</span>
        <Switch aria-labelledby="ew-compact-label" checked={preferences.density === 'compact'} onChange={(c) => update({ density: c ? 'compact' : 'default' })} />
      </Flex>
      <Button
        danger
        onClick={async () => {
          setOpen(false);
          await session.logout();
          navigate('/login', { replace: true });
        }}
      >
        {t('shell.userMenu.logout')}
      </Button>
    </Flex>
  );
  return (
    <Popover content={content} trigger="click" open={open} onOpenChange={setOpen} placement="bottomRight">
      <Button icon={<UserOutlined />} aria-label={t('shell.userMenu.label')} aria-expanded={open}>
        {name}
      </Button>
    </Popover>
  );
}

function SideMenu({ workspace, onNavigate }: { workspace: WorkspaceDef; onNavigate?: () => void }) {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const location = useLocation();
  const menus = view ? visibleMenus(workspace, view) : [];
  const selected = menuForPath(location.pathname);
  return (
    <nav aria-label={t('shell.layout.navigation')}>
      <Menu
        mode="inline"
        selectedKeys={selected ? [selected.key] : []}
        items={menus.map((m) => ({
          key: m.key,
          label: (
            <Link to={m.path} onClick={onNavigate}>
              {t(m.labelKey)}
            </Link>
          ),
        }))}
      />
    </nav>
  );
}

/** `/` → last workspace of this user (if still visible) or the first visible one. */
export function HomeRedirect() {
  const { view } = usePermissions();
  const { userKey } = useSession();
  if (!view) return <Loading />;
  const list = visibleWorkspaces(view);
  const last = userKey ? readItem('local', STORAGE_KEYS.lastWorkspace(userKey)) : null;
  const target = list.find((w) => w.key === last) ?? list[0];
  return target ? <Navigate to={target.path} replace /> : null;
}

export function PortalLayout() {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const session = useSession();
  const navigate = useNavigate();
  const location = useLocation();
  const { view, isLoading, error } = usePermissions();
  const desktop = useMediaQuery(`(min-width: ${DESKTOP_MIN_WIDTH_PX}px)`);
  const mobile = !useMediaQuery(`(min-width: ${token.screenMD}px)`);
  const [drawer, setDrawer] = useState(false);
  const workspace = workspaceForPath(location.pathname);
  const current = workspace?.key;
  const { dismissVpn } = session;

  useEffect(() => setDrawer(false), [location.pathname]);
  // the VPN notice belongs to the internal workspaces only (F051-AC-14)
  useEffect(() => {
    if (current && !INTERNAL_WORKSPACES.includes(current)) dismissVpn();
  }, [current, dismissVpn]);

  if (isLoading) return <Loading />;
  if (error || !view) return <ProblemMessage error={error} />;
  const workspaces = visibleWorkspaces(view);
  if (workspaces.length === 0) {
    return <NoAccessPage reason="no_role" onLogout={() => void session.logout().then(() => navigate('/login', { replace: true }))} />;
  }
  const menu = menuForPath(location.pathname);
  const touch = desktop ? undefined : { minHeight: TOUCH_TARGET_PX, minWidth: TOUCH_TARGET_PX };
  const tenantControls = (
    <>
      <TenantSwitcher workspace={workspace} />
      <TenantFilter />
    </>
  );
  const vpn = session.vpnRequired && !!current && INTERNAL_WORKSPACES.includes(current);

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <a href={`#${MAIN_ID}`} className="ew-skip-link">
        {t('shell.layout.skipToContent')}
      </a>
      <Layout.Header
        style={{
          background: token.colorBgContainer,
          borderBottom: `${token.lineWidth}px ${token.lineType} ${token.colorBorderSecondary}`,
          paddingInline: token.paddingLG,
          paddingBlock: token.paddingSM,
          height: 'auto',
          lineHeight: 'normal',
        }}
      >
        <Flex align="center" gap="middle" wrap justify="space-between">
          <Flex align="center" gap="middle" wrap>
            {!desktop ? <Button icon={<MenuOutlined />} aria-label={t('shell.layout.openMenu')} onClick={() => setDrawer(true)} style={touch} /> : null}
            <Typography.Text strong>{t('app.shortTitle')}</Typography.Text>
            <WorkspaceSwitcher workspaces={workspaces} current={workspace} />
            {!mobile ? tenantControls : null}
          </Flex>
          <UserMenu />
        </Flex>
      </Layout.Header>
      <BannerSlot>{vpn ? <VpnRequiredNotice onClose={dismissVpn} /> : null}</BannerSlot>
      <Layout>
        {desktop && workspace ? (
          <Layout.Sider width={token.screenXS / 2} style={{ background: token.colorBgContainer }}>
            <SideMenu workspace={workspace} />
          </Layout.Sider>
        ) : null}
        <Drawer open={!desktop && drawer} onClose={() => setDrawer(false)} placement="left" title={workspace ? t(workspace.labelKey) : t('app.shortTitle')}>
          <Flex vertical gap="middle">
            {mobile ? tenantControls : null}
            {workspace ? <SideMenu workspace={workspace} onNavigate={() => setDrawer(false)} /> : null}
          </Flex>
        </Drawer>
        <Layout.Content id={MAIN_ID} tabIndex={-1} style={{ padding: token.paddingLG }}>
          <Breadcrumb
            items={[...(workspace ? [{ title: <Link to={workspace.path}>{t(workspace.labelKey)}</Link> }] : []), ...(menu ? [{ title: t(menu.labelKey) }] : [])]}
          />
          <div style={{ marginTop: token.marginMD }}>
            <Outlet />
          </div>
        </Layout.Content>
      </Layout>
    </Layout>
  );
}
