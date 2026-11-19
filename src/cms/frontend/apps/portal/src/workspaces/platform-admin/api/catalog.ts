// Catalog hooks (F057 T032, F057-AC-11/12/13, ADR-0060): countries, provinces (`status=all` for the System Admin),
// province successions and the school catalog. Writes: createProvince/createProvinceSuccession with Idempotency-Key;
// updateCountry/updateProvince with If-Match = `version` of the list row (OQ F057-3 resolved 2026-11-20).
// listCatalogSchools (OQ F057-4): the System Admin reads one or more tenants with `tenant_ids[]` (CrossTenantScope, no
// X-Tenant-Id; the backend refuses a read without tenant_ids) and filters by `status`; a tenant user stays in the
// tenant of the session.
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useMutation } from '@tanstack/react-query';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { pageQuery, reloadOnStaleVersion, useCursorList, usePlatformApi, versionIfMatch, type Schemas } from './common';

export type Country = Schemas['Country'];
export type CountryInput = Schemas['CountryInput'];
export type Province = Schemas['Province'];
export type ProvinceInput = Schemas['ProvinceInput'];
export type ProvinceSuccessionInput = Schemas['ProvinceSuccessionInput'];
export type School = Schemas['School'];
export type SchoolLevel = Schemas['SchoolLevel'];
export type SchoolStatus = School['status'];
export type ProvinceStatusFilter = 'all' | 'active' | 'inactive';

export const SCHOOL_LEVELS: readonly SchoolLevel[] = ['primary', 'lower_secondary', 'multi_level'];
/** `status` filter of listCatalogSchools (OQ F057-4). */
export const SCHOOL_STATUSES: readonly SchoolStatus[] = ['pending_review', 'active', 'disabled'];
export const SUCCESSION_KINDS: readonly ProvinceSuccessionInput['kind'][] = ['merged', 'unchanged', 'renamed', 'split'];

const remember = async (api: ReturnType<typeof usePlatformApi>, path: string, pending: Promise<{ response: Response }>) =>
  api.etags.remember(path, (await pending).response.headers.get('ETag'));

export function useCountries() {
  const api = usePlatformApi();
  return useQuery({ queryKey: ['platform', 'countries'], queryFn: () => api.call(api.client.GET('/catalog/countries', {})) });
}

export function useUpdateCountry() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ code, version, input }: { code: string; version?: number; input: CountryInput }) => {
      const pending = api.client.PATCH('/admin/catalog/countries/{code}', { params: { path: { code } }, body: input, headers: versionIfMatch(version) });
      const data = await api.call(pending);
      await remember(api, `/admin/catalog/countries/${code}`, pending);
      return data;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['platform', 'countries'] }),
    onError: reloadOnStaleVersion(qc, ['platform', 'countries']),
  });
}

export function useProvinces(status: ProvinceStatusFilter) {
  const api = usePlatformApi();
  return useQuery({
    queryKey: ['platform', 'provinces', status],
    queryFn: () => api.call(api.client.GET('/catalog/provinces', { params: { query: { status } } })),
  });
}

export function useCreateProvince() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    async (input: ProvinceInput, headers) => {
      const pending = api.client.POST('/admin/catalog/provinces', { body: input, headers });
      const data = await api.call(pending);
      await remember(api, `/admin/catalog/provinces/${data.province_id}`, pending);
      return data;
    },
    { onSuccess: () => qc.invalidateQueries({ queryKey: ['platform', 'provinces'] }) },
  );
}

export function useUpdateProvince() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, version, input }: { id: string; version?: number; input: ProvinceInput }) => {
      const pending = api.client.PATCH('/admin/catalog/provinces/{province_id}', { params: { path: { province_id: id } }, body: input, headers: versionIfMatch(version) });
      const data = await api.call(pending);
      await remember(api, `/admin/catalog/provinces/${id}`, pending);
      return data;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['platform', 'provinces'] }),
    onError: reloadOnStaleVersion(qc, ['platform', 'provinces']),
  });
}

export function useCreateSuccession() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (input: ProvinceSuccessionInput, headers) => api.call(api.client.POST('/admin/catalog/province-successions', { body: input, headers })),
    { onSuccess: () => qc.invalidateQueries({ queryKey: ['platform', 'provinces'] }) },
  );
}

export interface SchoolFilter {
  province?: string;
  school_level?: SchoolLevel;
  status?: SchoolStatus;
  q?: string;
}

/**
 * listCatalogSchools: `ready` = the System Admin selected at least one tenant (a cross-tenant read needs `tenant_ids`);
 * a tenant user is always ready (session X-Tenant-Id).
 */
export function useCatalogSchools(filter: SchoolFilter) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const ready = !scope.crossTenant || scope.filter.length > 0;
  const query = scope.listQuery({
    ...(filter.province ? { province: filter.province } : {}),
    ...(filter.school_level ? { school_level: filter.school_level } : {}),
    ...(filter.status ? { status: filter.status } : {}),
    ...(filter.q ? { q: filter.q } : {}),
  });
  const list = useCursorList<School>(
    ['platform', 'catalog-schools', query],
    async (cursor) => {
      const r = await api.call(api.client.GET('/catalog/schools', { params: { query: { ...query, ...pageQuery(cursor) } } }));
      return { data: r.items, page: r.page };
    },
    ready,
  );
  return { ...list, ready };
}
