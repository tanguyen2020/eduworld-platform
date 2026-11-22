// Roles and permissions of the tenant (F052 T034, F052-AC-14, F052-AC-15): listTenantRoles, getTenantRole (its ETag
// feeds If-Match), listTenantPermissions, createTenantRole (Idempotency-Key), updateTenantRole (If-Match, reason),
// deleteTenantRole (If-Match, reason in the query — soft delete on the server, R21).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { useAllPages } from '../../../shared/api/fetchAllPages';
import { PAGE_SIZE, qk, useApi, usePagedList, type Schemas } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';

export type Role = Schemas['Role'];
export type RoleInput = Schemas['RoleInput'];

export function useRoleList(enabled = true) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return usePagedList<Role>(
    qk('roles', scope),
    (cursor) => api.call(api.client.GET('/tenant/roles', { params: { query: { limit: PAGE_SIZE, ...(cursor ? { cursor } : {}) } }, headers: scope.headers })),
    scope.ready && enabled,
  );
}

/** Roles of the tenant for dropdowns (filters, grant): every page (fetchAllPages, owner rule 2026-11-22). */
export function useRoleOptions(enabled = true) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useAllPages<Role>(
    [...qk('roles', scope), 'options'],
    (page) => api.call(api.client.GET('/tenant/roles', { params: { query: page }, headers: scope.headers })),
    { enabled: scope.ready && enabled },
  );
}

export function useRole(roleId: string | undefined) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useQuery({
    queryKey: qk('role', scope, roleId),
    queryFn: () => api.call(api.client.GET('/tenant/roles/{role_id}', { params: { path: { role_id: roleId! } }, headers: scope.headers })),
    enabled: scope.ready && !!roleId,
  });
}

export function usePermissionCatalog(locale: string, enabled = true) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useQuery({
    queryKey: qk('permissions', scope, locale),
    queryFn: async () => (await api.call(api.client.GET('/tenant/permissions', { headers: scope.headers }))).items,
    enabled: scope.ready && enabled,
    staleTime: 5 * 60_000,
  });
}

function useInvalidateRoles() {
  const scope = useSchoolLmsScope();
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: ['school-lms', 'roles', scope.key] });
    void qc.invalidateQueries({ queryKey: ['school-lms', 'role', scope.key] });
  };
}

export function useCreateRole() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const invalidate = useInvalidateRoles();
  return useIdempotentMutation<Role, RoleInput>((body, intent) => api.call(api.client.POST('/tenant/roles', { body, headers: { ...scope.headers, ...intent } })), { onSuccess: invalidate });
}

export function useUpdateRole(roleId: string) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const invalidate = useInvalidateRoles();
  return useMutation({
    mutationFn: (body: RoleInput) => api.call(api.client.PATCH('/tenant/roles/{role_id}', { params: { path: { role_id: roleId } }, body, headers: scope.headers })),
    onSuccess: invalidate,
  });
}

export function useDeleteRole(roleId: string) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const invalidate = useInvalidateRoles();
  return useMutation({
    mutationFn: (reason: string) => api.call(api.client.DELETE('/tenant/roles/{role_id}', { params: { path: { role_id: roleId }, query: { reason } }, headers: scope.headers })),
    onSuccess: invalidate,
  });
}
