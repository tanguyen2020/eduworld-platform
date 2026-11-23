// MOCK (test only, AGENTS R17): renders the whole portal against the MOCK backend of mock-backend.ts.
import { QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { I18nextProvider } from 'react-i18next';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { vi } from 'vitest';
import { PreferencesProvider } from '../app/PreferencesProvider';
import { DEFAULT_PREFS } from '../app/preferences';
import { createRoutes } from '../app/router';
import { createPortalRuntime, PortalRuntimeContext, type PortalRuntime } from '../app/runtime';
import type { WorkspaceKey } from '../generated/menu-catalog';
import { createI18n, type Locale } from '../i18n';
import { workspaceLoaders, type WorkspaceLoaders } from '../permissions/workspaceRegistry';
import { TEST_CONFIG } from '../shared/api/test-utils';
import { TenantContextProvider } from '../shared/auth/TenantContext';
import { MockPortalBackend } from './mock-backend';

export { ACCOUNTS, fakeJwt, MockPortalBackend, permissionsOf, problem, TENANTS } from './mock-backend';
export type { MockAccount, RecordedRequest } from './mock-backend';

let viewportWidth = 1440;

/** MOCK (test only): jsdom has no layout; min-width media queries are answered from this width (default desktop). */
export function setViewportWidth(width: number): void {
  viewportWidth = width;
}

window.matchMedia = (query: string) => {
  const m = /min-width:\s*(\d+)px/.exec(query);
  return {
    matches: m ? viewportWidth >= Number(m[1]) : false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  } as MediaQueryList;
};

export interface RenderPortalOptions {
  url?: string;
  backend?: MockPortalBackend;
  locale?: Locale;
  loaders?: WorkspaceLoaders;
}

/** Spied workspace loaders (to assert which chunks are imported). */
export function spyLoaders(): WorkspaceLoaders & Record<WorkspaceKey, ReturnType<typeof vi.fn>> {
  const out = {} as Record<WorkspaceKey, ReturnType<typeof vi.fn>>;
  for (const k of Object.keys(workspaceLoaders) as WorkspaceKey[]) out[k] = vi.fn(workspaceLoaders[k]);
  return out as never;
}

const mounted: PortalRuntime[] = [];

/** Renders the whole portal (router + session) against the MOCK backend. */
export async function renderPortal(opts: RenderPortalOptions = {}) {
  const backend = opts.backend ?? new MockPortalBackend();
  const i18n = await createI18n(opts.locale ?? 'vi-VN');
  const runtime = createPortalRuntime(TEST_CONFIG, i18n, { fetch: backend.fetch, locks: null, channel: null, readCsrf: () => backend.csrf });
  mounted.push(runtime);
  const loaders = opts.loaders ?? spyLoaders();
  const router = createMemoryRouter(createRoutes(loaders), { initialEntries: [opts.url ?? '/'] });
  const user = userEvent.setup({ delay: null });
  render(
    <I18nextProvider i18n={i18n}>
      <PreferencesProvider initial={{ ...DEFAULT_PREFS, locale: opts.locale ?? 'vi-VN' }} i18n={i18n}>
        <PortalRuntimeContext.Provider value={runtime}>
          <TenantContextProvider source={runtime.tenant}>
            <QueryClientProvider client={runtime.queryClient}>
              <RouterProvider router={router} />
            </QueryClientProvider>
          </TenantContextProvider>
        </PortalRuntimeContext.Provider>
      </PreferencesProvider>
    </I18nextProvider>,
  );
  return { backend, runtime, router, loaders, user, i18n };
}

/** Stops timers of every portal rendered by the current test (call in afterEach). */
export function stopPortals(): void {
  for (const rt of mounted.splice(0)) {
    rt.scheduler.stop();
    rt.idle.stop();
    rt.queryClient.clear();
  }
  window.localStorage.clear();
  window.sessionStorage.clear();
  viewportWidth = 1440;
}

/** Fills and submits the /login form of the portal (always English — owner 2026-11-23). */
export async function signIn(user: ReturnType<typeof userEvent.setup>, username: string, password = 'Demo@12345'): Promise<void> {
  fireEvent.change(await screen.findByLabelText('Username or email'), { target: { value: username } });
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: password } });
  await user.click(screen.getByRole('button', { name: 'Sign in' }));
}
