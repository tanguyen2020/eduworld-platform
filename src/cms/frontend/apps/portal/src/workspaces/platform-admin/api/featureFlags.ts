// Feature flag hooks (F059 T043, F059-AC-20/21, FR-ORG-007, ADR-0090): listFeatureFlags (`flag_kind`, `tenant_id` to
// read that tenant's override, cursor) and putFeatureFlag (scope platform/tenant, state, mandatory reason; If-Match =
// version of the row; X-Tenant-Id = target tenant for a tenant override). Flags are declared in code / the menu
// catalog: no create, no delete (AGENTS R3). `412` → the list is reloaded and the editor shows "Đã có người sửa".
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { PAGE_SIZE } from '../limits';
import { useCursorList, usePlatformApi, type CursorPage, type Schemas } from './common';

export type FeatureFlag = Schemas['FeatureFlag'];
export type FeatureFlagKind = Schemas['FeatureFlagKind'];
export type FeatureFlagWrite = Schemas['FeatureFlagWrite'];
export type FeatureFlagState = Schemas['FeatureFlagState'];

export const FLAG_KINDS: readonly FeatureFlagKind[] = ['feature', 'menu', 'action'];

export function useFeatureFlags(kind: FeatureFlagKind | undefined, tenantId: string | undefined) {
  const api = usePlatformApi();
  const query = { ...(kind ? { flag_kind: kind } : {}), ...(tenantId ? { tenant_id: tenantId } : {}) };
  return useCursorList<FeatureFlag>(['platform', 'feature-flags', query], (cursor) =>
    api.call<CursorPage<FeatureFlag>>(api.client.GET('/admin/feature-flags', { params: { query: { ...query, limit: PAGE_SIZE, ...(cursor ? { cursor } : {}) } } })),
  );
}

export function usePutFeatureFlag() {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ flag, body, tenantId }: { flag: Pick<FeatureFlag, 'flag_key' | 'version'>; body: FeatureFlagWrite; tenantId?: string }) => {
      api.etags.remember(`/admin/feature-flags/${flag.flag_key}`, `W/"${flag.version}"`);
      return api.call<FeatureFlag>(
        api.client.PUT('/admin/feature-flags/{flag_key}', {
          params: { path: { flag_key: flag.flag_key } },
          body,
          headers: body.scope === 'tenant' && tenantId ? scope.writeHeaders(tenantId) : {},
        }),
      );
    },
    onSettled: () => qc.invalidateQueries({ queryKey: ['platform', 'feature-flags'] }),
  });
}
