// Browser storage helpers (F051 data-model "Lưu trữ trình duyệt"): per-viewer conveniences only, never tokens,
// passwords, OTP, setup tickets, reset tokens, emails or student data (F051-AC-02, AGENTS R14). Every access is
// guarded: storage may be blocked (private mode) and the portal must keep working without it.

export const STORAGE_KEYS = {
  deviceId: 'ew.portal.deviceId',
  otpHint: 'ew.portal.otpHint',
  lastActivityAt: 'ew.portal.lastActivityAt',
  sessionTenantId: 'ew.portal.sessionTenantId',
  tenantFilter: 'ew.portal.tenantFilter',
  rememberedTenant: (userKey: string) => `ew.portal.${userKey}.rememberedTenantId`,
  lastWorkspace: (userKey: string) => `ew.portal.${userKey}.lastWorkspace`,
} as const;

type Area = 'local' | 'session';

const store = (area: Area): Storage => (area === 'local' ? window.localStorage : window.sessionStorage);

export function readItem(area: Area, key: string): string | null {
  try {
    return store(area).getItem(key);
  } catch {
    return null;
  }
}

export function writeItem(area: Area, key: string, value: string): void {
  try {
    store(area).setItem(key, value);
  } catch {
    // storage unavailable: convenience only
  }
}

export function removeItem(area: Area, key: string): void {
  try {
    store(area).removeItem(key);
  } catch {
    // storage unavailable
  }
}
