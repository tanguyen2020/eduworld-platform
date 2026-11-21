// Types for dev-internal-proxy.mjs (used from the Vite TS config).
import type { ProxyOptions } from 'vite';

export const DEV_INTERNAL_PREFIX: '/__internal';
export const DEV_INTERNAL_ENV: 'EW_PORTAL_DEV_INTERNAL_GATEWAY';
export function devInternalProxy(env: Record<string, string | undefined>): Record<string, ProxyOptions> | null;
