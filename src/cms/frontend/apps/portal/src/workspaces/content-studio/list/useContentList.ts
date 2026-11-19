// useContentList (F054-AC-24, AC-25, FR-CNT-021): the list of one content type — searchQuestions / listCmsQuestionSets
// / listCmsLessons / listCmsAssessmentBlueprints — with the filters mapped to query parameters (`q` searched without
// diacritics by the server), `tenant_ids[]` only from the System Admin filter, cursor pages; and the soft delete of an
// item (deleteQuestion / deleteCms*: If-Match = ETag of a fresh read, reason in the body). Lists never offer "show
// deleted" (ADR-0028): deleted items live in the Thùng rác.
import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import { readItem, writeItem } from '../../../shared/storage';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { withEtag } from '../questions/useQuestion';
import { idParamOf } from '../release/useRelease';
import { CONTENT_TYPES } from '../shared/contentTypes';

export type ListType = 'question' | 'question_set' | 'lesson' | 'assessment_blueprint';

export interface ContentFiltersValue {
  q?: string;
  subject_id?: string;
  grade_level?: number;
  topic_id?: string;
  objective_id?: string;
  status?: string;
  /** Questions only. */
  type?: string;
  difficulty?: number[];
  source?: 'own' | 'assigned' | 'platform';
}

/** One row of any list (authoring DTO, fields read by the list). */
export interface ContentRow {
  content_id: string;
  version: number;
  status: string;
  tenant_id?: string | null;
  owner_scope?: 'platform' | 'tenant';
  assigned?: boolean;
  title?: { vi?: string; en?: string };
  type?: string;
  updated_at?: string;
  published_at?: string;
  content_language?: 'vi' | 'en';
}

const QUESTION_ONLY: (keyof ContentFiltersValue)[] = ['type', 'difficulty', 'source'];

/** F054-AC-24: filters → query (empty values dropped; question-only filters ignored for other types). */
export function filtersToQuery(type: ListType, f: ContentFiltersValue): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(f) as [keyof ContentFiltersValue, unknown][]) {
    if (v === undefined || v === null || v === '' || (Array.isArray(v) && v.length === 0)) continue;
    if (type !== 'question' && QUESTION_ONLY.includes(k)) continue;
    out[k] = k === 'q' ? String(v).trim() : v;
  }
  if (out.q === '') delete out.q;
  return out;
}

const listPath = (type: ListType) => CONTENT_TYPES[type].base!;

type Page = { data: ContentRow[]; page?: { has_more?: boolean; next_cursor?: string | null } };

export function useContentList(type: ListType, filters: ContentFiltersValue) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  const scope = useCrossTenantScope();
  const query = scope.listQuery({ ...filtersToQuery(type, filters), limit: 50 });
  return useInfiniteQuery({
    queryKey: ['content-studio', 'list', scopeKey, type, query],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => api.call<Page>(api.client.GET(listPath(type) as never, { params: { query: { ...query, ...(pageParam ? { cursor: pageParam } : {}) } } } as never)),
    getNextPageParam: (p) => (p.page?.has_more ? (p.page.next_cursor ?? undefined) : undefined),
  });
}

export function useDeleteContent(type: ListType) {
  const { api } = usePortalRuntime();
  const qc = useQueryClient();
  const scope = useCrossTenantScope();
  return async (row: ContentRow, reason: string) => {
    const path = `${listPath(type)}/{${idParamOf(type)}}`;
    const params = { path: { [idParamOf(type)]: row.content_id } };
    // System Admin: the write goes to the item's tenant (none for platform data)
    const headers = scope.crossTenant && row.tenant_id ? scope.writeHeaders(row.tenant_id) : {};
    const { etag } = await withEtag(api, api.client.GET(path as never, { params, headers } as never));
    await api.call(api.client.DELETE(path as never, { params, body: reason ? { reason } : {}, headers: { ...headers, ...(etag ? { 'If-Match': etag } : {}) } } as never));
    await qc.invalidateQueries({ queryKey: ['content-studio'] });
  };
}

/** Per-viewer remembered filters (`ew.portal.<userSub>.content_studio.filters.<screen>`, convenience only). */
export function filtersStorageKey(userKey: string | null, screen: string): string | null {
  return userKey ? `ew.portal.${userKey}.content_studio.filters.${screen}` : null;
}

export function loadFilters(key: string | null): ContentFiltersValue {
  if (!key) return {};
  try {
    const v = JSON.parse(readItem('local', key) ?? '{}') as unknown;
    return v && typeof v === 'object' ? (v as ContentFiltersValue) : {};
  } catch {
    return {};
  }
}

export function saveFilters(key: string | null, f: ContentFiltersValue): void {
  if (key) writeItem('local', key, JSON.stringify(f));
}
