// Account hooks (F059 T033, F059-AC-11…14, ADR-0069, ADR-0081, FR-AUTH-010; OQ F059-1 resolved 2026-11-20):
// - listPlatformUsers (`q` 2–100, `user_type`, `status`, cursor; no X-Tenant-Id): without `tenant_ids` the backend
//   lists the platform staff only, with the tenant filter (`tenant_ids[]`) the members of those tenants, each row
//   carrying `tenant_ids` (its memberships within the filter — used by the grant dialog, OQ F059-5);
// - createPlatformUser (staff without home tenant, ≥ 1 initial platform role, mandatory reason, Idempotency-Key) and
//   createTenantUser (school staff in the target tenant, X-Tenant-Id = target); 409 USERNAME_TAKEN / EMAIL_TAKEN;
// - lockUser / unlockUser / deactivateUser (reason, Idempotency-Key, X-Tenant-Id = home tenant when known).
import { useQueryClient } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { pageQuery, useCursorList, usePlatformApi, type CursorPage, type PageInfo, type Schemas } from './common';

export type User = Schemas['User'];
export type UserStatus = User['status'];
export type UserType = User['user_type'];
export type TenantUserCreateRequest = Schemas['TenantUserCreateRequest'];
export type PlatformUserCreateRequest = Schemas['PlatformUserCreateRequest'];

export const USER_TYPES: readonly UserType[] = ['staff', 'student'];
export const USER_STATUSES: readonly UserStatus[] = ['pending', 'active', 'locked', 'deactivated'];

export interface PlatformUserFilter {
  q?: string;
  userType?: UserType;
  status?: UserStatus;
}

const toCursorPage = (p: { items: User[]; page: PageInfo }): CursorPage<User> => ({ data: p.items, page: p.page });
const invalidateAccounts = (qc: ReturnType<typeof useQueryClient>) => {
  void qc.invalidateQueries({ queryKey: ['platform', 'platform-users'] });
  void qc.invalidateQueries({ queryKey: ['platform', 'tenant-staff'] });
};

/** listPlatformUsers: platform staff (no tenant selected) or members of the tenants of the tenant filter. */
export function usePlatformUsers(f: PlatformUserFilter) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const query = scope.listQuery({ ...(f.q ? { q: f.q } : {}), ...(f.userType ? { user_type: f.userType } : {}), ...(f.status ? { status: f.status } : {}) });
  const list = useCursorList<User>(['platform', 'platform-users', query], async (cursor) =>
    toCursorPage(await api.call(api.client.GET('/admin/users', { params: { query: { ...query, ...pageQuery(cursor) } } }))),
  );
  return { ...list, platformOnly: !query.tenant_ids };
}

export function useCreatePlatformUser() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (body: PlatformUserCreateRequest, headers) => api.call<User>(api.client.POST('/admin/users', { body, headers })),
    { onSuccess: () => invalidateAccounts(qc) },
  );
}

export type AccountAction = 'lock' | 'unlock' | 'deactivate';
export const ACCOUNT_ACTIONS: readonly AccountAction[] = ['lock', 'unlock', 'deactivate'];

export function useCreateTenantUser() {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ tenantId, body }: { tenantId: string; body: TenantUserCreateRequest }, headers) =>
      api.call<User>(api.client.POST('/tenant/users', { body, headers: { ...headers, ...scope.writeHeaders(tenantId) } })),
    { onSuccess: () => invalidateAccounts(qc) },
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
    { onSuccess: () => invalidateAccounts(qc) },
  );
}
