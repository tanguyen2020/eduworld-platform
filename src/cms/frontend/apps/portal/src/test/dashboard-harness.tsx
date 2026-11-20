// MOCK (test only, AGENTS R17): renders the whole portal on a dashboard / results / reports URL (F056) against the shell
// MOCK backend + the F052 MOCK (schools, classes, students, login events) + the LMS MOCK (assignments, courses) + the
// dashboard MOCK (dashboard-mock.ts), signed in as a quickstart account. `roles` adds effective roles to
// getMyPermissions (e.g. an academic manager scoped to one subject).
import { configure } from '@testing-library/react';
import { vi } from 'vitest';
import { installDashboardMock, type DashboardMockState } from './dashboard-mock';
import { installLmsMock, type LmsMockState } from './lms-mock';
import { renderPortal, setViewportWidth, MockPortalBackend, TENANTS, type MockAccount } from './portal-harness';
import { accountWith, installSchoolLmsMock } from './school-lms-mock';
import type { WorkspaceLoaders } from '../permissions/workspaceRegistry';

configure({ asyncUtilTimeout: 30_000 });
vi.setConfig({ testTimeout: 240_000 });

/** School admin of TH Demo A (ADMIN_PORTAL §2.3): dashboard, reports export, LMS writes, login activity. */
export const SCHOOL_ADMIN_DASH_PERMS = [
  'analytics.school.read',
  'analytics.class.read',
  'lms.report.read',
  'report.export',
  'lms.assignment.read',
  'lms.assignment.write',
  'lms.course.read',
  'lms.class.read',
  'lms.student_activity.read',
  'tenant.school.read',
  'tenant.class.read',
  'learning_content.read',
  'moderation.school_notification.read',
];
export const adTruongA = (perms: string[] = SCHOOL_ADMIN_DASH_PERMS): MockAccount => accountWith('ad.truong.a', TENANTS.thDemoA, perms, '0193a4d2-1111-7000-8000-000000000001');

export interface RenderDashboardOptions {
  account?: MockAccount;
  as?: string;
  disabledMenus?: string[];
  disabledActions?: string[];
  roles?: { role: string; scope_type: 'tenant' | 'school' | 'subject'; scope_id?: string }[];
  width?: number;
  loaders?: WorkspaceLoaders;
  setup?: (dash: DashboardMockState, lms: LmsMockState) => void;
}

export async function renderDashboard(url: string, opts: RenderDashboardOptions = {}) {
  const backend = new MockPortalBackend().withSession(opts.as ?? 'ad.truong.a');
  if (!opts.as || opts.account) backend.user = opts.account ?? adTruongA();
  backend.disabledMenus = opts.disabledMenus ?? [];
  backend.disabledActions = opts.disabledActions ?? [];
  installSchoolLmsMock(backend);
  const lms = installLmsMock(backend);
  const dash = installDashboardMock(backend);
  if (opts.roles) {
    const roles = opts.roles.map((r) => ({ ...r, tenant_id: TENANTS.thDemoA.tenant_id }));
    backend.overrides.unshift((r) => {
      if (r.method !== 'GET' || r.path !== '/me/permissions' || !backend.user) return undefined;
      const fx = backend.user.perms[r.headers['x-tenant-id'] ?? 'platform'] ?? { workspaces: [], permissions: [] };
      const body = { roles, permissions: fx.permissions, workspaces: fx.workspaces, tenant_id: r.headers['x-tenant-id'] ?? null, disabled_menus: backend.disabledMenus, disabled_actions: backend.disabledActions };
      return new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } });
    });
  }
  opts.setup?.(dash, lms);
  setViewportWidth(opts.width ?? 1440);
  const r = await renderPortal({ url, backend, loaders: opts.loaders });
  return { ...r, dash, lms };
}

export { DASH_IDS, AS_OF, schoolDashboardFixture } from './dashboard-mock';
export { LMS_IDS, lmsAssignment } from './lms-mock';
export { IDS, accountWith } from './school-lms-mock';
export { problem, spyLoaders, stopPortals, TENANTS } from './portal-harness';
