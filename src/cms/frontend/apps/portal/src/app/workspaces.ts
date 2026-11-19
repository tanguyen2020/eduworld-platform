// The 5 workspaces of the single Admin Portal (ADMIN_PORTAL §1.2, ADR-0027 mục 7, Clarification 2026-11-17).
// Each one is a separate lazy chunk; nothing here decides access (the backend does, F051 adds the menu).
import type { ComponentType } from 'react';

export const WORKSPACES = [
  { id: 'school-lms', path: 'school' },
  { id: 'content-studio', path: 'content' },
  { id: 'game-ops', path: 'game-ops' },
  { id: 'live-ops', path: 'live-ops' },
  { id: 'platform-admin', path: 'platform' },
] as const;

export type WorkspaceId = (typeof WORKSPACES)[number]['id'];
export type WorkspaceLoader = () => Promise<{ default: ComponentType }>;

export const workspaceLoaders: Record<WorkspaceId, WorkspaceLoader> = {
  'school-lms': () => import('../workspaces/school-lms'),
  'content-studio': () => import('../workspaces/content-studio'),
  'game-ops': () => import('../workspaces/game-ops'),
  'live-ops': () => import('../workspaces/live-ops'),
  'platform-admin': () => import('../workspaces/platform-admin'),
};
