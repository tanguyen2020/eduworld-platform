// TenantContext (F050 T030, ADR-0065): the working tenant of the portal session, or null (System Admin reading
// across tenants). F051 fills it after login / tenant picker. apiClient reads it for `X-Tenant-Id`.
import { createContext, useContext, type ReactNode } from 'react';

export interface TenantSource {
  /** Working tenant id (UUID) or null when none is selected. */
  getTenantId(): string | null;
}

export const noTenant: TenantSource = { getTenantId: () => null };

const Ctx = createContext<TenantSource>(noTenant);

export function TenantContextProvider({ source, children }: { source: TenantSource; children: ReactNode }) {
  return <Ctx.Provider value={source}>{children}</Ctx.Provider>;
}

export function useTenantSource(): TenantSource {
  return useContext(Ctx);
}
