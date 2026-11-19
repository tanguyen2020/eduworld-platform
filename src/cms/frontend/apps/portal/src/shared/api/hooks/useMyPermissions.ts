// useMyPermissions (F051 T033, F051-AC-09/12, research R3): `getMyPermissions` of the session scope (X-Tenant-Id of
// the working tenant, none in platform scope). Query key ['me','permissions', scopeKey]; refreshed every
// `principal_cache_ttl` (runtime config), on window focus and right after `403 PERMISSION_DENIED`/`FEATURE_DISABLED`
// (SessionProvider invalidates the key). Used to build menus only — never a security decision (ADR-0027).
import { useQuery } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { components } from '../schema.gen';

export type PermissionSet = components['schemas']['PermissionSet'];

export const permissionsKey = (scope: string) => ['me', 'permissions', scope] as const;

/** Query options shared by the hook and its unit test (F051-AC-12). */
export function permissionsQueryTiming(principalCacheTtlSeconds: number) {
  const ms = principalCacheTtlSeconds * 1000;
  return { staleTime: ms, refetchInterval: ms, refetchOnWindowFocus: 'always' as const };
}

export function useMyPermissions() {
  const { api, config } = usePortalRuntime();
  const { state, scopeKey } = useSession();
  return useQuery({
    queryKey: permissionsKey(scopeKey),
    queryFn: () => api.call<PermissionSet>(api.client.GET('/me/permissions', {})),
    enabled: state.status === 'ready',
    ...permissionsQueryTiming(config.principalCacheTtlSeconds),
  });
}
