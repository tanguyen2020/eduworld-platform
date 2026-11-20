// Shared plumbing of the F055 hooks (T007): the hooks of `lms/api/` are the only code of the LMS screens that calls the
// network (AGENTS R1). Query keys carry the tenant of the data (`['school-lms', 'lms', <name>, scope.key, …]`), so a
// tenant switch never shows another tenant's rows; lists use `data`/`page` cursor pagination (API_DESIGN §6); writes
// carry the X-Tenant-Id of the scope and one Idempotency-Key per user intent (useIdempotentMutation, AGENTS R10).
import { useInfiniteQuery, useQueryClient, type QueryKey } from '@tanstack/react-query';
import { useApi } from '../../common/api';
import { useSchoolLmsScope, type SchoolLmsScope } from '../../common/scope';
import type { components } from '../../../../shared/api/schema.gen';

export type S = components['schemas'];
export type PageInfo = S['PageInfo'];

/** Page size of the LMS lists (≤ `Limit.maximum` = 100). */
export const LMS_PAGE_SIZE = 50;

export const lmsKey = (name: string, scope: SchoolLmsScope, ...rest: unknown[]): QueryKey => ['school-lms', 'lms', name, scope.key, ...rest];

export const MERGE_PATCH = { 'Content-Type': 'application/merge-patch+json' };

export interface DataPage<T> {
  data: T[];
  page: PageInfo;
}

/** Cursor list of an LMS operation answering `{ data, page }`. */
export function useDataList<T>(key: QueryKey, fetchPage: (cursor: string | undefined) => Promise<DataPage<T>>, enabled = true) {
  const q = useInfiniteQuery({
    queryKey: key,
    queryFn: ({ pageParam }) => fetchPage(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last: DataPage<T>) => (last.page?.has_more && last.page.next_cursor ? last.page.next_cursor : undefined),
    enabled,
  });
  return {
    items: q.data?.pages.flatMap((p) => p.data) ?? [],
    hasMore: !!q.hasNextPage,
    loadMore: () => void q.fetchNextPage(),
    isLoading: q.isPending && enabled,
    isFetching: q.isFetching,
    error: q.error,
    refetch: q.refetch,
  };
}

/** Api, scope and query client of a hook. */
export function useLmsContext() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const qc = useQueryClient();
  const invalidate = (name: string) => qc.invalidateQueries({ queryKey: ['school-lms', 'lms', name, scope.key] });
  return { api, scope, qc, invalidate };
}

/** Awaits an openapi-fetch result like `api.call` and also returns the ETag of the response (If-Match of the next write). */
export async function withEtag<T>(
  api: ReturnType<typeof useApi>,
  result: Promise<{ data?: T; error?: unknown; response: Response }>,
): Promise<{ data: T; etag: string | null }> {
  const r = await result;
  const data = await api.call<T>(Promise.resolve(r));
  return { data, etag: r.response.headers.get('ETag') };
}
