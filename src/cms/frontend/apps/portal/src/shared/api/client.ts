// apiClient (F050 US3): typed `openapi-fetch` client over the merged spec + a fetch pipeline applying, per
// operation metadata: language → auth → tenant → idempotency → ifMatch → gateway → retry → problem.
// No token is stored outside memory or written to logs (F050-AC-17, AGENTS R14).
import createClient, { type Client } from 'openapi-fetch';
import type { TokenProvider } from '../auth/TokenProvider';
import type { TenantSource } from '../auth/TenantContext';
import { ApiEvents } from './events';
import { traceparent } from './ids';
import { applyAuth, createRefresher } from './middleware/auth';
import { apiPath, targetUrl } from './middleware/gateway';
import { applyIdempotency, IdempotencyKeys } from './middleware/idempotency';
import { applyIfMatch, EtagCache } from './middleware/ifmatch';
import { applyLanguage } from './middleware/language';
import { ApiError, i18nKeyFor, peekProblem, peekProblemCode, toApiError, type HasTranslation } from './middleware/problem';
import { backoffMs, isRetryable, MAX_RETRIES, MAX_RETRY_AFTER_MS, NOT_REPEATED_CODES, parseRetryAfter, RETRYABLE_STATUS, wait429 } from './middleware/retry';
import { applyTenant, scopeSearch } from './middleware/tenant';
import { findOperation, gatewayForPath } from './operations';
import type { paths } from './schema.gen';
import type { RuntimeConfig } from './types';

export interface ApiClientOptions {
  config: RuntimeConfig;
  tokenProvider: TokenProvider;
  tenant: TenantSource;
  /** Current UI language (`vi-VN` | `en-US`). */
  getLocale: () => string;
  /** i18n lookup used to map `code` → `error.<CODE>` (fallback `error.generic`). */
  hasTranslation: HasTranslation;
  fetch?: typeof fetch;
  sleep?: (ms: number) => Promise<void>;
  random?: () => number;
  now?: () => number;
  events?: ApiEvents;
  /**
   * F051-AC-13: on `403 MFA_REQUIRED` the shell opens the re-login dialog (password + code, `portalLogin`); resolves
   * true when a new access token is in place — the request is then retried once. Shared by concurrent requests.
   */
  reauthenticate?: () => Promise<boolean>;
}

export interface ApiClient {
  client: Client<paths>;
  events: ApiEvents;
  idempotencyKeys: IdempotencyKeys;
  etags: EtagCache;
  /** The pipeline itself (used by openapi-fetch; exposed for tests and non-JSON calls). */
  fetch: (request: Request) => Promise<Response>;
  /** Unwraps an openapi-fetch result: data, or throws ApiError mapped from the Problem. */
  call<T>(result: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<T>;
}

const defaultSleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

export function createApiClient(opts: ApiClientOptions): ApiClient {
  const { config, tokenProvider, tenant } = opts;
  const fetchImpl = opts.fetch ?? ((...a: Parameters<typeof fetch>) => globalThis.fetch(...a));
  const sleep = opts.sleep ?? defaultSleep;
  const random = opts.random ?? Math.random;
  const now = opts.now ?? Date.now;
  const events = opts.events ?? new ApiEvents();
  const idempotencyKeys = new IdempotencyKeys(now);
  const etags = new EtagCache();
  const refresh = createRefresher(tokenProvider);
  let reauthInFlight: Promise<boolean> | null = null;
  const reauthenticate = (): Promise<boolean> => {
    if (!opts.reauthenticate) return Promise.resolve(false);
    reauthInFlight ??= opts
      .reauthenticate()
      .catch(() => false)
      .finally(() => {
        reauthInFlight = null;
      });
    return reauthInFlight;
  };

  async function pipeline(input: Request): Promise<Response> {
    const url = new URL(input.url);
    const path = apiPath(url, config);
    const method = input.method.toUpperCase();
    const meta = findOperation(method, path);
    const gateway = meta?.gateway ?? gatewayForPath(path);
    const operationId = meta?.operationId;

    const headers = new Headers(input.headers);
    applyLanguage(headers, opts.getLocale());
    const sessionTenant = tenant.getTenantId();
    applyTenant(headers, meta?.tenantHeader ?? 'none', sessionTenant);
    const intent = applyIdempotency(headers, meta?.idempotency ?? false, idempotencyKeys, now);
    applyIfMatch(headers, meta?.ifMatch ?? false, path, etags);
    headers.set('traceparent', traceparent());
    const target = targetUrl(gateway, path, scopeSearch(url.search, sessionTenant), config);
    const body = method === 'GET' || method === 'HEAD' ? undefined : await input.arrayBuffer();
    const retryable = isRetryable(method, headers);

    let token = await tokenProvider.getAccessToken();
    let refreshed = false;
    let reauthenticated = false;
    let attempt = 0;
    for (;;) {
      applyAuth(headers, token);
      const request = new Request(target, { method, headers, body, signal: input.signal, credentials: input.credentials, redirect: input.redirect });
      let res: Response;
      try {
        res = await fetchImpl(request);
      } catch (err) {
        if (input.signal?.aborted) throw err;
        if (retryable && attempt < MAX_RETRIES) {
          await sleep(backoffMs(attempt++, random));
          continue;
        }
        if (gateway === 'internal') {
          events.emit('vpn_required', { operationId });
          throw new ApiError({ status: 0, code: 'VPN_REQUIRED', i18nKey: i18nKeyFor('VPN_REQUIRED', opts.hasTranslation), operationId, retryable: true });
        }
        throw new ApiError({ status: 0, code: 'NETWORK_ERROR', i18nKey: i18nKeyFor('NETWORK_ERROR', opts.hasTranslation), operationId, retryable: true });
      }

      if (RETRYABLE_STATUS.has(res.status) && retryable && attempt < MAX_RETRIES && !NOT_REPEATED_CODES.has((await peekProblemCode(res)) ?? '')) {
        await sleep(backoffMs(attempt++, random));
        continue;
      }
      if (res.status === 429) {
        const wait = wait429(await peekProblemCode(res), parseRetryAfter(res.headers.get('Retry-After'), now()), backoffMs(attempt, random));
        if (retryable && attempt < MAX_RETRIES && wait <= MAX_RETRY_AFTER_MS) {
          attempt++;
          await sleep(wait);
          continue;
        }
        return res;
      }
      if (res.status === 401) {
        const code = await peekProblemCode(res);
        if (code === 'TOKEN_EXPIRED' && !refreshed) {
          refreshed = true;
          if (await refresh()) {
            token = await tokenProvider.getAccessToken();
            continue;
          }
          events.emit('session_expired', { operationId, status: 401 });
          return res;
        }
        if (refreshed || code === 'TOKEN_EXPIRED') events.emit('session_expired', { operationId, status: 401 });
        return res;
      }
      if (res.status === 403) {
        const { code, traceId, featureKey } = await peekProblem(res);
        const detail = { operationId, status: 403, code, traceId };
        if (code === 'MFA_REQUIRED') {
          events.emit('mfa_required', detail);
          if (!reauthenticated && opts.reauthenticate) {
            reauthenticated = true;
            if (await reauthenticate()) {
              token = await tokenProvider.getAccessToken();
              continue;
            }
          }
        }
        if (code === 'TENANT_MISMATCH' || code === 'TENANT_ACCESS_DISABLED') events.emit('tenant_mismatch', detail);
        if (code === 'PORTAL_ACCESS_STAFF_ONLY') events.emit('staff_only', detail);
        if (code === 'FEATURE_DISABLED') events.emit('feature_disabled', { ...detail, featureKey });
        if (code === 'PERMISSION_DENIED') events.emit('permission_denied', detail);
        return res;
      }
      if (res.ok) {
        if (method === 'GET' || meta?.ifMatch) etags.remember(path, res.headers.get('ETag'));
        if (intent) idempotencyKeys.complete(intent);
      }
      return res;
    }
  }

  const client = createClient<paths>({ baseUrl: config.apiBaseUrl, fetch: pipeline });

  async function call<T>(result: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<T> {
    const r = await result;
    if (!r.response.ok || r.error !== undefined) {
      throw toApiError(r.response.status, r.error, opts.hasTranslation, {
        retryAfterMs: parseRetryAfter(r.response.headers.get('Retry-After'), now()),
      });
    }
    return r.data as T;
  }

  return { client, events, idempotencyKeys, etags, fetch: pipeline, call };
}
