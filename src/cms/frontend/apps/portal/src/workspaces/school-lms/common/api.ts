// Shared plumbing of the workspace C hooks (data-model F052): query keys `['school-lms', <name>, scopeKey, …]`, cursor
// pagination (`limit`/`cursor`, API_DESIGN §6) and the typed apiClient of the portal runtime (F050). No business rule.
import { useInfiniteQuery, type QueryKey } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import type { components } from '../../../shared/api/schema.gen';
import type { SchoolLmsScope } from './scope';

export type Schemas = components['schemas'];
export type PageInfo = Schemas['PageInfo'];

/** Page size of the lists of F052 (≤ `Limit.maximum` = 100). */
export const PAGE_SIZE = 50;

export const qk = (name: string, scope: SchoolLmsScope, ...rest: unknown[]): QueryKey => ['school-lms', name, scope.key, ...rest];

/** Content type of JSON Merge Patch operations (updateTenantSettings, updateSchoolSettings). */
export const MERGE_PATCH = { 'Content-Type': 'application/merge-patch+json' };

export function useApi() {
  return usePortalRuntime().api;
}

export interface Paged<T> {
  items: T[];
  page: PageInfo;
}

/** Cursor-paginated list: accumulated items, `loadMore` while `page.has_more`. */
export function usePagedList<T>(key: QueryKey, fetchPage: (cursor: string | undefined) => Promise<Paged<T>>, enabled = true) {
  const q = useInfiniteQuery({
    queryKey: key,
    queryFn: ({ pageParam }) => fetchPage(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last: Paged<T>) => (last.page.has_more && last.page.next_cursor ? last.page.next_cursor : undefined),
    enabled,
  });
  return {
    items: q.data?.pages.flatMap((p) => p.items) ?? [],
    hasMore: !!q.hasNextPage,
    loadMore: () => void q.fetchNextPage(),
    isLoading: q.isPending && enabled,
    isFetching: q.isFetching,
    error: q.error,
    refetch: q.refetch,
  };
}
