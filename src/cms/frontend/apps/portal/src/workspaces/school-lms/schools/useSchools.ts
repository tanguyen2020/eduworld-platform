// Schools of the tenant (F052 T015, F052-AC-01..03): listTenantSchools, getTenantSchool, updateTenantSchool,
// approveTenantSchool (If-Match from the ETag the apiClient remembered on the last GET), listProvinces (active only).
// Hooks only call the API; status rules (pending_review → active, province required) are enforced by the server.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { PAGE_SIZE, qk, useApi, usePagedList, type Schemas } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';

export type School = Schemas['School'];
export type SchoolStatus = School['status'];
export type SchoolUpdate = Schemas['SchoolUpdateRequest'];
export type Province = Schemas['Province'];

export function useSchoolList(status?: SchoolStatus) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return usePagedList<School>(
    qk('schools', scope, status ?? 'all'),
    (cursor) =>
      api.call(
        api.client.GET('/tenant/schools', {
          params: { query: { limit: PAGE_SIZE, ...(cursor ? { cursor } : {}), ...(status ? { status } : {}) } },
          headers: scope.headers,
        }),
      ),
    scope.ready,
  );
}

export function useSchool(schoolId: string | undefined) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useQuery({
    queryKey: qk('school', scope, schoolId),
    queryFn: () => api.call(api.client.GET('/tenant/schools/{school_id}', { params: { path: { school_id: schoolId! } }, headers: scope.headers })),
    enabled: scope.ready && !!schoolId,
  });
}

export function useActiveProvinces(enabled = true) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useQuery({
    queryKey: qk('provinces', scope, 'active'),
    queryFn: async () => (await api.call(api.client.GET('/catalog/provinces', { params: { query: { status: 'active' } }, headers: scope.headers }))).items,
    enabled: scope.ready && enabled,
    staleTime: 10 * 60_000,
  });
}

function useSchoolWrite<V>(fn: (vars: V, headers: Record<string, string>) => Promise<School>) {
  const scope = useSchoolLmsScope();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: V) => fn(vars, scope.headers),
    onSuccess: (school) => {
      qc.setQueryData(qk('school', scope, school.school_id), school);
      void qc.invalidateQueries({ queryKey: ['school-lms', 'schools', scope.key] });
    },
  });
}

export function useUpdateSchool(schoolId: string) {
  const api = useApi();
  return useSchoolWrite<SchoolUpdate>((body, headers) =>
    api.call(api.client.PATCH('/tenant/schools/{school_id}', { params: { path: { school_id: schoolId } }, body, headers })),
  );
}

export function useApproveSchool(schoolId: string) {
  const api = useApi();
  return useSchoolWrite<void>((_v, headers) => api.call(api.client.POST('/tenant/schools/{school_id}:approve', { params: { path: { school_id: schoolId } }, headers })));
}
