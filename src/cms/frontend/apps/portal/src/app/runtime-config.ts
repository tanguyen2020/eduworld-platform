// Runtime configuration (F050-AC-03, F051 T001/T002, AGENTS R3, R33): `/config.json` generated at deploy time; in
// development the `VITE_*` variables are the fallback. No environment URL and no default for URLs lives in the
// source. ADR-0091: the portal never talks to Keycloak from the browser, so there is no OIDC issuer/client id.
// Durations come from CONFIG_DEFAULTS (`principal_cache_ttl`, `portal_session_sso` idle) through the config.
import type { RuntimeConfig } from '../shared/api/types';

export const CONFIG_PATH = '/config.json';

const STRING_KEYS = ['apiBaseUrl', 'apiInternalBaseUrl', 'env'] as const;
const NUMBER_KEYS = ['principalCacheTtlSeconds', 'sessionIdleTimeoutSeconds'] as const;
type Key = (typeof STRING_KEYS)[number] | (typeof NUMBER_KEYS)[number];
const KEYS: readonly Key[] = [...STRING_KEYS, ...NUMBER_KEYS];

const ENV_KEYS: Record<Key, string> = {
  apiBaseUrl: 'VITE_API_BASE_URL',
  apiInternalBaseUrl: 'VITE_API_INTERNAL_BASE_URL',
  env: 'VITE_APP_ENV',
  principalCacheTtlSeconds: 'VITE_PRINCIPAL_CACHE_TTL_SECONDS',
  sessionIdleTimeoutSeconds: 'VITE_SESSION_IDLE_TIMEOUT_SECONDS',
};
/** Optional keys (F079-AC-17): absent leaves the feature off; present must be a valid URL. */
const OPTIONAL_URL_KEYS = { gamePreviewUrl: 'VITE_GAME_PREVIEW_URL' } as const;

const optionalUrl = (v: unknown): string | undefined | null => {
  if (v === undefined || v === null || v === '') return undefined;
  if (typeof v !== 'string') return null;
  try {
    new URL(v);
    return v.replace(/\/+$/, '');
  } catch {
    return null;
  }
};
const URL_KEYS = new Set<Key>(['apiBaseUrl', 'apiInternalBaseUrl']);

export class RuntimeConfigError extends Error {
  constructor(readonly missing: string[]) {
    super(`runtime config incomplete: ${missing.join(', ')}`);
    this.name = 'RuntimeConfigError';
  }
}

const positiveInt = (v: unknown): number | null => {
  const n = typeof v === 'string' && v.trim() !== '' ? Number(v) : v;
  return typeof n === 'number' && Number.isInteger(n) && n > 0 ? n : null;
};

/** Validates a candidate object; returns the config or the list of missing/invalid keys. */
export function validateRuntimeConfig(raw: unknown): RuntimeConfig | string[] {
  const obj = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const bad = KEYS.filter((k) => {
    const v = obj[k];
    if ((NUMBER_KEYS as readonly string[]).includes(k)) return positiveInt(v) === null;
    if (typeof v !== 'string' || v.trim() === '') return true;
    if (URL_KEYS.has(k)) {
      try {
        new URL(v);
      } catch {
        return true;
      }
    }
    return false;
  });
  const preview = optionalUrl(obj.gamePreviewUrl);
  if (preview === null) bad.push('gamePreviewUrl' as Key);
  if (bad.length) return bad;
  return {
    ...(preview ? { gamePreviewUrl: preview } : {}),
    apiBaseUrl: String(obj.apiBaseUrl).replace(/\/+$/, ''),
    apiInternalBaseUrl: String(obj.apiInternalBaseUrl).replace(/\/+$/, ''),
    env: String(obj.env),
    principalCacheTtlSeconds: positiveInt(obj.principalCacheTtlSeconds)!,
    sessionIdleTimeoutSeconds: positiveInt(obj.sessionIdleTimeoutSeconds)!,
  };
}

type Env = Record<string, string | boolean | undefined>;

function fromEnv(env: Env): RuntimeConfig | string[] {
  const raw: Record<string, unknown> = {};
  for (const k of KEYS) raw[k] = env[ENV_KEYS[k]];
  raw.gamePreviewUrl = env[OPTIONAL_URL_KEYS.gamePreviewUrl];
  const r = validateRuntimeConfig(raw);
  return Array.isArray(r) ? r.map((k) => (k === 'gamePreviewUrl' ? OPTIONAL_URL_KEYS.gamePreviewUrl : ENV_KEYS[k as Key])) : r;
}

/**
 * Loads `/config.json`; when it is missing (404) or invalid and `allowEnvFallback` (dev), falls back to `VITE_*`.
 * Throws RuntimeConfigError when neither source is complete.
 */
export async function loadRuntimeConfig(
  fetchFn: typeof fetch = fetch,
  env: Env = import.meta.env as Env,
  allowEnvFallback: boolean = Boolean(env.DEV),
): Promise<RuntimeConfig> {
  let missing: string[] = [...KEYS];
  try {
    const res = await fetchFn(CONFIG_PATH, { cache: 'no-store', headers: { Accept: 'application/json' } });
    if (res.ok) {
      const r = validateRuntimeConfig(await res.json());
      if (!Array.isArray(r)) return r;
      missing = r;
    }
  } catch {
    // network error or invalid JSON: fall through to the env fallback
  }
  if (allowEnvFallback) {
    const r = fromEnv(env);
    if (!Array.isArray(r)) return r;
    missing = r;
  }
  throw new RuntimeConfigError(missing);
}
