// Tenant and school settings (F052 T041, T042; F052-AC-19, F052-AC-20, F052-AC-22, F052-AC-29, F052-AC-30):
// getTenantSettings / updateTenantSettings and getSchoolSettings / updateSchoolSettings — JSON Merge Patch with
// If-Match (ETag of the last GET, remembered by the apiClient). The server owns every limit (cleanup override,
// retention minimum, "curfew only stricter") and answers 403 / 412 / 422.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { MERGE_PATCH, qk, useApi, type Schemas } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';

export type TenantSettings = Schemas['TenantSettings'];
export type SchoolSettings = Schemas['SchoolSettings'];
export type AllowedHours = Schemas['AllowedHours'];

export function useTenantSettings() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useQuery({
    queryKey: qk('tenant-settings', scope),
    queryFn: () => api.call(api.client.GET('/tenant/settings', { headers: scope.headers })),
    enabled: scope.ready,
  });
}

export function useUpdateTenantSettings() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (patch: Partial<TenantSettings>) => api.call(api.client.PATCH('/tenant/settings', { body: patch, headers: { ...scope.headers, ...MERGE_PATCH } })),
    onSuccess: (data) => qc.setQueryData(qk('tenant-settings', scope), data),
  });
}

export function useSchoolSettings(schoolId: string | undefined) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useQuery({
    queryKey: qk('school-settings', scope, schoolId),
    queryFn: () => api.call(api.client.GET('/schools/{school_id}/settings', { params: { path: { school_id: schoolId! } }, headers: scope.headers })),
    enabled: scope.ready && !!schoolId,
  });
}

export function useUpdateSchoolSettings(schoolId: string) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (patch: Partial<SchoolSettings>) =>
      api.call(api.client.PATCH('/schools/{school_id}/settings', { params: { path: { school_id: schoolId } }, body: patch as SchoolSettings, headers: { ...scope.headers, ...MERGE_PATCH } })),
    onSuccess: (data) => qc.setQueryData(qk('school-settings', scope, schoolId), data),
  });
}
