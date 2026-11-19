// useTenantOptions (F051 T045, F051-AC-22/27; F057 edge case "tải tiếp theo cursor"): `listTenants` (internal gateway,
// `platform.cross_tenant.read`) as the source of the System Admin tenant filter and target-tenant field. Search by
// code/name, debounced; the search text is cut at the `maxLength` of the `q` parameter in OpenAPI (100). Pages follow
// the cursor (`fetchNextPage` when the dropdown is scrolled to its end) — the whole list is never loaded at once.
import { useInfiniteQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { usePortalRuntime } from '../../../app/runtime';
import type { components } from '../schema.gen';

export type TenantSummary = components['schemas']['TenantSummary'];
type PageInfo = components['schemas']['PageInfo'];

/** `maxLength` of `listTenants` `q` (specs/openapi/eduworld-api.v1.yaml). */
export const TENANT_SEARCH_MAX_LENGTH = 100;
export const TENANT_SEARCH_DEBOUNCE_MS = 300;

export function clampTenantSearch(q: string): string {
  return q.slice(0, TENANT_SEARCH_MAX_LENGTH);
}

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const h = setTimeout(() => setV(value), ms);
    return () => clearTimeout(h);
  }, [value, ms]);
  return v;
}

export function useTenantOptions(search: string, enabled: boolean) {
  const { api } = usePortalRuntime();
  const q = useDebounced(clampTenantSearch(search.trim()), TENANT_SEARCH_DEBOUNCE_MS);
  const query = useInfiniteQuery({
    queryKey: ['admin', 'tenants', q],
    queryFn: ({ pageParam }) => {
      const params = { ...(q ? { q } : {}), ...(pageParam ? { cursor: pageParam } : {}) };
      return api.call<{ data: TenantSummary[]; page?: PageInfo }>(api.client.GET('/admin/tenants', { params: { query: params } }));
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => (last.page?.has_more && last.page.next_cursor ? last.page.next_cursor : undefined),
    enabled,
    staleTime: 30_000,
  });
  return { ...query, data: query.data?.pages.flatMap((p) => p.data) };
}

/** Scroll handler of a Select popup: loads the next page when the list end is reached. */
export function loadMoreOnScroll(opts: { hasNextPage: boolean; isFetchingNextPage: boolean; fetchNextPage: () => unknown }) {
  return (e: { currentTarget: { scrollTop: number; clientHeight: number; scrollHeight: number } }) => {
    const el = e.currentTarget;
    if (opts.hasNextPage && !opts.isFetchingNextPage && el.scrollTop + el.clientHeight >= el.scrollHeight - 1) opts.fetchNextPage();
  };
}
