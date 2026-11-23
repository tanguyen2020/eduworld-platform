// Moderation status of the banner "Đang tắt kiểm duyệt" (F122 T028; F122-AC-13, AC-14; ADR-0030): refreshed every 30 s
// (≤ 60 s to appear or disappear).
// - tenant staff (session tenant, tenant.settings.read or school.settings.read): getTenantModerationStatus of THEIR tenant —
//   steps off and the earliest expiry, never the reason (another tenant's state never reaches them);
// - System Admin (chat.policy.toggle — the only holder module chat answers): listModerationSwitches for the tenants of
//   the topbar filter (platform row always); without a filter, the NUMBER of tenants with a step off
//   (disabled_tenant_count, counted by module chat in SQL — F122 T065), never a list of tenants.
// Also the count of school notifications waiting for an acknowledgement (menu badge, F122-AC-18).
// Kept in memory only (TanStack Query), never in browser storage.
import { useQuery } from '@tanstack/react-query';
import { usePortalRuntime } from '../../app/runtime';
import { usePermissions } from '../../permissions/usePermissions';
import { useSession } from '../../session/SessionProvider';
import { useTenantFilter } from '../../shell/tenantFilterStore';
import type { components } from '../api/schema.gen';

type Schemas = components['schemas'];
export type TenantModerationStatus = Schemas['TenantModerationStatus'];
export type SwitchState = Schemas['ModerationSwitchState'];

export const MODERATION_STATUS_REFRESH_MS = 30_000;

export interface DisabledLevel {
  /** null = the platform level. */
  tenantId: string | null;
  tenantName?: string;
  steps: string[];
  allOff: boolean;
  expiresAt?: string | null;
}

export interface ModerationBannerState {
  levels: DisabledLevel[];
  /** Platform staff without a tenant filter: tenants with a step off at the tenant level (0 = none). */
  disabledTenantCount: number;
}

export function useModerationBannerState(): ModerationBannerState {
  const { api } = usePortalRuntime();
  const { state } = useSession();
  const { view } = usePermissions();
  const filter = useTenantFilter();
  const perms = view?.permissions ?? [];
  const tenantId = state.status === 'ready' && state.scope?.kind === 'tenant' ? state.scope.tenantId : null;
  const platform = state.status === 'ready' && state.scope?.kind === 'platform';
  const tenantReader = !!tenantId && (perms.includes('tenant.settings.read') || perms.includes('school.settings.read'));
  // listModerationSwitches answers chat.policy.toggle only (System Admin): nobody else calls it, so no refused request every 30 s
  // (the Safety Moderator would only get 403 and no banner — Clarifications 2026-11-24)
  const platformReader = platform && perms.includes('chat.policy.toggle');
  const tenantQ = useQuery({
    queryKey: ['chat-safety', 'moderation-status', tenantId],
    enabled: tenantReader,
    refetchInterval: MODERATION_STATUS_REFRESH_MS,
    retry: false,
    queryFn: () => api.call<TenantModerationStatus>(api.client.GET('/tenant/chat/moderation-status', {})),
  });
  const ids = [...filter];
  const platformQ = useQuery({
    queryKey: ['chat-safety', 'switch-banner', ids],
    enabled: platformReader,
    refetchInterval: MODERATION_STATUS_REFRESH_MS,
    retry: false,
    queryFn: () =>
      api.call<{ platform: SwitchState; tenants: SwitchState[]; disabled_tenant_count?: number }>(
        api.client.GET('/admin/chat/moderation-switches', { params: { query: ids.length ? { tenant_ids: ids } : {} } }),
      ),
  });
  return {
    levels: bannerLevels(tenantReader ? tenantQ.data : undefined, platformReader ? platformQ.data : undefined),
    disabledTenantCount: platformReader && !ids.length ? (platformQ.data?.disabled_tenant_count ?? 0) : 0,
  };
}

/** Steps off of the session tenant (tenant staff) or of the platform + filtered tenants (platform staff). */
function bannerLevels(tenant: TenantModerationStatus | undefined, platform: { platform: SwitchState; tenants: SwitchState[] } | undefined): DisabledLevel[] {
  if (tenant) {
    // the server already resolved expiry; a lapsed expires_at hides the banner before the next refresh
    if (tenant.expires_at && Date.parse(tenant.expires_at) <= Date.now()) return [];
    return tenant.disabled_steps.length || !tenant.enabled_all
      ? [{ tenantId: tenant.tenant_id, steps: [...tenant.disabled_steps], allOff: !tenant.enabled_all, expiresAt: tenant.expires_at }]
      : [];
  }
  if (platform) {
    return [platform.platform, ...platform.tenants]
      .filter((l) => (l.disabled_steps?.length ?? 0) > 0 || !l.effective.enabled_all)
      .filter((l) => !l.configured?.expires_at || Date.parse(l.configured.expires_at) > Date.now())
      .map((l) => ({ tenantId: l.tenant_id ?? null, tenantName: l.tenant_name, steps: [...(l.disabled_steps ?? [])], allOff: !l.effective.enabled_all, expiresAt: l.configured?.expires_at ?? null }));
  }
  return [];
}

/** Number of school safety notifications waiting for an acknowledgement (menu badge; first page only, "50+" beyond). */
export function usePendingSafetyNotifications(enabled: boolean): { count: number; more: boolean } {
  const { api } = usePortalRuntime();
  const { state } = useSession();
  const tenantId = state.status === 'ready' && state.scope?.kind === 'tenant' ? state.scope.tenantId : null;
  const q = useQuery({
    queryKey: ['chat-safety', 'pending-notifications', tenantId],
    enabled: enabled && !!tenantId,
    refetchInterval: MODERATION_STATUS_REFRESH_MS * 2,
    retry: false,
    queryFn: () =>
      api.call<{ data: unknown[]; page: Schemas['PageInfo'] }>(api.client.GET('/tenant/safety/school-notifications', { params: { query: { acknowledged: false, limit: 50 } } })),
  });
  return { count: q.data?.data.length ?? 0, more: !!q.data?.page.has_more };
}
