// Runtime configuration (F050-AC-03, AGENTS R3): `/config.json` generated at deploy time; in development the
// `VITE_*` variables are the fallback. No environment URL lives in the source.
import type { RuntimeConfig } from '../shared/api/types';

export const CONFIG_PATH = '/config.json';

const KEYS = ['apiBaseUrl', 'apiInternalBaseUrl', 'oidcIssuer', 'oidcClientId', 'env'] as const;
const ENV_KEYS: Record<(typeof KEYS)[number], string> = {
  apiBaseUrl: 'VITE_API_BASE_URL',
  apiInternalBaseUrl: 'VITE_API_INTERNAL_BASE_URL',
  oidcIssuer: 'VITE_OIDC_ISSUER',
  oidcClientId: 'VITE_OIDC_CLIENT_ID',
  env: 'VITE_APP_ENV',
};
const URL_KEYS = new Set(['apiBaseUrl', 'apiInternalBaseUrl', 'oidcIssuer']);

export class RuntimeConfigError extends Error {
  constructor(readonly missing: string[]) {
    super(`runtime config incomplete: ${missing.join(', ')}`);
    this.name = 'RuntimeConfigError';
  }
}

/** Validates a candidate object; returns the config or the list of missing/invalid keys. */
export function validateRuntimeConfig(raw: unknown): RuntimeConfig | string[] {
  const obj = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const bad = KEYS.filter((k) => {
    const v = obj[k];
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
  if (bad.length) return bad;
  const out = {} as RuntimeConfig;
  for (const k of KEYS) out[k] = String(obj[k]).replace(/\/+$/, '');
  return out;
}

type Env = Record<string, string | boolean | undefined>;

function fromEnv(env: Env): RuntimeConfig | string[] {
  const raw: Record<string, unknown> = {};
  for (const k of KEYS) raw[k] = env[ENV_KEYS[k]];
  const r = validateRuntimeConfig(raw);
  return Array.isArray(r) ? r.map((k) => ENV_KEYS[k as (typeof KEYS)[number]]) : r;
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
