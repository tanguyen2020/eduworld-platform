// Dropdown data = the WHOLE list (owner rule 2026-11-22, ADMIN_PORTAL §1.6, API_DESIGN §5). Backend lists are cursor
// paginated (limit ≤ 100); a dropdown walks every page with `limit: 100` until `page.has_more = false`, caches the result
// with react-query and filters in the browser (SearchSelect, viFilterOption). A single `limit: 100` call is never
// treated as complete. A hard safety cap stops runaway lists and the caller shows `truncated` (SearchSelect warning).
// Tables do NOT use this: they page with the cursor ("Tải thêm", useCursorList / useInfiniteQuery).
import { useQuery, type QueryKey } from '@tanstack/react-query';

/** Page size of every request made by fetchAllPages (`components/parameters/Limit` maximum). */
export const ALL_PAGES_LIMIT = 100;
/** Safety cap of one dropdown list (50 requests); past it the list is cut and flagged `truncated`. */
export const ALL_PAGES_MAX_ITEMS = 5000;

export interface PageQuery {
  limit: number;
  cursor?: string;
}

/** Any cursor page of the contract: `data` (most lists) or `items` (some public lists) + PageInfo. */
export interface CursorPageLike<T> {
  data?: T[];
  items?: T[];
  page?: { has_more?: boolean; next_cursor?: string | null } | null;
}

export interface AllPages<T> {
  items: T[];
  /** True when the safety cap stopped the walk (or the server repeated a cursor): the list is incomplete. */
  truncated: boolean;
}

/** Walks every cursor page of a list. `fetchPage` receives `{ limit: 100, cursor? }` to spread into the query. */
export async function fetchAllPages<T>(
  fetchPage: (query: PageQuery) => Promise<CursorPageLike<T>>,
  opts: { maxItems?: number } = {},
): Promise<AllPages<T>> {
  const max = opts.maxItems ?? ALL_PAGES_MAX_ITEMS;
  const items: T[] = [];
  const seen = new Set<string>();
  let cursor: string | undefined;
  for (;;) {
    const page = await fetchPage(cursor ? { limit: ALL_PAGES_LIMIT, cursor } : { limit: ALL_PAGES_LIMIT });
    items.push(...(page.data ?? page.items ?? []));
    const next = page.page?.has_more ? (page.page.next_cursor ?? undefined) : undefined;
    if (!next) return { items: items.slice(0, max), truncated: items.length > max };
    if (items.length >= max || seen.has(next)) return { items: items.slice(0, max), truncated: true };
    seen.add(next);
    cursor = next;
  }
}

/** react-query wrapper of fetchAllPages for dropdowns: `items` (whole list) + `truncated`. */
export function useAllPages<T>(
  queryKey: QueryKey,
  fetchPage: (query: PageQuery) => Promise<CursorPageLike<T>>,
  opts: { enabled?: boolean; staleTime?: number; maxItems?: number } = {},
) {
  const q = useQuery({
    queryKey: [...queryKey, 'all-pages'],
    queryFn: () => fetchAllPages(fetchPage, { maxItems: opts.maxItems }),
    enabled: opts.enabled ?? true,
    staleTime: opts.staleTime ?? 60_000,
  });
  return { ...q, items: q.data?.items ?? [], truncated: q.data?.truncated ?? false };
}

/** fetchAllPages shaped as `{ data, truncated }` for hooks whose callers read `query.data.data`. */
export async function fetchAllAsPage<T>(
  fetchPage: (query: PageQuery) => Promise<CursorPageLike<T>>,
  opts: { maxItems?: number } = {},
): Promise<{ data: T[]; truncated: boolean }> {
  const r = await fetchAllPages(fetchPage, opts);
  return { data: r.items, truncated: r.truncated };
}
