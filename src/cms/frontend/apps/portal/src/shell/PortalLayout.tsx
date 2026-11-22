// PortalLayout (F051 T031, T035, F051-AC-09/16/17/18/19, ADMIN_PORTAL §1.3, §1.5): topbar (product, workspace
// switcher, working tenant or System Admin tenant filter, user menu), banner slot, sidebar of the current workspace
// built from the generated catalog filtered by getMyPermissions, breadcrumb, main content. Desktop ≥ 1280 px: fixed
// sidebar; < 1280 px: sidebar in a drawer (☰); < 768 px: tenant controls move into the drawer too; touch targets
// ≥ 44 px below desktop. Skip link, landmarks and labels for keyboard/screen readers. No business logic (AGENTS R1).
// Look of direction B "Sổ tay giáo viên" (DESIGN_SYSTEM §4.2, owner 2026-11-21): topbar and sidebar on the container
// colour with a 1 px subtle border, brand mark next to the product name, roomier content area — tokens only.
// Owner 2026-11-22 (ADR-0097): deep brand-blue sidebar (Menu theme="dark" + admin tokens ops.color.sider.*), fixed
// header and sidebar — only the content area scrolls (Tailwind utility classes mapped to the admin tokens) — and
// language / light-dark as icon buttons in the header (no longer in the account dropdown).
import { CloseOutlined, GlobalOutlined, MenuOutlined, MoonOutlined, SunOutlined, UserOutlined } from '@ant-design/icons';
import { Breadcrumb, Button, Drawer, Flex, Layout, Menu, Popover, Select, Switch, Tooltip, Typography, theme } from 'antd';
import type { CSSProperties } from 'react';
import { useEffect, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { usePreferences } from '../app/PreferencesProvider';
import type { WorkspaceDef } from '../generated/menu-catalog';
import { menuForPath, visibleMenus, visibleWorkspaces, workspaceForPath } from '../permissions/menuRegistry';
import { Loading } from '../permissions/RequirePermission';
import { usePermissions } from '../permissions/usePermissions';
import { LOCALES } from '../i18n';
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

/**
 * Shell layout classes (Tailwind, ADR-0097): the page itself never scrolls — header (and banner) stay on top, the
 * sidebar scrolls on its own and only the content area scrolls (a too-wide table scrolls inside it), so no horizontal
 * page scroll on a phone. Exported for the layout test.
 */
export const SHELL_CLASSES = {
  root: 'tw-h-dvh tw-overflow-hidden',
  header: 'tw-z-10 tw-shrink-0',
  body: 'tw-min-h-0',
  sider: 'tw-overflow-y-auto tw-overflow-x-hidden',
  content: 'tw-min-w-0 tw-overflow-auto',
} as const;

/** Sidebar tone (admin tokens ops.color.sider.*): the mobile drawer is the sidebar, so it wears the same colours. */
const SIDER_SURFACE: CSSProperties = { background: 'var(--ew-ops-color-sider-bg)', color: 'var(--ew-ops-color-sider-text-default)' };

/** Square brand mark (notebook) in the primary colour; decorative, the product name follows as text. */
export function BrandMark() {
  const { token } = theme.useToken();
  return (
    <span
      aria-hidden="true"
      data-testid="brand-mark"
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        flex: 'none',
        width: token.controlHeight,
        height: token.controlHeight,
        borderRadius: token.borderRadius,
        background: token.colorPrimary,
      }}
    >
      <svg width={token.sizeMD} height={token.sizeMD} viewBox="0 0 24 24" fill="none" stroke={token.colorTextLightSolid} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
        <path d="M4 19.5V5a2 2 0 0 1 2-2h13v16H6a2 2 0 0 0-2 2" />
        <path d="M8 7h7M8 11h5" />
      </svg>
    </span>
  );
}

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

/**
 * Header quick toggles (ADMIN_PORTAL §1.3, owner 2026-11-22): language (globe + current code, switches to the other
 * locale at once) and light/dark (sun/moon). Same PreferencesProvider + storage as before (AC-16/17); tooltip and
 * accessible name translated; the visible code is part of the accessible name (WCAG 2.5.3).
 */
export function LocaleToggle({ style }: { style?: CSSProperties }) {
  const { t } = useTranslation();
  const { preferences, update } = usePreferences();
  const current = preferences.locale;
  const next = LOCALES[(LOCALES.indexOf(current) + 1) % LOCALES.length]!;
  const code = t(`shell.locale.short.${current}`);
  const label = t('shell.header.language', { current: code, next: t(`shell.locale.${next}`) });
  return (
    <Tooltip title={label}>
      <Button type="text" icon={<GlobalOutlined />} aria-label={label} onClick={() => update({ locale: next })} style={style} data-testid="locale-toggle">
        {code}
      </Button>
    </Tooltip>
  );
}

export function ThemeToggle({ style }: { style?: CSSProperties }) {
  const { t } = useTranslation();
  const { preferences, update } = usePreferences();
  const dark = preferences.theme === 'dark';
  const label = dark ? t('shell.header.themeToLight') : t('shell.header.themeToDark');
  return (
    <Tooltip title={label}>
      <Button
        type="text"
        icon={dark ? <SunOutlined /> : <MoonOutlined />}
        aria-label={label}
        onClick={() => update({ theme: dark ? 'light' : 'dark' })}
        style={style}
        data-testid="theme-toggle"
      />
    </Tooltip>
  );
}

/** User menu (ADMIN_PORTAL §1.3): user name, compact density, sign out — applied without reload (AC-17). */
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
      <Flex gap="small" align="center" justify="space-between">
        <span id="ew-compact-label">{t('shell.userMenu.compact')}</span>
        <Switch aria-labelledby="ew-compact-label" checked={preferences.density === 'compact'} onChange={(c) => update({ density: c ? 'compact' : 'default', densityChosen: true })} />
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
        theme="dark"
        style={{ borderInlineEnd: 'none' }}
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
    <Layout className={SHELL_CLASSES.root} data-testid="portal-shell">
      <a href={`#${MAIN_ID}`} className="ew-skip-link">
        {t('shell.layout.skipToContent')}
      </a>
      <Layout.Header
        className={SHELL_CLASSES.header}
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
            <Flex align="center" gap="small">
              <BrandMark />
              <Typography.Text strong style={{ fontSize: token.fontSizeLG }}>
                {t('app.shortTitle')}
              </Typography.Text>
            </Flex>
            <WorkspaceSwitcher workspaces={workspaces} current={workspace} />
            {!mobile ? tenantControls : null}
          </Flex>
          <Flex align="center" gap="small">
            <LocaleToggle style={touch} />
            <ThemeToggle style={touch} />
            <UserMenu />
          </Flex>
        </Flex>
      </Layout.Header>
      <BannerSlot>{vpn ? <VpnRequiredNotice onClose={dismissVpn} /> : null}</BannerSlot>
      <Layout className={SHELL_CLASSES.body} hasSider={desktop && !!workspace}>
        {desktop && workspace ? (
          <Layout.Sider width={token.screenXS / 2} className={SHELL_CLASSES.sider} style={{ paddingBlock: token.paddingXS }} data-testid="portal-sider">
            <SideMenu workspace={workspace} />
          </Layout.Sider>
        ) : null}
        <Drawer
          open={!desktop && drawer}
          onClose={() => setDrawer(false)}
          placement="left"
          title={<span style={{ color: SIDER_SURFACE.color }}>{workspace ? t(workspace.labelKey) : t('app.shortTitle')}</span>}
          closeIcon={<CloseOutlined style={{ color: SIDER_SURFACE.color }} />}
          styles={{ content: SIDER_SURFACE, header: { ...SIDER_SURFACE, borderBottomColor: 'var(--ew-ops-color-sider-hover-bg)' } }}
        >
          <Flex vertical gap="middle">
            {mobile ? tenantControls : null}
            {workspace ? <SideMenu workspace={workspace} onNavigate={() => setDrawer(false)} /> : null}
          </Flex>
        </Drawer>
        <Layout.Content id={MAIN_ID} tabIndex={-1} className={SHELL_CLASSES.content} style={{ padding: desktop ? token.paddingXL : token.paddingMD }} data-testid="portal-content">
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
