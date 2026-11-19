// workspaceRegistry (F051 T010, F051-AC-10, ADR-0027 mục 3, 7): workspace key of `PermissionSet.workspaces` →
// folder `src/workspaces/<id>` → lazy chunk. Route paths come from the catalog (`/school-lms`, …), not from here.
// A chunk is imported only when the shell renders that workspace, i.e. only when `workspaces[]` contains it —
// an optimisation, never a security layer (the backend decides).
import type { ComponentType } from 'react';
import type { WorkspaceKey } from '../generated/menu-catalog';

export const WORKSPACE_FOLDERS = {
  school_lms: 'school-lms',
  content_studio: 'content-studio',
  game_ops: 'game-ops',
  live_ops: 'live-ops',
  platform_admin: 'platform-admin',
} as const satisfies Record<WorkspaceKey, string>;

export type WorkspaceId = (typeof WORKSPACE_FOLDERS)[WorkspaceKey];
export type WorkspaceLoader = () => Promise<{ default: ComponentType }>;
export type WorkspaceLoaders = Record<WorkspaceKey, WorkspaceLoader>;

export const workspaceLoaders: WorkspaceLoaders = {
  school_lms: () => import('../workspaces/school-lms'),
  content_studio: () => import('../workspaces/content-studio'),
  game_ops: () => import('../workspaces/game-ops'),
  live_ops: () => import('../workspaces/live-ops'),
  platform_admin: () => import('../workspaces/platform-admin'),
};

/** Workspaces served by the internal gateway (VPN): their network errors show the VPN notice (F051-AC-14). */
export const INTERNAL_WORKSPACES: readonly WorkspaceKey[] = ['game_ops', 'live_ops', 'platform_admin'];
