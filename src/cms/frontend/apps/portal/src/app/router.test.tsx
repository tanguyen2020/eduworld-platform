// F050-AC-01/02, F051-AC-03/09/10/11 (TC-CMS-WS-01, TC-CMS-AUTHZ-05/06): routes come from the generated catalog; a
// workspace chunk is imported only when the session may see that workspace; a forbidden workspace URL shows the
// portal 403 page without loading its chunk; `/` goes to the first visible workspace. Backend = MOCK harness.
// The production build is checked by scripts/check-chunks.mjs (manifest) — see check-chunks.test.mjs.
import { screen, waitFor } from '@testing-library/react';
import fs from 'node:fs';
import path from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';
import { WORKSPACE_CATALOG } from '../generated/menu-catalog';
import { WORKSPACE_FOLDERS } from '../permissions/workspaceRegistry';
import { MockPortalBackend, renderPortal, stopPortals, TENANTS } from '../test/portal-harness';

afterEach(() => stopPortals());

describe('portal routes', () => {
  it('has the 5 workspace folders of ADMIN_PORTAL §1.2, one per catalog workspace', () => {
    const dir = path.resolve(__dirname, '../workspaces');
    expect(fs.readdirSync(dir).sort()).toEqual(['content-studio', 'game-ops', 'live-ops', 'platform-admin', 'school-lms']);
    for (const w of WORKSPACE_CATALOG) expect(w.path).toBe(`/${WORKSPACE_FOLDERS[w.key]}`);
  });

  it('school admin: `/` → first visible workspace; only the school-lms chunk is imported (F051-AC-10)', async () => {
    const { loaders, router } = await renderPortal({ url: '/', backend: new MockPortalBackend().withSession('ad.truong.a') });
    expect(await screen.findByRole('heading', { level: 1, name: 'Tổng quan' })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/school-lms/overview');
    expect(loaders.school_lms).toHaveBeenCalledTimes(1);
    for (const k of ['content_studio', 'game_ops', 'live_ops', 'platform_admin'] as const) expect(loaders[k]).not.toHaveBeenCalled();
  });

  it('academic manager opening /game-ops: 403 page, no platform chunk imported, no screen API (F051-AC-11, SC-002)', async () => {
    const backend = new MockPortalBackend().withSession('qlcm.truong.a');
    window.sessionStorage.setItem('ew.portal.sessionTenantId', TENANTS.thDemoA.tenant_id);
    const { loaders } = await renderPortal({ url: '/game-ops/quests', backend });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    for (const k of ['game_ops', 'live_ops', 'platform_admin'] as const) expect(loaders[k]).not.toHaveBeenCalled();
    expect(backend.requests.every((r) => r.path.startsWith('/auth/') || r.path.startsWith('/me/'))).toBe(true);
  });

  it('unknown path inside the shell → not found', async () => {
    await renderPortal({ url: '/nope', backend: new MockPortalBackend().withSession('ad.truong.a') });
    expect(await screen.findByText('Không tìm thấy trang.')).toBeInTheDocument();
  });

  it('known menu path → its screen; unknown path inside a workspace → not found', async () => {
    const { router } = await renderPortal({ url: '/school-lms/classes', backend: new MockPortalBackend().withSession('ad.truong.a') });
    expect(await screen.findByRole('heading', { level: 1, name: 'Học sinh theo khối' })).toBeInTheDocument();
    await router.navigate('/school-lms/does-not-exist');
    await waitFor(() => expect(screen.getByText('Không tìm thấy trang.')).toBeInTheDocument());
  });
});
