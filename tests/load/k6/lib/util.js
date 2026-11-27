// Pure helpers of the F132 k6 scenarios (no k6 import, so node tests can load them).

function hex(n) {
  let s = '';
  for (let i = 0; i < n; i++) s += Math.floor(Math.random() * 16).toString(16);
  return s;
}

/** Random UUIDv4 (client_message_id, ad-hoc device ids). */
export function uuidv4() {
  const v = '89ab'[Math.floor(Math.random() * 4)];
  return `${hex(8)}-${hex(4)}-4${hex(3)}-${v}${hex(3)}-${hex(12)}`;
}

/** Time-ordered UUIDv7 (Idempotency-Key must be a recent UUIDv7). */
export function uuidv7(now = Date.now()) {
  const t = now.toString(16).padStart(12, '0');
  const v = '89ab'[Math.floor(Math.random() * 4)];
  return `${t.slice(0, 8)}-${t.slice(8, 12)}-7${hex(3)}-${v}${hex(3)}-${hex(12)}`;
}

/** Wait (s) of a 429: Retry-After seconds (missing/invalid/< 1 → 1 s, capped at maxS) plus up to 1 s jitter. */
export function retryWait(retryAfter, maxS = 120, jitter = Math.random()) {
  const n = parseInt(retryAfter, 10);
  const base = Number.isFinite(n) && n >= 1 ? Math.min(n, maxS) : 1;
  return base + jitter;
}
