// Portal runtime (F051 T019/T022): one object per page load wiring the session infrastructure — access token in
// memory (TokenStore), portal session API (authApi), single-flight refresh (RefreshScheduler = the apiClient
// TokenProvider), working tenant (X-Tenant-Id source), apiClient (F050), TanStack Query cache, IdleGuard and the
// cross-tab channel. React components read it through PortalRuntimeContext; no business logic here (AGENTS R1).
import { QueryClient } from '@tanstack/react-query';
import type { i18n as I18n } from 'i18next';
import { createContext, useContext } from 'react';
import { createAuthApi, type AuthApi } from '../auth/authApi';
import { readCsrfToken } from '../auth/csrf';
import { IdleGuard } from '../auth/IdleGuard';
import { RefreshScheduler } from '../auth/refreshScheduler';
import { TokenStore } from '../auth/tokenStore';
import { WorkingTenant } from '../session/tenantSelection';
import { createApiClient, type ApiClient } from '../shared/api/client';
import type { RuntimeConfig } from '../shared/api/types';

export const SESSION_CHANNEL = 'ew.portal.session';

export interface PortalRuntime {
  config: RuntimeConfig;
  i18n: I18n;
  tokens: TokenStore;
  authApi: AuthApi;
  scheduler: RefreshScheduler;
  tenant: WorkingTenant;
  api: ApiClient;
  queryClient: QueryClient;
  idle: IdleGuard;
  /** Current value of the CSRF cookie (X-EW-CSRF), null when missing. */
  readCsrf: () => string | null;
  /** Cross-tab channel (sign-out only; never a token). Null when BroadcastChannel is unavailable. */
  channel: BroadcastChannel | null;
  /** Plain fetch for presigned object-storage URLs (F082): never carries the token or the API middleware. */
  upload?: typeof fetch;
}

export interface RuntimeOverrides {
  fetch?: typeof fetch;
  locks?: ConstructorParameters<typeof RefreshScheduler>[0]['locks'];
  now?: () => number;
  channel?: BroadcastChannel | null;
  /** MOCK cookie jar of tests (jsdom refuses `__Host-` cookies on http). */
  readCsrf?: () => string | null;
}

export function createQueryClient(): QueryClient {
  // retries are owned by the apiClient (F050-AC-16)
  return new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false }, mutations: { retry: false } } });
}

export function createPortalRuntime(config: RuntimeConfig, i18n: I18n, o: RuntimeOverrides = {}): PortalRuntime {
  const now = o.now ?? Date.now;
  const hasTranslation = (key: string) => i18n.exists(key);
  const getLocale = () => i18n.language;
  const tokens = new TokenStore(now);
  const authApi = createAuthApi({ config, getLocale, hasTranslation, fetch: o.fetch, now });
  const idle = new IdleGuard({ idleMs: config.sessionIdleTimeoutSeconds * 1000, now });
  const readCsrf = o.readCsrf ?? (() => readCsrfToken());
  const scheduler = new RefreshScheduler({ authApi, store: tokens, locks: o.locks, now, readCsrf, isActive: () => idle.isActive() });
  const tenant = new WorkingTenant();
  const api = createApiClient({ config, tokenProvider: scheduler, tenant, getLocale, hasTranslation, fetch: o.fetch, now });
  const channel = o.channel !== undefined ? o.channel : typeof BroadcastChannel === 'undefined' ? null : new BroadcastChannel(SESSION_CHANNEL);
  const upload: typeof fetch = (input, init) => (o.fetch ?? globalThis.fetch)(input, init);
  return { config, i18n, tokens, authApi, scheduler, tenant, api, queryClient: createQueryClient(), idle, readCsrf, channel, upload };
}

export const PortalRuntimeContext = createContext<PortalRuntime | null>(null);

export function usePortalRuntime(): PortalRuntime {
  const r = useContext(PortalRuntimeContext);
  if (!r) throw new Error('PortalRuntime not provided');
  return r;
}
