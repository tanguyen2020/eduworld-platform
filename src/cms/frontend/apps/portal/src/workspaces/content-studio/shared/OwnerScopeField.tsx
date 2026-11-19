// OwnerScopeField (F053-AC-12, ADR-0027, F051-AC-25): a System Admin creating content must pick its owner — Platform
// (owner_scope = platform, no X-Tenant-Id) or one target tenant (owner_scope = tenant, X-Tenant-Id + tenant_id).
// Tenant users never see it (their session tenant applies).
import { TenantTargetField } from '../../../shell/TenantControls';
import { PLATFORM_TARGET, targetTenantHeaders } from '../../../shared/api/withTenantScope';

export const OWNER_FIELD = 'owner_target';

export function OwnerScopeField() {
  return <TenantTargetField name={OWNER_FIELD} allowPlatform />;
}

/** Body fields + headers of a create for the chosen owner (System Admin) or for a tenant user (`target` null). */
export function ownerOf(target: string | null | undefined): { body: { owner_scope?: 'platform' | 'tenant'; tenant_id?: string }; headers: Record<string, string> } {
  if (!target) return { body: {}, headers: {} };
  if (target === PLATFORM_TARGET) return { body: { owner_scope: 'platform' }, headers: {} };
  return { body: { owner_scope: 'tenant', tenant_id: target }, headers: targetTenantHeaders(target) };
}
