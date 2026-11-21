// apiClient types (F050 data-model).

export type Gateway = 'public' | 'internal';
export type TenantHeaderMode = 'required' | 'optional' | 'none';

/** Generated per operation of the merged spec (operation-meta.gen.ts). */
export interface OperationMeta {
  operationId: string;
  method: string;
  path: string;
  /** `x-service`: one module or a list. */
  service: string | readonly string[] | null;
  gateway: Gateway;
  tenantHeader: TenantHeaderMode;
  idempotency: boolean;
  ifMatch: boolean;
}

/** Runtime configuration (`/config.json`, fallback `VITE_*` in dev) — AGENTS R3, F051 T002 (no OIDC keys, ADR-0091). */
export interface RuntimeConfig {
  apiBaseUrl: string;
  apiInternalBaseUrl: string;
  env: string;
  /** CONFIG_DEFAULTS `principal_cache_ttl` (seconds): longest refresh period of getMyPermissions (F051-AC-12). */
  principalCacheTtlSeconds: number;
  /** CONFIG_DEFAULTS `portal_session_sso` idle (seconds): no interaction for longer ends the session (F051-AC-29). */
  sessionIdleTimeoutSeconds: number;
}

export interface FieldError {
  code: string;
  message?: string;
}

/**
 * apiClient events the shell reacts to (F050 plan, F051 interceptors): session expired, tenant mismatch or tenant
 * access disabled (F051-AC-21), VPN (F051-AC-14), staff only (F051-AC-04), feature
 * disabled (F051-AC-37), permission denied (F051-AC-12).
 */
export type ApiEventType =
  | 'session_expired'
  | 'tenant_mismatch'
  | 'vpn_required'
  | 'staff_only'
  | 'feature_disabled'
  | 'permission_denied';
