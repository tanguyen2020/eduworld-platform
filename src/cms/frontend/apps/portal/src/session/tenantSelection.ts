// tenantSelection (F051 T020, F051-AC-05/06/28): which tenant the session works in. Choosing a tenant never asks for
// a new token (ADR-0091 mục 4): the same access token is used and the apiClient sends `X-Tenant-Id` = this tenant.
// Per-viewer conveniences (data-model): remembered tenant per user in localStorage (checkbox "Nhớ lựa chọn"),
// tenant of this tab in sessionStorage (F5 restore). Both are ids, not secrets.
import { readItem, removeItem, STORAGE_KEYS, writeItem } from '../shared/storage';
import type { TenantSource } from '../shared/auth/TenantContext';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const valid = (v: string | null) => (v && UUID.test(v) ? v : null);

export function readRememberedTenant(userKey: string | null): string | null {
  return userKey ? valid(readItem('local', STORAGE_KEYS.rememberedTenant(userKey))) : null;
}

export function writeRememberedTenant(userKey: string | null, tenantId: string | null): void {
  if (!userKey) return;
  if (tenantId) writeItem('local', STORAGE_KEYS.rememberedTenant(userKey), tenantId);
  else removeItem('local', STORAGE_KEYS.rememberedTenant(userKey));
}

export function readSessionTenant(): string | null {
  return valid(readItem('session', STORAGE_KEYS.sessionTenantId));
}

export function writeSessionTenant(tenantId: string | null): void {
  if (tenantId) writeItem('session', STORAGE_KEYS.sessionTenantId, tenantId);
  else removeItem('session', STORAGE_KEYS.sessionTenantId);
}

/**
 * Working tenant read by the apiClient (`TenantSource`). Updated synchronously before any query of the new scope
 * runs, so no request leaves with the previous tenant header after a switch (SC-003).
 */
export class WorkingTenant implements TenantSource {
  private id: string | null = null;

  getTenantId(): string | null {
    return this.id;
  }

  set(id: string | null): void {
    this.id = id;
  }
}
