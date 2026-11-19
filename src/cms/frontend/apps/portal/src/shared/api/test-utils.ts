// MOCK / test doubles for apiClient tests (AGENTS R17): fake TokenProvider, fixed tenant, fake gateway hosts
// served by MSW. Never imported by application code.
import vi from '../../i18n/vi.json';
import type { TokenProvider } from '../auth/TokenProvider';
import { createApiClient, type ApiClientOptions } from './client';
import type { RuntimeConfig } from './types';

export const TEST_CONFIG: RuntimeConfig = {
  apiBaseUrl: 'https://public.gateway.test/api/v1',
  apiInternalBaseUrl: 'https://internal.gateway.test/api/v1',
  oidcIssuer: 'https://idp.test/realms/eduworld',
  oidcClientId: 'eduworld-portal',
  env: 'test',
};

export const PUBLIC = TEST_CONFIG.apiBaseUrl;
export const INTERNAL = TEST_CONFIG.apiInternalBaseUrl;
export const TENANT_ID = '0193a4d2-6c1e-7cc0-9b5e-3f2a1d4c5b6a';
export const SECRET_TOKEN = 'test-access-token-NOT-A-REAL-SECRET';

/** MOCK TokenProvider: returns `tokens` in order; refresh() answers `refreshResult`. */
export class FakeTokenProvider implements TokenProvider {
  refreshCalls = 0;
  private current: string | null;

  constructor(
    initial: string | null = SECRET_TOKEN,
    private readonly refreshResult = true,
    private readonly refreshed = `${SECRET_TOKEN}-refreshed`,
  ) {
    this.current = initial;
  }

  async getAccessToken() {
    return this.current;
  }

  async refresh() {
    this.refreshCalls++;
    if (this.refreshResult) this.current = this.refreshed;
    return this.refreshResult;
  }
}

const hasKey = (key: string) => {
  let node: unknown = vi;
  for (const part of key.split('.')) {
    if (!node || typeof node !== 'object' || !(part in node)) return false;
    node = (node as Record<string, unknown>)[part];
  }
  return typeof node === 'string';
};

export function testClient(overrides: Partial<ApiClientOptions> & { tenantId?: string | null } = {}) {
  const sleeps: number[] = [];
  const tokenProvider = overrides.tokenProvider ?? new FakeTokenProvider();
  const tenantId = overrides.tenantId === undefined ? TENANT_ID : overrides.tenantId;
  const api = createApiClient({
    config: TEST_CONFIG,
    tokenProvider,
    tenant: { getTenantId: () => tenantId },
    getLocale: () => 'vi-VN',
    hasTranslation: hasKey,
    sleep: async (ms) => {
      sleeps.push(ms);
    },
    random: () => 0.5,
    ...overrides,
  });
  return { api, sleeps, tokenProvider: tokenProvider as FakeTokenProvider };
}

export const problem = (status: number, code: string, extra: Record<string, unknown> = {}) =>
  new Response(JSON.stringify({ type: 'about:blank', title: 'ignored title', status, code, trace_id: 'trace-123', ...extra }), {
    status,
    headers: { 'Content-Type': 'application/problem+json' },
  });
