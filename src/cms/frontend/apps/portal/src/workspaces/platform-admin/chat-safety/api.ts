// API hooks of workspace D "An toàn chat" (F122 T021/T025, plan "application" layer): the platform sensitive-term catalogue
// (categories, terms, allowlist, test, publish, versions) and the moderation switches (state of the platform and the
// filtered tenants, change requests, approval by a second System Admin, history). Writes carry Idempotency-Key (one per
// user intent) or If-Match; a tenant-level switch change carries X-Tenant-Id of that tenant. Rules stay in module chat.
import { useQueryClient } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { TENANT_HEADER } from '../../../shared/api/middleware/tenant';
import type { components } from '../../../shared/api/schema.gen';
import { useAllPages } from '../../../shared/api/fetchAllPages';
import { useCursorPages } from '../../../shared/api/useCursorPages';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';

export type Schemas = components['schemas'];
export type Category = Schemas['SensitiveTermCategory'];
export type CategoryInput = Schemas['SensitiveTermCategoryInput'];
export type Term = Schemas['SensitiveTerm'];
export type TermInput = Schemas['SensitiveTermInput'];
export type AllowEntry = Schemas['SensitiveAllowlistEntry'];
export type CatalogVersion = Schemas['SensitiveCatalogVersion'];
export type SwitchState = Schemas['ModerationSwitchState'];
export type SwitchUpdate = Schemas['ModerationSwitchUpdate'];
export type SwitchRequest = Schemas['ModerationSwitchRequest'];
export type SwitchChange = Schemas['ModerationSwitchChange'];
export type Steps = Schemas['ModerationSwitchSteps'];

interface Page<T> {
  data: T[];
  page: Schemas['PageInfo'];
}

export const PAGE_SIZE = 50;
const pageQuery = (cursor?: string) => (cursor ? { limit: PAGE_SIZE, cursor } : { limit: PAGE_SIZE });

function useApi() {
  return usePortalRuntime().api;
}

const CATALOG = ['platform', 'chat-catalog'] as const;

/** listSensitiveCategories for the categories TABLE: cursor pages ("Tải thêm"), keyset in SQL (owner rule 2026-11-23). */
export function useCategories() {
  const api = useApi();
  return useCursorPages<Category>([...CATALOG, 'categories'], (cursor) =>
    api.call<Page<Category>>(api.client.GET('/admin/chat/sensitive-categories', { params: { query: pageQuery(cursor) } })),
  );
}

/** Every category (all cursor pages, cached) for the category dropdowns and the name lookup (fetchAllPages). */
export function useAllCategories() {
  const api = useApi();
  return useAllPages<Category>([...CATALOG, 'categories', 'all'], (query) =>
    api.call<Page<Category>>(api.client.GET('/admin/chat/sensitive-categories', { params: { query } })),
  );
}

/** Term filters applied by the server in SQL (listSensitiveTerms, F122 T063): never a filter of the loaded page. */
export interface TermFilters {
  categoryId?: string;
  locale?: 'vi' | 'en';
  matchType?: Schemas['SensitiveMatchType'];
  status?: 'draft' | 'published';
}

export function useTerms(f: TermFilters = {}) {
  const api = useApi();
  const query = {
    ...(f.categoryId ? { category_id: f.categoryId } : {}),
    ...(f.locale ? { locale: f.locale } : {}),
    ...(f.matchType ? { match_type: f.matchType } : {}),
    ...(f.status ? { status: f.status } : {}),
  };
  return useCursorPages<Term>([...CATALOG, 'terms', query], (cursor) =>
    api.call<Page<Term>>(api.client.GET('/admin/chat/sensitive-terms', { params: { query: { ...query, ...pageQuery(cursor) } } })),
  );
}

export function useAllowlist() {
  const api = useApi();
  return useCursorPages<AllowEntry>([...CATALOG, 'allowlist'], (cursor) =>
    api.call<Page<AllowEntry>>(api.client.GET('/admin/chat/sensitive-allowlist', { params: { query: pageQuery(cursor) } })),
  );
}

export function useVersions() {
  const api = useApi();
  return useCursorPages<CatalogVersion>([...CATALOG, 'versions'], (cursor) =>
    api.call<Page<CatalogVersion>>(api.client.GET('/admin/chat/sensitive-catalog/versions', { params: { query: pageQuery(cursor) } })),
  );
}

/** Every write of the catalogue; each reloads the catalogue lists. */
export function useCatalogWrites() {
  const api = useApi();
  const qc = useQueryClient();
  const reload = () => qc.invalidateQueries({ queryKey: CATALOG });
  return {
    createCategory: useIdempotentMutation((body: CategoryInput, intent) => api.call<Category>(api.client.POST('/admin/chat/sensitive-categories', { body, headers: intent })), { onSuccess: reload }),
    updateCategory: useIdempotentMutation(
      ({ id, body }: { id: string; body: CategoryInput }) => api.call<Category>(api.client.PATCH('/admin/chat/sensitive-categories/{category_id}', { params: { path: { category_id: id } }, body })),
      { onSuccess: reload },
    ),
    deleteCategory: useIdempotentMutation((id: string) => api.call(api.client.DELETE('/admin/chat/sensitive-categories/{category_id}', { params: { path: { category_id: id } } })), {
      onSuccess: reload,
    }),
    createTerm: useIdempotentMutation((body: TermInput, intent) => api.call<Term>(api.client.POST('/admin/chat/sensitive-terms', { body, headers: intent })), { onSuccess: reload }),
    deleteTerm: useIdempotentMutation((id: string) => api.call(api.client.DELETE('/admin/chat/sensitive-terms/{term_id}', { params: { path: { term_id: id } } })), { onSuccess: reload }),
    createAllow: useIdempotentMutation(
      (body: Schemas['SensitiveAllowlistInput'], intent) => api.call<AllowEntry>(api.client.POST('/admin/chat/sensitive-allowlist', { body, headers: intent })),
      { onSuccess: reload },
    ),
    deleteAllow: useIdempotentMutation((id: string) => api.call(api.client.DELETE('/admin/chat/sensitive-allowlist/{entry_id}', { params: { path: { entry_id: id } } })), {
      onSuccess: reload,
    }),
    publish: useIdempotentMutation(
      (changeSummary: string, intent) => api.call<CatalogVersion>(api.client.POST('/admin/chat/sensitive-catalog:publish', { body: { change_summary: changeSummary }, headers: intent })),
      { onSuccess: reload },
    ),
  };
}

export function useTestTerms() {
  const api = useApi();
  return useIdempotentMutation((body: Schemas['SensitiveTermTestRequest']) =>
    api.call<Schemas['SensitiveTermTestResult']>(api.client.POST('/admin/chat/sensitive-terms:test', { body })),
  );
}

// ---- moderation switches ----

const SWITCHES = ['platform', 'moderation-switches'] as const;

type SwitchPage = { platform: SwitchState; tenants: SwitchState[]; page: { has_more: boolean; next_cursor?: string | null; limit: number } };

/**
 * listModerationSwitches: the platform row + one cursor page of the tenants of the topbar filter (`tenant_ids[]`, none =
 * platform only) — the tenant rows are paged by the query (owner rule 2026-11-23, API_DESIGN §5); "Tải thêm" appends.
 */
export function useSwitches() {
  const api = useApi();
  const scope = useCrossTenantScope();
  const query = scope.filter.length ? { tenant_ids: [...scope.filter] } : {};
  const q = useCursorPages<SwitchState, SwitchPage & { data: SwitchState[] }>(
    [...SWITCHES, 'state', query],
    async (cursor) => {
      const p = await api.call<SwitchPage>(api.client.GET('/admin/chat/moderation-switches', { params: { query: { ...query, ...pageQuery(cursor) } } }));
      return { ...p, data: p.tenants };
    },
    { refetchInterval: 30_000 },
  );
  return { ...q, platform: q.data?.pages[0]?.platform };
}

export function useSwitchHistory() {
  const api = useApi();
  const scope = useCrossTenantScope();
  const query = scope.filter.length ? { tenant_ids: [...scope.filter] } : {};
  return useCursorPages<SwitchChange>([...SWITCHES, 'history', query], (cursor) =>
    api.call<Page<SwitchChange>>(api.client.GET('/admin/chat/moderation-switches/history', { params: { query: { ...query, ...pageQuery(cursor) } } })),
  );
}

/** setModerationSwitches: 200 = re-enabled at once; 202 = disabling request waiting for a second System Admin. */
export function useSetSwitches() {
  const api = useApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (body: SwitchUpdate, intent) =>
      api.call<SwitchState | SwitchRequest>(
        api.client.PUT('/admin/chat/moderation-switches', { body, headers: { ...intent, ...(body.scope === 'tenant' && body.tenant_id ? { [TENANT_HEADER]: body.tenant_id } : {}) } }),
      ),
    { onSettled: () => qc.invalidateQueries({ queryKey: SWITCHES }) },
  );
}

/**
 * rejectModerationSwitchChange (F122 T060): a second System Admin rejects a pending request with a reason; If-Match =
 * version of the request, X-Tenant-Id of a tenant request, Idempotency-Key kept for a retry of the same intent.
 */
export function useRejectSwitch() {
  const api = useApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (r: Pick<SwitchRequest, 'request_id' | 'tenant_id' | 'version'> & { reason: string }, intent) =>
      api.call<SwitchRequest>(
        api.client.POST('/admin/chat/moderation-switches/requests/{request_id}:reject', {
          params: { path: { request_id: r.request_id } },
          body: { reason: r.reason },
          headers: { ...intent, 'If-Match': `W/"${r.version ?? 1}"`, ...(r.tenant_id ? { [TENANT_HEADER]: r.tenant_id } : {}) },
        }),
      ),
    { onSuccess: () => qc.invalidateQueries({ queryKey: SWITCHES }) },
  );
}

export function useApproveSwitch() {
  const api = useApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (r: Pick<SwitchRequest, 'request_id' | 'tenant_id'>, intent) =>
      api.call<SwitchState>(
        api.client.POST('/admin/chat/moderation-switches/requests/{request_id}:approve', {
          params: { path: { request_id: r.request_id } },
          headers: { ...intent, ...(r.tenant_id ? { [TENANT_HEADER]: r.tenant_id } : {}) },
        }),
      ),
    { onSettled: () => qc.invalidateQueries({ queryKey: SWITCHES }) },
  );
}
