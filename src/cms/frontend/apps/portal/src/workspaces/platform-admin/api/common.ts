// Shared plumbing of the workspace D API hooks (F057, plan "application" layer): typed apiClient of F050, cursor
// pages, job polling. Hooks only call the API and shape requests — every rule is enforced by the backend (AGENTS R1, R7).
import { useInfiniteQuery, type QueryKey } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import type { components } from '../../../shared/api/schema.gen';
import { PAGE_SIZE } from '../limits';

export type Schemas = components['schemas'];
export type ResourceType = Schemas['ResourceType'];
export type PageInfo = Schemas['PageInfo'];

export interface CursorPage<T> {
  data: T[];
  page: PageInfo;
}

export function usePlatformApi() {
  return usePortalRuntime().api;
}

/** Cursor list (`limit` + `cursor`, PageInfo.next_cursor): "Tải thêm" loads the next page, never the whole list. */
export function useCursorList<T>(queryKey: QueryKey, fetchPage: (cursor: string | undefined) => Promise<CursorPage<T>>, enabled = true) {
  const q = useInfiniteQuery({
    queryKey,
    queryFn: ({ pageParam }) => fetchPage(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last: CursorPage<T>) => (last.page.has_more && last.page.next_cursor ? last.page.next_cursor : undefined),
    enabled,
  });
  const rows = q.data?.pages.flatMap((p) => p.data) ?? [];
  return { ...q, rows };
}

/** `limit` + optional `cursor` query parameters. */
export const pageQuery = (cursor: string | undefined) => (cursor ? { limit: PAGE_SIZE, cursor } : { limit: PAGE_SIZE });

/** Final statuses of the asynchronous jobs of workspace D (clone, school import): polling stops there. */
export const FINAL_JOB_STATUSES: readonly string[] = ['succeeded', 'partially_failed', 'failed', 'cancelled'];

export const isFinalJob = (status: string | undefined): boolean => !!status && FINAL_JOB_STATUSES.includes(status);
