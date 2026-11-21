// Routes of the portal (F050 T010, F051 T032): public pages drawn by the portal (/login, /forgot-password,
// /reset-password, /account-setup — ADR-0091), tenant picker, then the protected shell: `/` → last/first visible
// workspace, one route per workspace of the generated catalog (`/school-lms`, `/content-studio`, `/game-ops`,
// `/live-ops`, `/platform-admin`) gated by getMyPermissions so that a workspace chunk loads only when visible.
import { Outlet, type RouteObject } from 'react-router-dom';
import { AccountSetupPage, ForgotPasswordPage, ResetPasswordPage } from '../auth/AccountRecoveryPages';
import { LoginPage } from '../auth/LoginPage';
import { WORKSPACE_CATALOG } from '../generated/menu-catalog';
import { WorkspaceGate } from '../permissions/RequirePermission';
import { workspaceLoaders, type WorkspaceLoaders } from '../permissions/workspaceRegistry';
import { RequireSession, TenantSelectPage } from '../session/SessionPages';
import { SessionProvider } from '../session/SessionProvider';
import { HomeRedirect, PortalLayout } from '../shell/PortalLayout';
import { NotFound } from './NotFound';

function SessionRoot() {
  return (
    <SessionProvider>
      <Outlet />
    </SessionProvider>
  );
}

export function createRoutes(loaders: WorkspaceLoaders = workspaceLoaders): RouteObject[] {
  return [
    {
      element: <SessionRoot />,
      children: [
        { path: '/login', element: <LoginPage /> },
        { path: '/forgot-password', element: <ForgotPasswordPage /> },
        { path: '/reset-password', element: <ResetPasswordPage /> },
        { path: '/account-setup', element: <AccountSetupPage /> },
        { path: '/select-tenant', element: <TenantSelectPage /> },
        {
          element: <RequireSession />,
          children: [
            {
              path: '/',
              element: <PortalLayout />,
              children: [
                { index: true, element: <HomeRedirect /> },
                ...WORKSPACE_CATALOG.map(
                  (w): RouteObject => ({
                    path: `${w.path.slice(1)}/*`,
                    element: <WorkspaceGate key={w.key} workspace={w.key} loader={loaders[w.key]} />,
                  }),
                ),
                { path: '*', element: <NotFound /> },
              ],
            },
          ],
        },
      ],
    },
  ];
}
