// Account hooks (F059 T033, F059-AC-11…14, ADR-0069, ADR-0081, FR-AUTH-010): createTenantUser for a school staff
// account in the target tenant (X-Tenant-Id = target, Idempotency-Key; 409 USERNAME_TAKEN / EMAIL_TAKEN), and the
// account status operations lockUser / unlockUser / deactivateUser (reason, Idempotency-Key, X-Tenant-Id = home tenant
// when known). `listPlatformUsers` and `createPlatformUser` (delta owner F012) are not in the contract yet: the
// cross-tenant account list and "Nhân sự platform" creation stay off until they exist (Câu hỏi mở F059-Q1).
import { useQueryClient } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { usePlatformApi, type Schemas } from './common';

export type User = Schemas['User'];
export type TenantUserCreateRequest = Schemas['TenantUserCreateRequest'];

/** listPlatformUsers / createPlatformUser are not in specs/openapi/eduworld-api.v1.yaml yet (F059 T004, owner F012). */
export const PLATFORM_USERS_AVAILABLE = false;

export type AccountAction = 'lock' | 'unlock' | 'deactivate';
export const ACCOUNT_ACTIONS: readonly AccountAction[] = ['lock', 'unlock', 'deactivate'];

export function useCreateTenantUser() {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ tenantId, body }: { tenantId: string; body: TenantUserCreateRequest }, headers) =>
      api.call<User>(api.client.POST('/tenant/users', { body, headers: { ...headers, ...scope.writeHeaders(tenantId) } })),
    { onSuccess: () => qc.invalidateQueries({ queryKey: ['platform', 'tenant-staff'] }) },
  );
}

/** lockUser / unlockUser / deactivateUser of one account (204). */
export function useAccountAction() {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ action, userId, homeTenantId, reason }: { action: AccountAction; userId: string; homeTenantId?: string | null; reason: string }, intent) => {
      const opts = { params: { path: { user_id: userId } }, body: { reason }, headers: { ...intent, ...(homeTenantId ? scope.writeHeaders(homeTenantId) : {}) } };
      if (action === 'lock') return api.call(api.client.POST('/admin/users/{user_id}:lock', opts));
      if (action === 'unlock') return api.call(api.client.POST('/admin/users/{user_id}:unlock', opts));
      return api.call(api.client.POST('/admin/users/{user_id}:deactivate', opts));
    },
    { onSuccess: () => qc.invalidateQueries({ queryKey: ['platform', 'tenant-staff'] }) },
  );
}
