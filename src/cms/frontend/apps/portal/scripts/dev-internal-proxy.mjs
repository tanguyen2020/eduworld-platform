// Local-dev-only proxy to the internal gateway (LOCAL_DEVELOPMENT §6.4). The internal gateway never serves CORS
// (F004-AC-21), so the Vite dev server forwards the same-origin prefix DEV_INTERNAL_PREFIX to it and the dev config
// (public/config.example.json) points apiInternalBaseUrl at `<dev origin>/__internal/api/v1`. The target comes from
// EW_PORTAL_DEV_INTERNAL_GATEWAY (no URL in source, AGENTS R3); without it there is no proxy and the internal
// workspaces show the VPN notice as before. Production builds never use this (Vite `serve` only, config.json at deploy).

export const DEV_INTERNAL_PREFIX = '/__internal';
export const DEV_INTERNAL_ENV = 'EW_PORTAL_DEV_INTERNAL_GATEWAY';

/**
 * Builds the Vite `server.proxy` entry, or null when the variable is unset. Only an http(s) origin without
 * credentials, path, query or fragment is accepted, so the proxy cannot be pointed at an arbitrary path.
 * @param {Record<string, string | undefined>} env
 * @returns {Record<string, import('vite').ProxyOptions> | null}
 */
export function devInternalProxy(env) {
  const raw = (env[DEV_INTERNAL_ENV] ?? '').trim();
  if (!raw) return null;
  let url;
  try {
    url = new URL(raw);
  } catch {
    throw new Error(`${DEV_INTERNAL_ENV} must be an http(s) origin`);
  }
  if (!/^https?:$/.test(url.protocol) || url.username || url.password || url.search || url.hash || url.pathname.replace(/\/+$/, '') !== '') {
    throw new Error(`${DEV_INTERNAL_ENV} must be an http(s) origin without user, path, query or fragment`);
  }
  const prefix = new RegExp(`^${DEV_INTERNAL_PREFIX}(?=/|$)`);
  return {
    [DEV_INTERNAL_PREFIX]: {
      target: url.origin,
      changeOrigin: true,
      rewrite: (p) => p.replace(prefix, '') || '/',
    },
  };
}
