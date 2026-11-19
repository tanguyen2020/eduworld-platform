// idempotency (AGENTS R10, ADR-0089 §4, F050-AC-12): operations with a required Idempotency-Key get a UUIDv7.
// The key is fixed before the retry loop (automatic retries reuse it). A user retrying the same action passes an
// intent id (header `x-ew-intent`, stripped before sending) and gets the same key back while it is younger than
// `idempotency_record_ttl` (24 h, CONFIG_DEFAULTS).
import { uuidv7, uuidv7Time } from '../ids';

export const IDEMPOTENCY_HEADER = 'Idempotency-Key';
export const INTENT_HEADER = 'x-ew-intent';
/** idempotency_record_ttl (CONFIG_DEFAULTS, API_DESIGN): the server forgets keys older than this. */
export const IDEMPOTENCY_RECORD_TTL_MS = 24 * 60 * 60 * 1000;

export class IdempotencyKeys {
  private readonly byIntent = new Map<string, string>();

  constructor(private readonly now: () => number = Date.now) {}

  /** Same key for the same intent while it is valid; a fresh UUIDv7 otherwise. */
  forIntent(intent: string): string {
    const existing = this.byIntent.get(intent);
    if (existing && this.now() - uuidv7Time(existing) < IDEMPOTENCY_RECORD_TTL_MS) return existing;
    const key = uuidv7(this.now());
    this.byIntent.set(intent, key);
    return key;
  }

  /** Forget an intent once its operation succeeded. */
  complete(intent: string): void {
    this.byIntent.delete(intent);
  }
}

/** Sets Idempotency-Key when the operation requires it; returns the intent id (if any) for completion. */
export function applyIdempotency(headers: Headers, required: boolean, keys: IdempotencyKeys, now: () => number = Date.now): string | null {
  const intent = headers.get(INTENT_HEADER);
  headers.delete(INTENT_HEADER);
  if (!required) return null;
  if (!headers.has(IDEMPOTENCY_HEADER)) headers.set(IDEMPOTENCY_HEADER, intent ? keys.forIntent(intent) : uuidv7(now()));
  return intent;
}
