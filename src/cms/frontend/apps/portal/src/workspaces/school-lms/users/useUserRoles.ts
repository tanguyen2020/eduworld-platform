// UserRole of a staff member (F052 T035, F052-AC-16, F052-AC-17): list (tenant or school route), grant (Tenant Admin:
// grantTenantUserRole; School Admin: grantSchoolUserRole — Idempotency-Key per intent), revoke (reason in the query).
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { PAGE_SIZE, qk, useApi, usePagedList, type Schemas } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';
import type { GrantMode } from './canGrant';

export type UserRole = Schemas['UserRole'];
export type GrantRequest = Schemas['UserRoleGrantRequest'];

export function useUserRoleList(userId: string | undefined, mode: GrantMode) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const schoolId = mode.kind === 'school' ? mode.schoolIds[0] : undefined;
  return usePagedList<UserRole>(
    qk('user-roles', scope, userId, schoolId ?? 'tenant'),
    (cursor) => {
      const query = { limit: PAGE_SIZE, ...(cursor ? { cursor } : {}), ...(userId ? { user_id: userId } : {}) };
      return schoolId
        ? api.call(api.client.GET('/schools/{school_id}/user-roles', { params: { path: { school_id: schoolId }, query }, headers: scope.headers }))
        : api.call(api.client.GET('/tenant/user-roles', { params: { query }, headers: scope.headers }));
    },
    scope.ready && !!userId && (mode.kind === 'tenant' || !!schoolId),
  );
}

function useInvalidate() {
  const scope = useSchoolLmsScope();
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: ['school-lms', 'user-roles', scope.key] });
    void qc.invalidateQueries({ queryKey: ['school-lms', 'users', scope.key] });
  };
}

export function useGrantUserRole(mode: GrantMode) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const onSuccess = useInvalidate();
  return useIdempotentMutation<UserRole, { body: GrantRequest; schoolId?: string }>(
    (v, intent) => {
      const headers = { ...scope.headers, ...intent };
      return mode.kind === 'school' && v.schoolId
        ? api.call(api.client.POST('/schools/{school_id}/user-roles', { params: { path: { school_id: v.schoolId } }, body: v.body, headers }))
        : api.call(api.client.POST('/tenant/user-roles', { body: v.body, headers }));
    },
    { onSuccess },
  );
}

export function useRevokeUserRole() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const onSuccess = useInvalidate();
  return useMutation({
    mutationFn: (v: { userRoleId: string; reason: string }) =>
      api.call(api.client.DELETE('/tenant/user-roles/{user_role_id}', { params: { path: { user_role_id: v.userRoleId }, query: { reason: v.reason } }, headers: scope.headers })),
    onSuccess,
  });
}
