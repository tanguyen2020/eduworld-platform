// Tenant scope of workspace C screens (F052-AC-24, F051-AC-24, ADMIN_PORTAL §2.5): every F052 operation takes only
// X-Tenant-Id. A tenant user works in the session tenant (the apiClient sets the header); a System Admin in platform
// scope must pick exactly one tenant in the topbar filter, which becomes the header of every read and write. Query keys
// carry the effective tenant so data of one tenant never shows under another. Display/plumbing only (AGENTS R1, R20).
import { Alert } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { usePermissions } from '../../../permissions/usePermissions';
import { useSession } from '../../../session/SessionProvider';
import { requireSingleTenant } from '../../../shared/api/withTenantScope';
import { useTenantFilter } from '../../../shell/tenantFilterStore';

export interface SchoolLmsScope {
  ready: boolean;
  /** Extra headers of every call (X-Tenant-Id of the System Admin's chosen tenant; empty for tenant users). */
  headers: Record<string, string>;
  /** Query-key segment: the tenant the data belongs to. */
  key: string;
}

/** Pure resolution, unit-tested: session tenant → ready; platform scope → exactly one filtered tenant. */
export function resolveScope(sessionTenantId: string | null, crossTenant: boolean, filter: readonly string[]): SchoolLmsScope {
  if (sessionTenantId) return { ready: true, headers: {}, key: sessionTenantId };
  const single = requireSingleTenant({ crossTenant, filter });
  if (!single.ready) return { ready: false, headers: {}, key: 'none' };
  return { ready: true, headers: single.headers, key: filter[0] ?? 'none' };
}

export function useSchoolLmsScope(): SchoolLmsScope {
  const { state } = useSession();
  const { crossTenant } = usePermissions();
  const filter = useTenantFilter();
  const sessionTenant = state.scope?.kind === 'tenant' ? state.scope.tenantId : null;
  return resolveScope(sessionTenant, crossTenant, filter);
}

/** Renders its children only when one tenant is in scope; otherwise asks the System Admin to pick one (F052-AC-24). */
export function SingleTenantGate({ children }: { children: (scope: SchoolLmsScope) => ReactNode }) {
  const { t } = useTranslation();
  const scope = useSchoolLmsScope();
  if (!scope.ready) {
    return <Alert type="info" showIcon role="status" message={t('portal.school_lms.scope.pickOneTenant')} description={t('portal.school_lms.scope.pickOneTenantHelp')} />;
  }
  return <>{children(scope)}</>;
}
