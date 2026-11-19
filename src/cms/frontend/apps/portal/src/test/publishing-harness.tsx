// MOCK (test only, AGENTS R17): renders the whole portal on a workspace B / C URL against the shell MOCK backend plus
// the F054 MOCK handlers (mock-publishing.ts), signed in as a quickstart account or a custom one.
import { configure } from '@testing-library/react';
import { vi } from 'vitest';
import { MockPortalBackend, permissionsOf, renderPortal, setViewportWidth, TENANTS, type MockAccount } from './portal-harness';
import { contentAccount, MockPublishing } from './mock-publishing';

// whole-portal screens with Ant Design tables / dialogs on a shared, throttled CPU (2 workers): generous timeouts
configure({ asyncUtilTimeout: 30_000 });
vi.setConfig({ testTimeout: 240_000 });

/** Every permission of workspaces B and C for a tenant user (no platform permission). */
export const TENANT_CONTENT_PERMS = [...new Set([...permissionsOf(['content_studio']), ...permissionsOf(['school_lms'])])];

export interface RenderPublishingOptions {
  /** Shell MOCK account (default `cm.noidung`, platform content admin). */
  as?: string;
  account?: MockAccount;
  disabledActions?: string[];
  disabledMenus?: string[];
  width?: number;
  setup?: (m: MockPublishing) => void;
}

export async function renderPublishing(url: string, opts: RenderPublishingOptions = {}) {
  const backend = new MockPortalBackend().withSession(opts.as ?? 'cm.noidung');
  if (opts.account) backend.user = opts.account;
  backend.disabledActions = opts.disabledActions ?? [];
  backend.disabledMenus = opts.disabledMenus ?? [];
  const mock = new MockPublishing(backend);
  opts.setup?.(mock);
  if (opts.width) setViewportWidth(opts.width);
  const r = await renderPortal({ url, backend });
  return { ...r, mock };
}

/** Tenant user of TH Demo A with the given permissions (default: all of B and C). */
export const tenantUser = (permissions: string[] = TENANT_CONTENT_PERMS, sub?: string) => contentAccount('gv.noidung.a', TENANTS.thDemoA, permissions, sub);

export { P, contentAccount, MockPublishing } from './mock-publishing';
export { problem, stopPortals, TENANTS } from './portal-harness';
