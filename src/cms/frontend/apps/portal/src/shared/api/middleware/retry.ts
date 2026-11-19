// retry (API_DESIGN §12.3, F050-AC-15/16): only GET or requests carrying an Idempotency-Key are retried, on a
// network error or 502/503/504, at most MAX_RETRIES times with exponential backoff + jitter; 4xx are never
// retried except 429, which waits for Retry-After.
export const MAX_RETRIES = 2;
export const BASE_BACKOFF_MS = 100;
/** Retry-After longer than this is not waited for automatically: the error goes back to the caller. */
export const MAX_RETRY_AFTER_MS = 30_000;
export const RETRYABLE_STATUS = new Set([502, 503, 504]);

export function isRetryable(method: string, headers: Headers): boolean {
  return method.toUpperCase() === 'GET' || headers.has('Idempotency-Key');
}

export function backoffMs(attempt: number, random: () => number = Math.random): number {
  const base = BASE_BACKOFF_MS * 2 ** attempt;
  return base + Math.floor(random() * base);
}

/** Retry-After as delta-seconds or HTTP date → milliseconds (≥ 0), or null when absent/invalid. */
export function parseRetryAfter(value: string | null, now: number = Date.now()): number | null {
  if (!value) return null;
  if (/^\d+$/.test(value.trim())) return Number(value.trim()) * 1000;
  const at = Date.parse(value);
  return Number.isNaN(at) ? null : Math.max(0, at - now);
}
