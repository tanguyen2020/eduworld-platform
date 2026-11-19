// Portal root (F050): runtime config → i18n → OpsThemeProvider → TanStack Query → router.
// TanStack Query retries are off: the apiClient owns the retry policy (F050-AC-16).
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { OpsThemeProvider } from '@eduworld/ops-ui';
import type { i18n as I18n } from 'i18next';
import { createContext, useContext, useMemo } from 'react';
import { I18nextProvider } from 'react-i18next';
import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import type { Locale } from '../i18n';
import type { RuntimeConfig } from '../shared/api/types';
import { createRoutes } from './router';
import type { UserPreference } from './preferences';

/** Runtime config for the shell and the apiClient (F051 creates the client from it). */
const RuntimeConfigContext = createContext<RuntimeConfig | null>(null);

export function useRuntimeConfig(): RuntimeConfig {
  const c = useContext(RuntimeConfigContext);
  if (!c) throw new Error('RuntimeConfig not provided');
  return c;
}

export function createQueryClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false }, mutations: { retry: false } } });
}

export interface AppProps {
  config: RuntimeConfig;
  i18n: I18n;
  preferences: UserPreference;
}

export function App({ config, i18n, preferences }: AppProps) {
  const router = useMemo(() => createBrowserRouter(createRoutes()), []);
  const queryClient = useMemo(() => createQueryClient(), []);
  return (
    <I18nextProvider i18n={i18n}>
      <OpsThemeProvider mode={preferences.theme} density={preferences.density} locale={preferences.locale as Locale}>
        <RuntimeConfigContext.Provider value={config}>
          <QueryClientProvider client={queryClient}>
            <RouterProvider router={router} />
          </QueryClientProvider>
        </RuntimeConfigContext.Provider>
      </OpsThemeProvider>
    </I18nextProvider>
  );
}
