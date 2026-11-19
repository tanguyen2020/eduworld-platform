// useMe (F051 T022): `getMe` of the session scope for the user menu (display name). Same refresh as permissions.
import { useQuery } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { components } from '../schema.gen';
import { permissionsQueryTiming } from './useMyPermissions';

export type Principal = components['schemas']['Principal'];

export function useMe() {
  const { api, config } = usePortalRuntime();
  const { state, scopeKey } = useSession();
  return useQuery({
    queryKey: ['me', 'principal', scopeKey],
    queryFn: () => api.call<Principal>(api.client.GET('/auth/me', {})),
    enabled: state.status === 'ready',
    ...permissionsQueryTiming(config.principalCacheTtlSeconds),
    refetchOnWindowFocus: false,
  });
}
