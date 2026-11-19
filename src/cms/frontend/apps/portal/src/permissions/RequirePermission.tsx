// Route gates (F051 T032, T061, F051-AC-10/11/35): a workspace route renders its lazy chunk only when the workspace is
// visible for the session (so a tenant user never downloads game_ops/live_ops/platform_admin code), and a menu route
// renders its screen only when the menu is visible — otherwise the 403 page or the "Tính năng đang tắt" page, and the
// screen (and its API calls) never mounts. Display gates only: the backend authorizes every request (AGENTS R20).
import { Spin } from 'antd';
import { lazy, Suspense, useMemo, type ComponentType, type LazyExoticComponent } from 'react';
import { useTranslation } from 'react-i18next';
import { Navigate, Route, Routes } from 'react-router-dom';
import type { MenuKey, WorkspaceKey } from '../generated/menu-catalog';
import { MenuPlaceholder, FeatureDisabledPage, ForbiddenPage } from '../shell/StatusPages';
import { NotFound } from '../app/NotFound';
import { ProblemMessage } from '../shell/ProblemMessage';
import { menuStatus, visibleMenus, visibleWorkspaces, workspaceByKey } from './menuRegistry';
import { usePermissions } from './usePermissions';
import type { WorkspaceLoader } from './workspaceRegistry';

export function Loading() {
  const { t } = useTranslation();
  return <Spin aria-label={t('app.loading')} />;
}

const lazyCache = new Map<WorkspaceLoader, LazyExoticComponent<ComponentType>>();
const lazyOf = (loader: WorkspaceLoader) => {
  let c = lazyCache.get(loader);
  if (!c) {
    c = lazy(loader);
    lazyCache.set(loader, c);
  }
  return c;
};

export function WorkspaceGate({ workspace, loader }: { workspace: WorkspaceKey; loader: WorkspaceLoader }) {
  const { view, isLoading, error } = usePermissions();
  const Workspace = useMemo(() => lazyOf(loader), [loader]);
  if (isLoading) return <Loading />;
  if (error || !view) return <ProblemMessage error={error} />;
  if (!visibleWorkspaces(view).some((w) => w.key === workspace)) return <ForbiddenPage />;
  return (
    <Suspense fallback={<Loading />}>
      <Workspace />
    </Suspense>
  );
}

export function MenuGate({ menuKey, children }: { menuKey: MenuKey; children: JSX.Element }) {
  const { view } = usePermissions();
  if (!view) return <Loading />;
  const status = menuStatus(menuKey, view);
  if (status === 'disabled') return <FeatureDisabledPage featureKey={menuKey} />;
  if (status === 'forbidden') return <ForbiddenPage />;
  return children;
}

/** Menu keys of one workspace (`menu.<workspace>.<menu>`). */
export type WorkspaceMenuKey<W extends WorkspaceKey> = Extract<MenuKey, `menu.${W}.${string}`>;

/** Screens of a workspace: every menu key of the catalog maps to a component (missing key → compile error). */
export type MenuScreens<W extends WorkspaceKey> = Record<WorkspaceMenuKey<W>, ComponentType<{ title: string }>>;

/** Default screen until a later feature attaches the real one to the menu key. */
export const placeholderScreen: ComponentType<{ title: string }> = MenuPlaceholder;

/**
 * Routes of a workspace chunk, generated from the catalog: index → first visible menu; one gated route per menu
 * (`path` from the catalog). Workspace features only attach components to menu keys (ADR-0090).
 */
export function WorkspaceRoutes<W extends WorkspaceKey>({ workspace, screens }: { workspace: W; screens: MenuScreens<W> }) {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const ws = workspaceByKey(workspace);
  const first = view ? visibleMenus(ws, view)[0] : undefined;
  const relative = (path: string) => path.slice(ws.path.length + 1);
  return (
    <Routes>
      <Route index element={first ? <Navigate to={relative(first.path)} replace /> : <ForbiddenPage />} />
      {ws.menus.map((m) => {
        const Screen = (screens as Record<string, ComponentType<{ title: string }>>)[m.key] ?? placeholderScreen;
        return (
          <Route
            key={m.key}
            path={`${relative(m.path)}/*`}
            element={
              <MenuGate menuKey={m.key}>
                <Screen title={t(m.labelKey)} />
              </MenuGate>
            }
          />
        );
      })}
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
}
