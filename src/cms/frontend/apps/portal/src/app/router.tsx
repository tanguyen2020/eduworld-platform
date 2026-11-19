// Routes of the portal (F050 T010): `/` home + one lazy route per workspace (react-router `lazy`, so a
// workspace chunk loads only when its route matches — F050-AC-02).
import type { RouteObject } from 'react-router-dom';
import { Home } from './Home';
import { NotFound } from './NotFound';
import { Shell } from './Shell';
import { WORKSPACES, workspaceLoaders, type WorkspaceId, type WorkspaceLoader } from './workspaces';

export function createRoutes(loaders: Record<WorkspaceId, WorkspaceLoader> = workspaceLoaders): RouteObject[] {
  return [
    {
      path: '/',
      element: <Shell />,
      children: [
        { index: true, element: <Home /> },
        ...WORKSPACES.map(
          (w): RouteObject => ({
            path: `${w.path}/*`,
            lazy: async () => ({ Component: (await loaders[w.id]()).default }),
          }),
        ),
        { path: '*', element: <NotFound /> },
      ],
    },
  ];
}
