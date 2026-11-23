// Permission catalog + platform role hooks (F059 T018, F059-AC-01…05, ADMIN_PORTAL §2.2/§3.17, ADR-0066):
// listAllPermissions (read-only catalog with `catalog_version`), listPlatformRoles (`role_scope_kind`, `tenant_ids[]`
// of the tenant filter, cursor — no X-Tenant-Id), getPlatformRole (ETag kept by the apiClient for If-Match),
// createPlatformRole (Idempotency-Key), updatePlatformRole (If-Match, reason), deletePlatformRole (If-Match from the
// row version, reason). Shared roles (`tenant_id = null`) are written without X-Tenant-Id; custom roles of a tenant are
// read only here (edited in workspace C, F052). Hooks only shape requests — the backend authorizes.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ApiError } from '../../../shared/api/middleware/problem';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { fetchAllPages, useAllPages } from '../../../shared/api/fetchAllPages';
import { useCursorPages } from '../../../shared/api/useCursorPages';
import { LIMITS } from '../limits';
import { pageQuery, useCursorList, usePlatformApi, type CursorPage, type PageInfo, type Schemas } from './common';

export type Role = Schemas['Role'];
export type RoleInput = Schemas['RoleInput'];
export type PermissionList = Schemas['PermissionList'];
export type PermissionPage = Schemas['PermissionPage'];
export type Permission = Schemas['Permission'];
export type PermissionScopeKind = Schemas['PermissionScopeKind'];

export const SCOPE_KINDS: readonly PermissionScopeKind[] = ['platform', 'tenant'];

const rolePath = (id: string) => `/admin/roles/${id}`;
const weakEtag = (version: number) => `W/"${version}"`;

/** RolePage uses `items`; the cursor list helper reads `data`. */
const toCursorPage = (p: { items: Role[]; page: PageInfo }): CursorPage<Role> => ({ data: p.items, page: p.page });

const SCOPE_ERROR = 'PERMISSION_NOT_ALLOWED_FOR_SCOPE';

/** Codes rejected with PERMISSION_NOT_ALLOWED_FOR_SCOPE (`errors[].field = permission_codes[i]`). */
export function rejectedPermissions(error: unknown, sent: readonly string[]): string[] {
  if (!(error instanceof ApiError)) return [];
  return Object.entries(error.fieldErrors)
    .filter(([field, fe]) => fe.code === SCOPE_ERROR && field.startsWith('permission_codes'))
    .map(([field]) => sent[Number(/\[(\d+)\]|\.(\d+)/.exec(field)?.slice(1).find(Boolean) ?? -1)])
    .filter((x): x is string => !!x);
}

/**
 * The whole permission catalog for the role editor's picker: every cursor page of listAllPermissions (fetchAllPages,
 * ADMIN_PORTAL §1.6); `catalog_version` from the first page, `truncated` when the safety cap cut the list.
 */
export function usePermissionCatalog() {
  const api = usePlatformApi();
  return useQuery({
    queryKey: ['platform', 'permission-catalog', 'all'],
    queryFn: async () => {
      let version = '';
      const all = await fetchAllPages<Permission>(async (query) => {
        const p = await api.call<PermissionPage>(api.client.GET('/admin/permissions', { params: { query } }));
        version ||= p.catalog_version;
        return p;
      });
      return { items: all.items, catalog_version: version, truncated: all.truncated };
    },
    staleTime: 60_000,
  });
}

/** Read-only catalog grid: cursor pages of listAllPermissions (keyset in the query, owner rule 2026-11-23 — "Tải thêm"). */
export function usePermissionCatalogPages() {
  const api = usePlatformApi();
  return useCursorPages<Permission, PermissionPage>(['platform', 'permission-catalog', 'pages'], (cursor) =>
    api.call<PermissionPage>(api.client.GET('/admin/permissions', { params: { query: pageQuery(cursor) } })),
  );
}

export function useRoleList(roleScopeKind: PermissionScopeKind | undefined) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const query = scope.listQuery(roleScopeKind ? { role_scope_kind: roleScopeKind } : {});
  return useCursorList<Role>(['platform', 'roles', query], async (cursor) =>
    toCursorPage(
      await api.call(api.client.GET('/admin/roles', { params: { query: { ...query, limit: LIMITS.pageSize.maximum, ...(cursor ? { cursor } : {}) } } })),
    ),
  );
}

/**
 * First page (≤ Limit.maximum) of the roles of one kind, optionally for one tenant (shared + its custom roles): role
 * pickers and the "roles holding this permission" column. Not the whole list on very large installations (Câu hỏi mở F059-Q6).
 */
export function useRoleOptions(roleScopeKind: PermissionScopeKind | undefined, tenantId?: string, enabled = true) {
  const api = usePlatformApi();
  const all = useAllPages<Role>(
    ['platform', 'role-options', roleScopeKind ?? null, tenantId ?? null],
    (page) =>
      api.call(
        api.client.GET('/admin/roles', {
          params: { query: { ...page, ...(roleScopeKind ? { role_scope_kind: roleScopeKind } : {}), ...(tenantId ? { tenant_ids: [tenantId] } : {}) } },
        }),
      ),
    { enabled, staleTime: 30_000 },
  );
  return { ...all, data: all.items };
}

export function useRole(roleId: string | undefined) {
  const api = usePlatformApi();
  return useQuery({
    queryKey: ['platform', 'role', roleId],
    queryFn: () => api.call<Role>(api.client.GET('/admin/roles/{role_id}', { params: { path: { role_id: roleId! } } })),
    enabled: !!roleId,
    retry: false,
    gcTime: 0,
  });
}

function useInvalidateRoles() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: ['platform', 'roles'] });
    void qc.invalidateQueries({ queryKey: ['platform', 'role-options'] });
  };
}

export function useCreateRole() {
  const api = usePlatformApi();
  const invalidate = useInvalidateRoles();
  return useIdempotentMutation((body: RoleInput, headers) => api.call<Role>(api.client.POST('/admin/roles', { body, headers })), { onSuccess: invalidate });
}

/** updatePlatformRole: If-Match = ETag of the getPlatformRole the editor loaded (kept by the apiClient). */
export function useUpdateRole() {
  const api = usePlatformApi();
  const invalidate = useInvalidateRoles();
  return useMutation({
    mutationFn: ({ roleId, body }: { roleId: string; body: RoleInput }) =>
      api.call<Role>(api.client.PATCH('/admin/roles/{role_id}', { params: { path: { role_id: roleId } }, body })),
    onSuccess: invalidate,
  });
}

/** deletePlatformRole (soft delete, AGENTS R21): If-Match = version of the row, mandatory reason. */
export function useDeleteRole() {
  const api = usePlatformApi();
  const invalidate = useInvalidateRoles();
  return useMutation({
    mutationFn: ({ role, reason }: { role: Pick<Role, 'role_id' | 'version'>; reason: string }) => {
      api.etags.remember(rolePath(role.role_id), weakEtag(role.version));
      return api.call(api.client.DELETE('/admin/roles/{role_id}', { params: { path: { role_id: role.role_id }, query: { reason } } }));
    },
    onSuccess: invalidate,
  });
}
