// MOCK (test only, AGENTS R17): renders the whole portal on a workspace C URL against the shell MOCK backend plus the
// F052 MOCK handlers (school-lms-mock.ts), signed in as a quickstart account.
import { configure } from '@testing-library/react';
import { vi } from 'vitest';
import { setViewportWidth, MockPortalBackend, renderPortal, type MockAccount } from './portal-harness';
import { installSchoolLmsMock } from './school-lms-mock';

// whole-portal screens with Ant Design tables/forms on a shared, throttled CPU: allow 30 s for async queries
configure({ asyncUtilTimeout: 30_000 });
// multi-step dialogs of F052 on the throttled CPU (2 workers shared with other agents): 4 min per test
vi.setConfig({ testTimeout: 240_000 });

export interface RenderSchoolLmsOptions {
  /** Shell MOCK account to restore (default `ad.truong.a`). */
  as?: string;
  /** Replaces the account after restore (accounts missing from the shell MOCK). */
  account?: MockAccount;
  disabledMenus?: string[];
  disabledActions?: string[];
  width?: number;
  /** Installs more MOCK handlers (e.g. school events of F081) before the portal renders. */
  beforeRender?: (backend: MockPortalBackend) => void;
}

export async function renderSchoolLms(url: string, opts: RenderSchoolLmsOptions = {}) {
  const backend = new MockPortalBackend().withSession(opts.as ?? 'ad.truong.a');
  if (opts.account) backend.user = opts.account;
  backend.disabledMenus = opts.disabledMenus ?? [];
  backend.disabledActions = opts.disabledActions ?? [];
  const state = installSchoolLmsMock(backend);
  opts.beforeRender?.(backend);
  if (opts.width) setViewportWidth(opts.width);
  const r = await renderPortal({ url, backend });
  return { ...r, state };
}

export { IDS, accountWith, READ_ONLY_PERMS, SCHOOL_PERMS } from './school-lms-mock';
export { stopPortals, TENANTS, problem } from './portal-harness';
