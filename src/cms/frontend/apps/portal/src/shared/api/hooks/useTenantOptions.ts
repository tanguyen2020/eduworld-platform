// useTenantOptions (F051 T045, F051-AC-22/27; owner rule 2026-11-22, ADMIN_PORTAL §1.6): `listTenants` (internal gateway,
// `platform.cross_tenant.read`) as the source of the System Admin tenant filter and target-tenant field. A dropdown loads
// the WHOLE list (fetchAllPages walks every cursor page, limit 100, cached) and filters in the browser ignoring case and
// Vietnamese diacritics (viFilterOption). `truncated` is true when the safety cap cut the list.
import { foldVi } from '@eduworld/ops-ui';
import { usePortalRuntime } from '../../../app/runtime';
import { useAllPages } from '../fetchAllPages';
import type { components } from '../schema.gen';

export type TenantSummary = components['schemas']['TenantSummary'];
type PageInfo = components['schemas']['PageInfo'];

/** `maxLength` of `listTenants` `q` (specs/openapi/eduworld-api.v1.yaml); also caps the client-side search text. */
export const TENANT_SEARCH_MAX_LENGTH = 100;

export function clampTenantSearch(q: string): string {
  return q.slice(0, TENANT_SEARCH_MAX_LENGTH);
}

/** Every tenant (all cursor pages), cached; `search` filters client-side on code and name. */
export function useTenantOptions(search: string, enabled: boolean) {
  const { api } = usePortalRuntime();
  const all = useAllPages<TenantSummary>(
    ['admin', 'tenants'],
    (query) => api.call<{ data: TenantSummary[]; page?: PageInfo }>(api.client.GET('/admin/tenants', { params: { query } })),
    { enabled, staleTime: 60_000 },
  );
  const words = foldVi(clampTenantSearch(search)).split(' ').filter(Boolean);
  const data = words.length
    ? all.items.filter((x) => {
        const text = foldVi(`${x.code} ${x.name}`);
        return words.every((w) => text.includes(w));
      })
    : all.items;
  return { ...all, data, hasNextPage: false, isFetchingNextPage: false, fetchNextPage: () => undefined };
}

/** Scroll handler of a Select popup: loads the next page when the list end is reached. */
export function loadMoreOnScroll(opts: { hasNextPage: boolean; isFetchingNextPage: boolean; fetchNextPage: () => unknown }) {
  return (e: { currentTarget: { scrollTop: number; clientHeight: number; scrollHeight: number } }) => {
    const el = e.currentTarget;
    if (opts.hasNextPage && !opts.isFetchingNextPage && el.scrollTop + el.clientHeight >= el.scrollHeight - 1) opts.fetchNextPage();
  };
}
