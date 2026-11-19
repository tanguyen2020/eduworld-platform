// Tenant filter of the System Admin (F051 T045, F051-AC-22): selected tenant ids (empty = "Tất cả tenant"), kept
// for the browser session (`sessionStorage` `ew.portal.tenantFilter`, data-model) and readable by any screen.
import { useSyncExternalStore } from 'react';
import { readItem, STORAGE_KEYS, writeItem, removeItem } from '../shared/storage';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const listeners = new Set<() => void>();
let current: readonly string[] | null = null;

function load(): readonly string[] {
  try {
    const parsed = JSON.parse(readItem('session', STORAGE_KEYS.tenantFilter) ?? '[]') as unknown;
    return Array.isArray(parsed) ? parsed.filter((x): x is string => typeof x === 'string' && UUID.test(x)) : [];
  } catch {
    return [];
  }
}

export const tenantFilterStore = {
  get(): readonly string[] {
    current ??= load();
    return current;
  },
  set(ids: readonly string[]): void {
    current = [...new Set(ids)];
    if (current.length) writeItem('session', STORAGE_KEYS.tenantFilter, JSON.stringify(current));
    else removeItem('session', STORAGE_KEYS.tenantFilter);
    listeners.forEach((l) => l());
  },
  reset(): void {
    current = null;
    listeners.forEach((l) => l());
  },
  subscribe(l: () => void): () => void {
    listeners.add(l);
    return () => listeners.delete(l);
  },
};

export function useTenantFilter(): readonly string[] {
  return useSyncExternalStore(tenantFilterStore.subscribe, tenantFilterStore.get, tenantFilterStore.get);
}
