// Owner rule 2026-11-22 (ADMIN_PORTAL §1.6, API_DESIGN §5): dropdown data walks every cursor page (limit 100) until
// has_more = false; a safety cap stops runaway lists and flags `truncated`.
import { describe, expect, it } from 'vitest';
import { ALL_PAGES_LIMIT, fetchAllAsPage, fetchAllPages, type PageQuery } from './fetchAllPages';

const pages = (n: number, size: number) => (q: PageQuery) => {
  const i = q.cursor ? Number(q.cursor) : 0;
  const data = Array.from({ length: size }, (_, k) => i * size + k);
  return Promise.resolve({ data, page: { has_more: i + 1 < n, next_cursor: i + 1 < n ? String(i + 1) : null } });
};

describe('fetchAllPages', () => {
  it('walks every page with limit 100 and stops on has_more = false', async () => {
    const seen: PageQuery[] = [];
    const r = await fetchAllPages((q) => {
      seen.push(q);
      return pages(3, 2)(q);
    });
    expect(r).toEqual({ items: [0, 1, 2, 3, 4, 5], truncated: false });
    expect(seen.map((q) => q.limit)).toEqual([ALL_PAGES_LIMIT, ALL_PAGES_LIMIT, ALL_PAGES_LIMIT]);
    expect(seen.map((q) => q.cursor)).toEqual([undefined, '1', '2']);
  });
  it('never treats a first page with has_more as complete', async () => {
    const r = await fetchAllPages(pages(2, 100));
    expect(r.items).toHaveLength(200);
  });
  it('cuts at the safety cap and flags truncated', async () => {
    const r = await fetchAllPages(pages(10, 3), { maxItems: 5 });
    expect(r).toEqual({ items: [0, 1, 2, 3, 4], truncated: true });
  });
  it('stops on a repeated cursor', async () => {
    const r = await fetchAllPages(() => Promise.resolve({ data: [1], page: { has_more: true, next_cursor: 'same' } }));
    expect(r.truncated).toBe(true);
  });
  it('reads `items` lists and shapes { data, truncated }', async () => {
    expect(await fetchAllAsPage(() => Promise.resolve({ items: ['x'], page: { has_more: false } }))).toEqual({ data: ['x'], truncated: false });
  });
});
