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

/** Runtime configuration (`/config.json`, fallback `VITE_*` in dev) — AGENTS R3. */
export interface RuntimeConfig {
  apiBaseUrl: string;
  apiInternalBaseUrl: string;
  oidcIssuer: string;
  oidcClientId: string;
  env: string;
}

export interface FieldError {
  code: string;
  message?: string;
}

export type ApiEventType = 'session_expired' | 'tenant_mismatch' | 'mfa_required' | 'vpn_required';
