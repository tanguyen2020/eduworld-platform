// API hooks of workspace E "Kiểm duyệt" (F122 T013/T017/T032, plan "application" layer): moderation cases, case messages,
// assign/act, school notifications, sanctions, tenant terms and tenant categories. Hooks only shape requests: every
// write of a case carries X-Tenant-Id = the tenant of the case (or the chosen target tenant), Idempotency-Key (apiClient,
// one key per user intent — useIdempotentMutation) and If-Match = the case version; nothing is retried automatically.
// Authorization, SLA and every rule stay in module chat (AGENTS R1, R7, R20).
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { IF_MATCH_HEADER } from '../../../shared/api/middleware/ifmatch';
import { TENANT_HEADER } from '../../../shared/api/middleware/tenant';
import type { components } from '../../../shared/api/schema.gen';
import { useCursorPages } from '../../../shared/api/useCursorPages';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';

export type Schemas = components['schemas'];
export type ModerationCase = Schemas['ModerationCase'];
export type ModerationCaseDetail = Schemas['ModerationCaseDetail'];
export type CaseMessage = Schemas['ModerationCaseMessage'];
export type ModerationAction = Schemas['ModerationActionRequest'];
export type Sanction = Schemas['Sanction'];
export type SanctionCreate = Schemas['SanctionCreate'];
export type SchoolNotification = Schemas['SchoolSafetyNotification'];
export type SchoolNotificationCreate = Schemas['SchoolSafetyNotificationCreate'];
export type SensitiveTerm = Schemas['SensitiveTerm'];
export type SensitiveTermInput = Schemas['SensitiveTermInput'];
export type TenantCategory = Schemas['TenantSensitiveCategory'];
export type CatalogVersion = Schemas['SensitiveCatalogVersion'];

interface Page<T> {
  data: T[];
  page: Schemas['PageInfo'];
}

/** Page size of the tables of the module (≤ Limit.maximum; cursor "Tải thêm", never the whole list). */
export const PAGE_SIZE = 50;

const pageQuery = (cursor?: string) => (cursor ? { limit: PAGE_SIZE, cursor } : { limit: PAGE_SIZE });
const tenantHeaders = (tenantId: string) => ({ [TENANT_HEADER]: tenantId });
const caseIfMatch = (version?: number) => (version == null ? {} : { [IF_MATCH_HEADER]: `W/"${version}"` });

function useApi() {
  return usePortalRuntime().api;
}

export interface CaseFilters {
  tenantIds: string[];
  severity?: 'p1' | 'p2' | 'p3';
  status?: Schemas['ModerationCaseStatus'];
  assignee?: 'me' | 'unassigned' | 'any';
  category?: string;
}

/** listModerationCases (F122-AC-01): tenant filter of the module (`tenant_ids[]`, empty = every tenant), cursor pages. */
export function useCases(f: CaseFilters) {
  const api = useApi();
  const query = {
    ...(f.tenantIds.length ? { tenant_ids: f.tenantIds } : {}),
    ...(f.severity ? { severity: f.severity } : {}),
    ...(f.status ? { status: f.status } : {}),
    ...(f.assignee ? { assignee: f.assignee } : {}),
    ...(f.category ? { category_code: f.category } : {}),
  };
  return useCursorPages<ModerationCase>(['live-ops', 'cases', query], (cursor) =>
    api.call<Page<ModerationCase>>(api.client.GET('/admin/moderation/cases', { params: { query: { ...query, ...pageQuery(cursor) } } })),
  );
}

export function useCase(caseId: string) {
  const api = useApi();
  return useQuery({
    queryKey: ['live-ops', 'case', caseId],
    queryFn: () => api.call<ModerationCaseDetail>(api.client.GET('/admin/moderation/cases/{case_id}', { params: { path: { case_id: caseId } } })),
  });
}

/** listModerationCaseMessages: evidence snapshot ± context; `q` searches inside this case only (F122-AC-02). */
export function useCaseMessages(caseId: string, q: string) {
  const api = useApi();
  return useCursorPages<CaseMessage>(['live-ops', 'case-messages', caseId, q], (cursor) =>
    api.call<Page<CaseMessage>>(
      api.client.GET('/admin/moderation/cases/{case_id}/messages', { params: { path: { case_id: caseId }, query: { ...(q ? { q } : {}), ...pageQuery(cursor) } } }),
    ),
  );
}

function useCaseInvalidation(caseId: string) {
  const qc = useQueryClient();
  return () => Promise.all([qc.invalidateQueries({ queryKey: ['live-ops', 'case', caseId] }), qc.invalidateQueries({ queryKey: ['live-ops', 'cases'] })]);
}

/** assignModerationCase (take the case): X-Tenant-Id = tenant of the case. */
export function useAssignCase(c: Pick<ModerationCase, 'case_id' | 'tenant_id'>) {
  const api = useApi();
  const reload = useCaseInvalidation(c.case_id);
  return useIdempotentMutation(
    (moderatorId: string, intent) =>
      api.call<ModerationCase>(
        api.client.POST('/admin/moderation/cases/{case_id}:assign', {
          params: { path: { case_id: c.case_id } },
          body: { assignee_moderator_id: moderatorId },
          headers: { ...intent, ...tenantHeaders(c.tenant_id) },
        }),
      ),
    { onSettled: reload },
  );
}

/** actOnModerationCase: If-Match = case version, X-Tenant-Id = tenant of the case; 412 reloads the case (F122-AC-04). */
export function useActOnCase(c: Pick<ModerationCase, 'case_id' | 'tenant_id' | 'version'>) {
  const api = useApi();
  const reload = useCaseInvalidation(c.case_id);
  return useIdempotentMutation(
    (body: ModerationAction, intent) =>
      api.call<ModerationCaseDetail>(
        api.client.POST('/admin/moderation/cases/{case_id}:act', {
          params: { path: { case_id: c.case_id } },
          body,
          headers: { ...intent, ...tenantHeaders(c.tenant_id), ...caseIfMatch(c.version) },
        }),
      ),
    { onSettled: reload },
  );
}

/** notifySchoolForCase (F122-AC-05). */
export function useNotifySchool(c: Pick<ModerationCase, 'case_id' | 'tenant_id'>) {
  const api = useApi();
  const reload = useCaseInvalidation(c.case_id);
  return useIdempotentMutation(
    (body: SchoolNotificationCreate, intent) =>
      api.call<SchoolNotification>(
        api.client.POST('/admin/moderation/cases/{case_id}:notify-school', {
          params: { path: { case_id: c.case_id } },
          body,
          headers: { ...intent, ...tenantHeaders(c.tenant_id) },
        }),
      ),
    { onSuccess: reload },
  );
}

/** listSchoolSafetyNotificationsPlatform (F122-AC-06). */
export function usePlatformNotifications(f: { tenantIds: string[]; acknowledged?: boolean; overdue?: boolean }) {
  const api = useApi();
  const query = {
    ...(f.tenantIds.length ? { tenant_ids: f.tenantIds } : {}),
    ...(f.acknowledged != null ? { acknowledged: f.acknowledged } : {}),
    ...(f.overdue ? { overdue: true } : {}),
  };
  return useCursorPages<SchoolNotification>(['live-ops', 'platform-notifications', query], (cursor) =>
    api.call<Page<SchoolNotification>>(api.client.GET('/admin/safety/school-notifications', { params: { query: { ...query, ...pageQuery(cursor) } } })),
  );
}

/** listSanctions. */
export function useSanctions(f: { tenantIds: string[]; userId?: string; active?: boolean }) {
  const api = useApi();
  const query = {
    ...(f.tenantIds.length ? { tenant_ids: f.tenantIds } : {}),
    ...(f.userId ? { user_id: f.userId } : {}),
    ...(f.active != null ? { active: f.active } : {}),
  };
  return useCursorPages<Sanction>(['live-ops', 'sanctions', query], (cursor) =>
    api.call<Page<Sanction>>(api.client.GET('/admin/moderation/sanctions', { params: { query: { ...query, ...pageQuery(cursor) } } })),
  );
}

/** createSanction (X-Tenant-Id = tenant of the student). */
export function useCreateSanction() {
  const api = useApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ tenantId, body }: { tenantId: string; body: SanctionCreate }, intent) =>
      api.call<Sanction>(api.client.POST('/admin/moderation/sanctions', { body, headers: { ...intent, ...tenantHeaders(tenantId) } })),
    { onSettled: () => qc.invalidateQueries({ queryKey: ['live-ops'] }) },
  );
}

/** revokeSanction (reason required). */
export function useRevokeSanction() {
  const api = useApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ sanction, reason }: { sanction: Pick<Sanction, 'sanction_id' | 'tenant_id'>; reason: string }, intent) =>
      api.call<Sanction>(
        api.client.POST('/admin/moderation/sanctions/{sanction_id}:revoke', {
          params: { path: { sanction_id: sanction.sanction_id } },
          body: { reason },
          headers: { ...intent, ...tenantHeaders(sanction.tenant_id) },
        }),
      ),
    { onSettled: () => qc.invalidateQueries({ queryKey: ['live-ops', 'sanctions'] }) },
  );
}

// ---- tenant terms and categories (Safety Team for one target tenant, F122 US6) ----

export function useTenantTerms(tenantId: string | undefined) {
  const api = useApi();
  return useCursorPages<SensitiveTerm>(
    ['live-ops', 'tenant-terms', tenantId],
    (cursor) => api.call<Page<SensitiveTerm>>(api.client.GET('/admin/chat/tenant-terms', { params: { query: pageQuery(cursor) }, headers: tenantHeaders(tenantId!) })),
    { enabled: !!tenantId },
  );
}

/** listTenantSensitiveCategories of the target tenant (X-Tenant-Id): names, state, allow_tenant_toggle — never a term. */
export function useTenantCategories(tenantId: string | undefined) {
  const api = useApi();
  return useQuery({
    queryKey: ['live-ops', 'tenant-categories', tenantId],
    enabled: !!tenantId,
    queryFn: () => api.call<{ data: TenantCategory[] }>(api.client.GET('/tenant/chat/categories', { headers: tenantHeaders(tenantId!) })),
  });
}

export function useTenantTermWrite(tenantId: string | undefined) {
  const api = useApi();
  const qc = useQueryClient();
  const reload = () => qc.invalidateQueries({ queryKey: ['live-ops', 'tenant-terms', tenantId] });
  const create = useIdempotentMutation(
    (body: SensitiveTermInput, intent) => api.call<SensitiveTerm>(api.client.POST('/admin/chat/tenant-terms', { body, headers: { ...intent, ...tenantHeaders(tenantId!) } })),
    { onSuccess: reload },
  );
  const remove = useIdempotentMutation(
    (termId: string) => api.call(api.client.DELETE('/admin/chat/tenant-terms/{term_id}', { params: { path: { term_id: termId } }, headers: tenantHeaders(tenantId!) })),
    { onSuccess: reload },
  );
  const publish = useIdempotentMutation(
    (changeSummary: string, intent) =>
      api.call<CatalogVersion>(api.client.POST('/admin/chat/tenant-terms:publish', { body: { change_summary: changeSummary }, headers: { ...intent, ...tenantHeaders(tenantId!) } })),
    { onSuccess: reload },
  );
  return { create, remove, publish };
}

/** testSensitiveTerms with the target tenant (`tenant_id` in the body: platform + tenant catalogue). */
export function useTestTerms() {
  const api = useApi();
  return useIdempotentMutation((body: Schemas['SensitiveTermTestRequest']) =>
    api.call<Schemas['SensitiveTermTestResult']>(api.client.POST('/admin/chat/sensitive-terms:test', { body })),
  );
}

export function useToggleTenantCategory(tenantId: string | undefined) {
  const api = useApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ code, enabled, reason }: { code: string; enabled: boolean; reason: string }, intent) =>
      api.call<TenantCategory>(
        api.client.POST('/admin/chat/tenant-categories/{code}:toggle', {
          params: { path: { code } },
          body: { enabled, reason },
          headers: { ...intent, ...tenantHeaders(tenantId!) },
        }),
      ),
    { onSettled: () => qc.invalidateQueries({ queryKey: ['live-ops', 'tenant-categories', tenantId] }) },
  );
}

/** listSensitiveCategories (platform catalogue, read): the category ids the tenant terms are written into (F122-AC-15). */
export function useCatalogCategories(enabled = true) {
  const api = useApi();
  return useQuery({
    queryKey: ['live-ops', 'catalog-categories'],
    enabled,
    queryFn: () => api.call<{ data: Schemas['SensitiveTermCategory'][] }>(api.client.GET('/admin/chat/sensitive-categories', {})),
  });
}
