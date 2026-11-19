// device_id (F051 T017, F051-AC-30, ADR-0091): random UUID per browser sent with `portalLogin`/`forgotPortalPassword`/
// `resetPortalPassword` for rate limiting by device (never by IP — AGENTS R30). Not a secret. When localStorage is
// blocked a new id is generated for each page load (spec edge case).
import { readItem, STORAGE_KEYS, writeItem } from '../shared/storage';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
let perLoad: string | null = null;

export function deviceId(random: () => string = () => crypto.randomUUID()): string {
  const stored = readItem('local', STORAGE_KEYS.deviceId);
  if (stored && UUID.test(stored)) return stored;
  perLoad ??= random();
  writeItem('local', STORAGE_KEYS.deviceId, perLoad);
  return perLoad;
}

/** Test helper: forget the per-load id. */
export function resetDeviceIdForTest(): void {
  perLoad = null;
}
