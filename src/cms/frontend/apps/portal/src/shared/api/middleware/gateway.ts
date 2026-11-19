// gateway (F050-AC-13, Clarification 2026-11-16): /admin/, /sis/, /game-admin/, /game-media go to the internal
// gateway (VPN only), everything else to the public gateway. Base URLs come from the runtime config (AGENTS R3).
import type { Gateway, RuntimeConfig } from '../types';

export function baseUrlFor(gateway: Gateway, config: RuntimeConfig): string {
  return gateway === 'internal' ? config.apiInternalBaseUrl : config.apiBaseUrl;
}

/** Path of a request URL relative to the public base (`/api/v1/me/tenants` → `/me/tenants`). */
export function apiPath(url: URL, config: RuntimeConfig): string {
  const basePath = new URL(config.apiBaseUrl).pathname.replace(/\/+$/, '');
  const p = basePath && url.pathname.startsWith(basePath) ? url.pathname.slice(basePath.length) : url.pathname;
  return p.startsWith('/') ? p : `/${p}`;
}

export function targetUrl(gateway: Gateway, path: string, search: string, config: RuntimeConfig): string {
  return `${baseUrlFor(gateway, config)}${path}${search}`;
}
