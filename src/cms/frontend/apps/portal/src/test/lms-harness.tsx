// MOCK (test only, AGENTS R17): renders the whole portal on an LMS URL of workspace C (F055) against the shell MOCK
// backend + the F052 MOCK (schools, classes, students) + the LMS MOCK (lms-mock.ts), signed in as a quickstart account.
import { configure } from '@testing-library/react';
import { vi } from 'vitest';
import { renderPortal, setViewportWidth, MockPortalBackend, TENANTS, type MockAccount } from './portal-harness';
import { accountWith, installSchoolLmsMock, SCHOOL_PERMS } from './school-lms-mock';
import { installLmsMock, type LmsMockState } from './lms-mock';
import type { WorkspaceLoaders } from '../permissions/workspaceRegistry';

configure({ asyncUtilTimeout: 30_000 });
vi.setConfig({ testTimeout: 240_000 });

/** School admin of TH Demo A with the LMS permissions and the content read needed by the picker (ADMIN_PORTAL §2.3). */
export const LMS_PERMS = [...SCHOOL_PERMS, 'learning_content.read'];
export const adTruongA = (perms: string[] = LMS_PERMS): MockAccount => accountWith('ad.truong.a', TENANTS.thDemoA, perms, '0193a4d2-1111-7000-8000-000000000001');
/** Tenant admin: LMS read only (ADMIN_PORTAL §2.3: no assignment write). */
export const TENANT_ADMIN_PERMS = ['lms.student.read', 'lms.assignment.read', 'tenant.school.read', 'tenant.student.read', 'learning_content.read'];

export interface RenderLmsOptions {
  as?: string;
  account?: MockAccount | null;
  disabledMenus?: string[];
  disabledActions?: string[];
  width?: number;
  loaders?: WorkspaceLoaders;
  setup?: (s: LmsMockState) => void;
  /** Extra MOCK overrides placed before the LMS handlers (e.g. a 403 of another module). */
  overrides?: MockPortalBackend['overrides'];
}

export async function renderLms(url: string, opts: RenderLmsOptions = {}) {
  const backend = new MockPortalBackend().withSession(opts.as ?? 'ad.truong.a');
  if (opts.account !== null) backend.user = opts.account ?? adTruongA();
  backend.disabledMenus = opts.disabledMenus ?? [];
  backend.disabledActions = opts.disabledActions ?? [];
  installSchoolLmsMock(backend);
  const lms = installLmsMock(backend);
  opts.setup?.(lms);
  if (opts.overrides) backend.overrides.unshift(...opts.overrides);
  setViewportWidth(opts.width ?? 1440);
  const r = await renderPortal({ url, backend, loaders: opts.loaders });
  return { ...r, lms };
}

export { LMS_IDS, lmsAssignment } from './lms-mock';
export { IDS, accountWith } from './school-lms-mock';
export { problem, spyLoaders, stopPortals, TENANTS } from './portal-harness';
