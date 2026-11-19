// UserRole hooks (F059 T027, F059-AC-06…10, ADR-0066, ADR-0072, ADR-0081): listAllUserRoles (`user_id`, `role_id`,
// `tenant_ids[]`, cursor — no X-Tenant-Id), grantPlatformUserRole (Idempotency-Key; X-Tenant-Id = target tenant for a
// `tenant` role, none for a `platform` role), revokePlatformUserRole (reason; X-Tenant-Id = tenant of the UserRole).
// Account pickers for staff with a home tenant: listTenantUsers and listAdminTenantSchools of the target tenant.
// The backend validates scope/role/tenant (422/409) — the dialog only narrows what it offers (AGENTS R1, R20).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { LIMITS } from '../limits';
import { useCursorList, usePlatformApi, type CursorPage, type PageInfo, type Schemas } from './common';

export type UserRole = Schemas['UserRole'];
export type UserRoleGrantRequest = Schemas['UserRoleGrantRequest'];
export type UserRoleScopeType = Schemas['UserRoleScopeType'];
export type User = Schemas['User'];

export interface UserRoleFilter {
  userId?: string;
  roleId?: string;
}

const toCursorPage = (p: { items: UserRole[]; page: PageInfo }): CursorPage<UserRole> => ({ data: p.items, page: p.page });

/** UserRoles grouped by tenant (null = platform first). */
export function groupByTenant(rows: readonly UserRole[]): [string | null, UserRole[]][] {
  const out = new Map<string | null, UserRole[]>();
  for (const r of rows) out.set(r.tenant_id ?? null, [...(out.get(r.tenant_id ?? null) ?? []), r]);
  return [...out.entries()].sort(([a], [b]) => (a === null ? -1 : b === null ? 1 : a.localeCompare(b)));
}

export function useUserRoleList(f: UserRoleFilter, enabled = true) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const query = scope.listQuery({ ...(f.userId ? { user_id: f.userId } : {}), ...(f.roleId ? { role_id: f.roleId } : {}) });
  return useCursorList<UserRole>(
    ['platform', 'user-roles', query],
    async (cursor) =>
      toCursorPage(
        await api.call(api.client.GET('/admin/user-roles', { params: { query: { ...query, limit: LIMITS.pageSize.maximum, ...(cursor ? { cursor } : {}) } } })),
      ),
    enabled,
  );
}

/** Every UserRole of one account across tenants (detail drawer, F059-AC-14): first page, all tenants. */
export function useUserRolesOf(userId: string | undefined) {
  const api = usePlatformApi();
  return useQuery({
    queryKey: ['platform', 'user-roles-of', userId],
    queryFn: async () => (await api.call(api.client.GET('/admin/user-roles', { params: { query: { user_id: userId!, limit: LIMITS.pageSize.maximum } } }))).items,
    enabled: !!userId,
  });
}

function useInvalidateUserRoles() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: ['platform', 'user-roles'] });
    void qc.invalidateQueries({ queryKey: ['platform', 'user-roles-of'] });
    void qc.invalidateQueries({ queryKey: ['platform', 'roles'] });
  };
}

/** `tenantId` = target tenant of a `tenant` role (X-Tenant-Id); undefined for a `platform` role (no header). */
export function useGrantUserRole() {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const invalidate = useInvalidateUserRoles();
  return useIdempotentMutation(
    ({ body, tenantId }: { body: UserRoleGrantRequest; tenantId?: string }, headers) =>
      api.call<UserRole>(api.client.POST('/admin/user-roles', { body, headers: { ...headers, ...(tenantId ? scope.writeHeaders(tenantId) : {}) } })),
    { onSuccess: invalidate },
  );
}

export function useRevokeUserRole() {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const invalidate = useInvalidateUserRoles();
  return useMutation({
    mutationFn: ({ userRole, reason }: { userRole: Pick<UserRole, 'user_role_id' | 'tenant_id'>; reason: string }) =>
      api.call(
        api.client.DELETE('/admin/user-roles/{user_role_id}', {
          params: { path: { user_role_id: userRole.user_role_id }, query: { reason } },
          headers: userRole.tenant_id ? scope.writeHeaders(userRole.tenant_id) : {},
        }),
      ),
    onSuccess: invalidate,
  });
}

/** Accounts of one tenant (`listTenantUsers`, X-Tenant-Id = that tenant) of one type, searched by `q` (2–100). */
export function useTenantStaff(tenantId: string | undefined, q: string, userType: 'staff' | 'student' = 'staff') {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const search = q.trim().length >= (LIMITS.tenantUserSearch.minLength ?? 0) ? q.trim() : '';
  return useQuery({
    queryKey: ['platform', 'tenant-staff', tenantId, userType, search],
    queryFn: async () =>
      (
        await api.call(
          api.client.GET('/tenant/users', {
            params: { query: { user_type: userType, limit: LIMITS.pageSize.maximum, ...(search ? { q: search } : {}) } },
            headers: scope.writeHeaders(tenantId!),
          }),
        )
      ).items,
    enabled: !!tenantId,
  });
}

/** Schools of the target tenant (scope `school`). */
export function useTenantSchools(tenantId: string | undefined) {
  const api = usePlatformApi();
  return useQuery({
    queryKey: ['platform', 'tenant-schools', tenantId],
    queryFn: async () =>
      (await api.call(api.client.GET('/admin/tenants/{tenant_id}/schools', { params: { path: { tenant_id: tenantId! }, query: { limit: LIMITS.pageSize.maximum } } }))).items,
    enabled: !!tenantId,
  });
}
