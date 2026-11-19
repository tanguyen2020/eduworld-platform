// Portal root (F050, F051 T008/T019): runtime config → i18n → preferences (theme, density, locale — live) → portal
// runtime (token in memory, refresh, working tenant, apiClient) → TanStack Query → router (session inside).
// TanStack Query retries are off: the apiClient owns the retry policy (F050-AC-16).
import { QueryClientProvider } from '@tanstack/react-query';
import type { i18n as I18n } from 'i18next';
import { useMemo } from 'react';
import { I18nextProvider } from 'react-i18next';
import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import type { RuntimeConfig } from '../shared/api/types';
import { TenantContextProvider } from '../shared/auth/TenantContext';
import { PreferencesProvider } from './PreferencesProvider';
import type { UserPreference } from './preferences';
import { createRoutes } from './router';
import { createPortalRuntime, PortalRuntimeContext, type PortalRuntime } from './runtime';

export interface AppProps {
  config: RuntimeConfig;
  i18n: I18n;
  preferences: UserPreference;
  /** Tests inject a runtime with MOCK fetch; production builds it from the config. */
  runtime?: PortalRuntime;
}

export function App({ config, i18n, preferences, runtime }: AppProps) {
  const rt = useMemo(() => runtime ?? createPortalRuntime(config, i18n), [runtime, config, i18n]);
  const router = useMemo(() => createBrowserRouter(createRoutes()), []);
  return (
    <I18nextProvider i18n={i18n}>
      <PreferencesProvider initial={preferences} i18n={i18n}>
        <PortalRuntimeContext.Provider value={rt}>
          <TenantContextProvider source={rt.tenant}>
            <QueryClientProvider client={rt.queryClient}>
              <RouterProvider router={router} />
            </QueryClientProvider>
          </TenantContextProvider>
        </PortalRuntimeContext.Provider>
      </PreferencesProvider>
    </I18nextProvider>
  );
}
