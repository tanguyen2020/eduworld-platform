// Catalog hooks (F057 T032, F057-AC-11/12/13, ADR-0060): countries, provinces (`status=all` for the System Admin),
// province successions and the school catalog. Writes: createProvince/createProvinceSuccession with Idempotency-Key;
// updateCountry/updateProvince with If-Match (known only after an earlier write of the item — Câu hỏi mở F057-Q3).
// listCatalogSchools accepts only X-Tenant-Id: the System Admin picks exactly one tenant in the filter.
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useMutation } from '@tanstack/react-query';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { pageQuery, useCursorList, usePlatformApi, type Schemas } from './common';

export type Country = Schemas['Country'];
export type CountryInput = Schemas['CountryInput'];
export type Province = Schemas['Province'];
export type ProvinceInput = Schemas['ProvinceInput'];
export type ProvinceSuccessionInput = Schemas['ProvinceSuccessionInput'];
export type School = Schemas['School'];
export type SchoolLevel = Schemas['SchoolLevel'];
export type ProvinceStatusFilter = 'all' | 'active' | 'inactive';

export const SCHOOL_LEVELS: readonly SchoolLevel[] = ['primary', 'lower_secondary', 'multi_level'];
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
    mutationFn: async ({ code, input }: { code: string; input: CountryInput }) => {
      const pending = api.client.PATCH('/admin/catalog/countries/{code}', { params: { path: { code } }, body: input });
      const data = await api.call(pending);
      await remember(api, `/admin/catalog/countries/${code}`, pending);
      return data;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['platform', 'countries'] }),
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
    mutationFn: async ({ id, input }: { id: string; input: ProvinceInput }) => {
      const pending = api.client.PATCH('/admin/catalog/provinces/{province_id}', { params: { path: { province_id: id } }, body: input });
      const data = await api.call(pending);
      await remember(api, `/admin/catalog/provinces/${id}`, pending);
      return data;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['platform', 'provinces'] }),
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
  q?: string;
}

export function useCatalogSchools(filter: SchoolFilter) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const headers = scope.single.ready ? scope.single.headers : {};
  const query = { ...(filter.province ? { province: filter.province } : {}), ...(filter.school_level ? { school_level: filter.school_level } : {}), ...(filter.q ? { q: filter.q } : {}) };
  const list = useCursorList<School>(
    ['platform', 'catalog-schools', headers, query],
    async (cursor) => {
      const r = await api.call(api.client.GET('/catalog/schools', { params: { query: { ...query, ...pageQuery(cursor) } }, headers }));
      return { data: r.items, page: r.page };
    },
    scope.single.ready,
  );
  return { ...list, ready: scope.single.ready };
}
