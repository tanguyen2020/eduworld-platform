// Rows of a table or list = cursor pages (owner rule 2026-11-22, ADMIN_PORTAL §1.6, API_DESIGN §5): the first page is
// shown, "Tải thêm" (`fetchNextPage`) appends the next one while `page.has_more`. Never load the whole list for rows —
// dropdowns use fetchAllPages instead.
import { useInfiniteQuery, type QueryKey } from '@tanstack/react-query';
import type { CursorPageLike } from './fetchAllPages';

export function useCursorPages<T, P extends CursorPageLike<T> = CursorPageLike<T>>(
  queryKey: QueryKey,
  fetchPage: (cursor: string | undefined) => Promise<P>,
  opts: { enabled?: boolean } = {},
) {
  const q = useInfiniteQuery({
    queryKey,
    queryFn: ({ pageParam }) => fetchPage(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (p: P) => (p.page?.has_more ? (p.page.next_cursor ?? undefined) : undefined),
    enabled: opts.enabled ?? true,
  });
  const rows: T[] = q.data?.pages.flatMap((p) => p.data ?? p.items ?? []) ?? [];
  return { ...q, rows };
}
