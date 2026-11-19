// F050-AC-01, F050-AC-02: the portal renders `/` without loading any workspace chunk; opening a workspace loads
// only its own chunk; 5 workspaces under src/workspaces/<id>/. The production build is checked by
// scripts/check-chunks.mjs (manifest) — see check-chunks.test.mjs.
import { OpsThemeProvider } from '@eduworld/ops-ui';
import { render, screen } from '@testing-library/react';
import fs from 'node:fs';
import path from 'node:path';
import { I18nextProvider } from 'react-i18next';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { createI18n } from '../i18n';
import { createRoutes } from './router';
import { WORKSPACES, workspaceLoaders, type WorkspaceId } from './workspaces';

function spyLoaders() {
  const spies = {} as Record<WorkspaceId, ReturnType<typeof vi.fn>>;
  for (const w of WORKSPACES) spies[w.id] = vi.fn(workspaceLoaders[w.id]);
  return spies;
}

async function renderAt(url: string, loaders = spyLoaders()) {
  const i18n = await createI18n('vi-VN');
  const router = createMemoryRouter(createRoutes(loaders as never), { initialEntries: [url] });
  render(
    <I18nextProvider i18n={i18n}>
      <OpsThemeProvider>
        <RouterProvider router={router} />
      </OpsThemeProvider>
    </I18nextProvider>,
  );
  return { loaders, router };
}

describe('portal routes', () => {
  it('has the 5 workspace folders of ADMIN_PORTAL §1.2', () => {
    const dir = path.resolve(__dirname, '../workspaces');
    expect(fs.readdirSync(dir).sort()).toEqual(['content-studio', 'game-ops', 'live-ops', 'platform-admin', 'school-lms']);
    expect(WORKSPACES.map((w) => w.id).sort()).toEqual(['content-studio', 'game-ops', 'live-ops', 'platform-admin', 'school-lms']);
  });

  it('`/` renders the home without loading any workspace', async () => {
    const { loaders } = await renderAt('/');
    expect(await screen.findByRole('heading', { name: 'Chọn khu vực làm việc' })).toBeInTheDocument();
    for (const w of WORKSPACES) expect(loaders[w.id]).not.toHaveBeenCalled();
    expect(screen.getAllByRole('link').length).toBeGreaterThanOrEqual(5);
  });

  it('opening /school loads only the school-lms workspace', async () => {
    const { loaders } = await renderAt('/school');
    expect(await screen.findByRole('heading', { name: 'Quản lý trường và học tập' })).toBeInTheDocument();
    expect(loaders['school-lms']).toHaveBeenCalledTimes(1);
    for (const w of WORKSPACES.filter((x) => x.id !== 'school-lms')) expect(loaders[w.id]).not.toHaveBeenCalled();
  });

  it('every workspace route renders its translated title', async () => {
    for (const w of WORKSPACES) {
      const { router } = await renderAt(`/${w.path}`);
      expect(await screen.findByRole('heading', { level: 2 })).toBeInTheDocument();
      router.dispose();
      document.body.innerHTML = '';
    }
  });

  it('unknown path → not found', async () => {
    await renderAt('/nope');
    expect(await screen.findByText('Không tìm thấy trang.')).toBeInTheDocument();
  });
});
