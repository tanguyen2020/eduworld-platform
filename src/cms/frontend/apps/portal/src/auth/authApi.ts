// authApi (F051 T017, ADR-0091): the six portal session operations `/api/v1/auth/portal/*` — portalLogin,
// refreshPortalToken, portalLogout, setupPortalPassword, forgotPortalPassword, resetPortalPassword. Public gateway,
// same origin, `credentials: "include"` so the browser sends/stores the HttpOnly refresh cookie; refresh and logout
// carry `X-EW-CSRF`. No Authorization, no X-Tenant-Id, no Idempotency-Key (contract). Bodies (password, OTP, setup
// ticket, reset token) are never logged or stored; errors map to ApiError by Problem `code` (F050-AC-14).
import type { RuntimeConfig } from '../shared/api/types';
import { traceparent } from '../shared/api/ids';
import { ApiError, i18nKeyFor, toApiError, type HasTranslation } from '../shared/api/middleware/problem';
import { parseRetryAfter } from '../shared/api/middleware/retry';
import type { components } from '../shared/api/schema.gen';
import { CSRF_HEADER } from './csrf';

type Schemas = components['schemas'];
export type PortalLoginRequest = Schemas['PortalLoginRequest'];
export type PortalTokenResponse = Schemas['PortalTokenResponse'];
export type PortalSetupStep = Schemas['PortalSetupStep'];
export type PortalSetupResult = Schemas['PortalSetupResult'];
export type PortalForgotPasswordRequest = Schemas['PortalForgotPasswordRequest'];
export type PortalResetPasswordRequest = Schemas['PortalResetPasswordRequest'];
export type PortalSetupPasswordRequest = Schemas['PortalSetupPasswordRequest'];

/** Extra fields of `403 ACCOUNT_SETUP_REQUIRED` (PortalLoginProblem). */
export interface SetupRequired {
  setupTicket: string;
  requiredSteps: PortalSetupStep[];
  otpSetupEmailSent: boolean;
}

export type AuthResult<T> = { ok: true; status: number; data: T } | { ok: false; status: number; error: ApiError; setup?: SetupRequired };

export const AUTH_PATHS = {
  login: '/auth/portal/login',
  refresh: '/auth/portal/token:refresh',
  logout: '/auth/portal/logout',
  setupPassword: '/auth/portal/setup:password',
  forgotPassword: '/auth/portal/password:forgot',
  resetPassword: '/auth/portal/password:reset',
} as const;

export interface AuthApiOptions {
  config: Pick<RuntimeConfig, 'apiBaseUrl'>;
  getLocale: () => string;
  hasTranslation: HasTranslation;
  fetch?: typeof fetch;
  now?: () => number;
}

export interface AuthApi {
  login(body: PortalLoginRequest): Promise<AuthResult<PortalTokenResponse>>;
  refresh(csrf: string | null): Promise<AuthResult<PortalTokenResponse>>;
  logout(csrf: string | null): Promise<AuthResult<undefined>>;
  setupPassword(body: PortalSetupPasswordRequest): Promise<AuthResult<PortalSetupResult>>;
  forgotPassword(body: PortalForgotPasswordRequest): Promise<AuthResult<undefined>>;
  resetPassword(body: PortalResetPasswordRequest): Promise<AuthResult<undefined>>;
}

function setupOf(problem: Record<string, unknown>): SetupRequired | undefined {
  if (problem.code !== 'ACCOUNT_SETUP_REQUIRED' || typeof problem.setup_ticket !== 'string') return undefined;
  const steps = Array.isArray(problem.required_steps)
    ? problem.required_steps.filter((s): s is PortalSetupStep => s === 'update_password' || s === 'configure_otp')
    : [];
  return { setupTicket: problem.setup_ticket, requiredSteps: steps, otpSetupEmailSent: problem.otp_setup_email_sent === true };
}

export function createAuthApi(opts: AuthApiOptions): AuthApi {
  const fetchImpl = opts.fetch ?? ((...a: Parameters<typeof fetch>) => globalThis.fetch(...a));
  const now = opts.now ?? Date.now;

  async function post<T>(path: string, body?: unknown, csrf?: string | null): Promise<AuthResult<T>> {
    const headers = new Headers({ Accept: 'application/json, application/problem+json', 'Accept-Language': opts.getLocale(), traceparent: traceparent() });
    if (body !== undefined) headers.set('Content-Type', 'application/json');
    if (csrf) headers.set(CSRF_HEADER, csrf);
    let res: Response;
    try {
      res = await fetchImpl(`${opts.config.apiBaseUrl}${path}`, {
        method: 'POST',
        headers,
        body: body === undefined ? undefined : JSON.stringify(body),
        credentials: 'include',
        cache: 'no-store',
        referrerPolicy: 'no-referrer',
      });
    } catch {
      return { ok: false, status: 0, error: new ApiError({ status: 0, code: 'NETWORK_ERROR', i18nKey: i18nKeyFor('NETWORK_ERROR', opts.hasTranslation), retryable: true }) };
    }
    if (res.ok) {
      const data = res.status === 204 || res.status === 202 ? undefined : await res.json().catch(() => undefined);
      return { ok: true, status: res.status, data: data as T };
    }
    const problem = ((await res.json().catch(() => ({}))) ?? {}) as Record<string, unknown>;
    const error = toApiError(res.status, problem, opts.hasTranslation, { retryAfterMs: parseRetryAfter(res.headers.get('Retry-After'), now()) });
    return { ok: false, status: res.status, error, setup: setupOf(problem) };
  }

  return {
    login: (body) => post<PortalTokenResponse>(AUTH_PATHS.login, body),
    refresh: (csrf) => post<PortalTokenResponse>(AUTH_PATHS.refresh, undefined, csrf),
    logout: (csrf) => post<undefined>(AUTH_PATHS.logout, undefined, csrf),
    setupPassword: (body) => post<PortalSetupResult>(AUTH_PATHS.setupPassword, body),
    forgotPassword: (body) => post<undefined>(AUTH_PATHS.forgotPassword, body),
    resetPassword: (body) => post<undefined>(AUTH_PATHS.resetPassword, body),
  };
}
