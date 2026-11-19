// Users of the tenant (F052 T032, T033; F052-AC-12, F052-AC-13): listTenantUsers (q, user_type, school_id, role_code),
// createTenantUser (Idempotency-Key per intent). Duplicate username/email is decided by the server (409).
import { useQueryClient } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { PAGE_SIZE, qk, useApi, usePagedList, type Schemas } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';

export type User = Schemas['User'];
export type UserCreate = Schemas['TenantUserCreateRequest'];

export interface UserFilters {
  q?: string;
  userType?: User['user_type'];
  schoolId?: string;
  roleCode?: string;
}

export function useUserList(f: UserFilters) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const q = f.q && f.q.trim().length >= 2 ? f.q.trim() : undefined;
  return usePagedList<User>(
    qk('users', scope, q ?? '', f.userType ?? '', f.schoolId ?? '', f.roleCode ?? ''),
    (cursor) =>
      api.call(
        api.client.GET('/tenant/users', {
          params: {
            query: {
              limit: PAGE_SIZE,
              ...(cursor ? { cursor } : {}),
              ...(q ? { q } : {}),
              ...(f.userType ? { user_type: f.userType } : {}),
              ...(f.schoolId ? { school_id: f.schoolId } : {}),
              ...(f.roleCode ? { role_code: f.roleCode } : {}),
            },
          },
          headers: scope.headers,
        }),
      ),
    scope.ready,
  );
}

export function useCreateStaff() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const qc = useQueryClient();
  return useIdempotentMutation<User, UserCreate>((body, intent) => api.call(api.client.POST('/tenant/users', { body, headers: { ...scope.headers, ...intent } })), {
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['school-lms', 'users', scope.key] }),
  });
}
