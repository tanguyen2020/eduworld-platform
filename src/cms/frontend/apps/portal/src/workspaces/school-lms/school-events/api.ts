// API hooks of "Sự kiện trường" (F081 T010, T022; FR-EVT-006, FR-EVT-004): templates assigned to the tenant
// (listEventTemplates — the picker walks every page), school events (listSchoolEvents — cursor pages, System Admin across
// tenants with `tenant_ids`), one event (getSchoolEvent + ETag), create/update/cancel (Idempotency-Key per intent, If-Match
// on update) and the aggregate progress (getSchoolEventProgress). No rule here: the server decides the parameters, the
// limits, the scope and the cell suppression (AGENTS R1).
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { useAllPages } from '../../../shared/api/fetchAllPages';
import { IF_MATCH_HEADER } from '../../../shared/api/middleware/ifmatch';
import { TENANT_HEADER } from '../../../shared/api/middleware/tenant';
import { useCursorPages } from '../../../shared/api/useCursorPages';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { PAGE_SIZE, useApi, type Schemas } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';

export type EventTemplate = Schemas['EventTemplate'];
export type SchoolEvent = Schemas['SchoolEvent'];
export type SchoolEventCreate = Schemas['SchoolEventCreate'];
export type SchoolEventUpdate = Schemas['SchoolEventUpdate'];
export type SchoolEventStatus = Schemas['SchoolEventStatus'];
export type SchoolEventProgress = Schemas['SchoolEventProgress'];

interface Page<T> {
  data: T[];
  page: Schemas['PageInfo'];
}

/** Query parameter of the detail URL that carries the event's tenant for System Admin (platform scope). */
export const TENANT_PARAM = 'tenant';

/** Statuses of the list filter (SchoolEventStatus without the server-only `draft`/`archived`). */
export const LIST_STATUSES: SchoolEventStatus[] = ['scheduled', 'live', 'suspended', 'ended', 'cancelled'];

/**
 * Tenant of one event (detail, edit, cancel, progress): the session tenant for tenant users; for System Admin the tenant
 * of the row (`?tenant=` of the URL) or the one tenant of the topbar filter. `headers` is empty for tenant users (the
 * apiClient sets the session X-Tenant-Id).
 */
export function useEventTenant(): { ready: boolean; tenantId: string | null; headers: Record<string, string>; key: string } {
  const scope = useSchoolLmsScope();
  const cross = useCrossTenantScope();
  const [params] = useSearchParams();
  const fromUrl = params.get(TENANT_PARAM);
  if (cross.crossTenant && fromUrl) return { ready: true, tenantId: fromUrl, headers: { [TENANT_HEADER]: fromUrl }, key: fromUrl };
  return { ready: scope.ready, tenantId: scope.headers[TENANT_HEADER] ?? null, headers: scope.headers, key: scope.key };
}

const eventKey = (tenant: string, ...rest: unknown[]) => ['school-lms', 'school-events', tenant, ...rest];

/** listEventTemplates of a tenant: every page (template picker = dropdown rule), cached per tenant. */
export function useEventTemplates(tenantHeaders: Record<string, string>, tenantKey: string, enabled: boolean) {
  const api = useApi();
  return useAllPages<EventTemplate>(
    ['school-lms', 'event-templates', tenantKey],
    (q) => api.call<Page<EventTemplate>>(api.client.GET('/lms/event-templates', { params: { query: q }, headers: tenantHeaders })),
    { enabled },
  );
}

/**
 * listTenantSchools of a tenant for the audience and progress pickers: every page (dropdown rule), searchable. A School
 * Admin receives only the schools of their scope (server).
 */
export function useTenantSchools(tenantHeaders: Record<string, string>, tenantKey: string, enabled: boolean) {
  const api = useApi();
  return useAllPages<Schemas['School']>(
    ['school-lms', 'school-events-schools', tenantKey],
    (q) => api.call<{ items: Schemas['School'][]; page: Schemas['PageInfo'] }>(api.client.GET('/tenant/schools', { params: { query: q }, headers: tenantHeaders })),
    { enabled },
  );
}

/**
 * listSchoolEvents (F081-AC-06): cursor pages ("Xem thêm"), status filter in SQL. Tenant users: the session tenant;
 * System Admin: no X-Tenant-Id, `tenant_ids` = the topbar filter (empty = every tenant).
 */
export function useSchoolEvents(status: SchoolEventStatus | undefined) {
  const api = useApi();
  const cross = useCrossTenantScope();
  const query = cross.listQuery({ ...(status ? { status } : {}) });
  return useCursorPages<SchoolEvent>(['school-lms', 'school-events', 'list', cross.crossTenant ? cross.filter.join(',') || 'all' : 'session', query], (cursor) =>
    api.call<Page<SchoolEvent>>(api.client.GET('/lms/school-events', { params: { query: { ...query, limit: PAGE_SIZE, ...(cursor ? { cursor } : {}) } } })),
  );
}

/** getSchoolEvent with its ETag (version) for If-Match. */
export function useSchoolEvent(eventId: string | undefined) {
  const api = useApi();
  const tenant = useEventTenant();
  return useQuery({
    queryKey: eventKey(tenant.key, eventId),
    enabled: tenant.ready && !!eventId,
    queryFn: () => api.call<SchoolEvent>(api.client.GET('/lms/school-events/{event_id}', { params: { path: { event_id: eventId! } }, headers: tenant.headers })),
  });
}

/** createSchoolEvent (F081-AC-03): one Idempotency-Key per intent, kept on retry; the target tenant of System Admin. */
export function useCreateSchoolEvent() {
  const api = useApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ body, headers }: { body: SchoolEventCreate; headers: Record<string, string> }, intent) =>
      api.call<SchoolEvent>(api.client.POST('/lms/school-events', { body, headers: { ...intent, ...headers } })),
    { onSuccess: () => void qc.invalidateQueries({ queryKey: ['school-lms', 'school-events'] }) },
  );
}

/** updateSchoolEvent (F081-AC-07): If-Match = version of the event read; Idempotency-Key per intent. */
export function useUpdateSchoolEvent(eventId: string) {
  const api = useApi();
  const qc = useQueryClient();
  const tenant = useEventTenant();
  return useIdempotentMutation(
    ({ body, version }: { body: SchoolEventUpdate; version: number }, intent) =>
      api.call<SchoolEvent>(
        api.client.PATCH('/lms/school-events/{event_id}', {
          params: { path: { event_id: eventId } },
          body,
          headers: { ...intent, ...tenant.headers, [IF_MATCH_HEADER]: `"${version}"` },
        }),
      ),
    { onSuccess: () => void qc.invalidateQueries({ queryKey: ['school-lms', 'school-events'] }) },
  );
}

/** cancelSchoolEvent (F081-AC-08): reason 3–1000, Idempotency-Key per intent. */
export function useCancelSchoolEvent(eventId: string) {
  const api = useApi();
  const qc = useQueryClient();
  const tenant = useEventTenant();
  return useIdempotentMutation(
    (reason: string, intent) =>
      api.call<SchoolEvent>(
        api.client.POST('/lms/school-events/{event_id}:cancel', { params: { path: { event_id: eventId } }, body: { reason }, headers: { ...intent, ...tenant.headers } }),
      ),
    { onSettled: () => void qc.invalidateQueries({ queryKey: ['school-lms', 'school-events'] }) },
  );
}

/** getSchoolEventProgress (F081-AC-10, AC-11): refreshed by hand only (no polling). */
export function useSchoolEventProgress(eventId: string, schoolId: string | undefined, enabled: boolean) {
  const api = useApi();
  const tenant = useEventTenant();
  return useQuery({
    queryKey: eventKey(tenant.key, eventId, 'progress', schoolId ?? 'default'),
    enabled: enabled && tenant.ready,
    retry: false,
    queryFn: () =>
      api.call<SchoolEventProgress>(
        api.client.GET('/lms/school-events/{event_id}/progress', {
          params: { path: { event_id: eventId }, query: schoolId ? { school_id: schoolId } : {} },
          headers: tenant.headers,
        }),
      ),
  });
}
