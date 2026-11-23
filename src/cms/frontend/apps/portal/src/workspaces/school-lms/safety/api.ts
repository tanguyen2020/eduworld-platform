// API hooks of the chat-safety screens of workspace C (F122 T037/T040/T032/T042): school safety notifications (list,
// acknowledge), the school chat policy (getChatPolicy, getEffectiveChatPolicy per channel type, putChatPolicy with
// If-Match + reason), the tenant categories (read only) and the clubs of a school (approve, archive). Every call carries the
// tenant of the scope (session tenant, or the one tenant a System Admin picked — useSchoolLmsScope). No rule here.
import { useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import { IF_MATCH_HEADER } from '../../../shared/api/middleware/ifmatch';
import { ApiError } from '../../../shared/api/middleware/problem';
import { useCursorPages } from '../../../shared/api/useCursorPages';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { PAGE_SIZE, qk, useApi, type Schemas } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';

export type { Schemas };
export type SchoolNotification = Schemas['SchoolSafetyNotification'];
export type ChatPolicyDocument = Schemas['ChatPolicyDocument'];
export type ChatPolicyPut = Schemas['ChatPolicyPut'];
export type EffectivePolicy = Schemas['EffectiveChatPolicy'];
export type ChannelType = Schemas['ChannelType'];
export type TenantCategory = Schemas['TenantSensitiveCategory'];
export type Club = Schemas['Club'];
export type ClubMember = Schemas['ClubMember'];

interface Page<T> {
  data: T[];
  page: Schemas['PageInfo'];
}

const pageQuery = (cursor?: string) => (cursor ? { limit: PAGE_SIZE, cursor } : { limit: PAGE_SIZE });

/** Channel types of the school policy screen (no class/study_group — ADR-0095). */
export const POLICY_CHANNELS: ChannelType[] = ['direct', 'world', 'area', 'team', 'match', 'party'];
/** Always-on gameplay chat (ADR-0020 §6): no on/off, no hours; only rate, slow mode ≤ 10 s, strictness tighten. */
export const ALWAYS_ON: ChannelType[] = ['match', 'team', 'party'];

/** listSchoolSafetyNotifications; `createdFrom` is filtered by module chat in SQL (F122 T066). */
export function useSchoolNotifications(f: { acknowledged?: boolean; schoolId?: string; createdFrom?: string }) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const query = {
    ...(f.acknowledged != null ? { acknowledged: f.acknowledged } : {}),
    ...(f.schoolId ? { school_id: f.schoolId } : {}),
    ...(f.createdFrom ? { created_from: f.createdFrom } : {}),
  };
  return useCursorPages<SchoolNotification>(
    qk('safety-notifications', scope, query),
    (cursor) => api.call<Page<SchoolNotification>>(api.client.GET('/tenant/safety/school-notifications', { params: { query: { ...query, ...pageQuery(cursor) } }, headers: scope.headers })),
    { enabled: scope.ready },
  );
}

/** acknowledgeSchoolSafetyNotification (F122-AC-19): Idempotency-Key per intent; 409 = already acknowledged. */
export function useAcknowledge() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ id, body }: { id: string; body: Schemas['SchoolSafetyNotificationAck'] }, intent) =>
      api.call<SchoolNotification>(
        api.client.POST('/tenant/safety/school-notifications/{notification_id}:acknowledge', {
          params: { path: { notification_id: id } },
          body,
          headers: { ...intent, ...scope.headers },
        }),
      ),
    { onSettled: () => qc.invalidateQueries({ queryKey: ['school-lms', 'safety-notifications'] }) },
  );
}

/** The school document (null = no school-level policy: every value inherited from tenant/platform). */
export function useSchoolPolicy(schoolId: string | undefined) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useQuery({
    queryKey: qk('chat-policy', scope, schoolId),
    enabled: scope.ready && !!schoolId,
    queryFn: async () => {
      try {
        return await api.call<ChatPolicyDocument>(
          api.client.GET('/tenant/chat-policies/{scope_type}/{scope_id}', { params: { path: { scope_type: 'school', scope_id: schoolId! } }, headers: scope.headers }),
        );
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) return null;
        throw e;
      }
    },
  });
}

/** getEffectiveChatPolicy of every channel type of the screen (limits from the levels above). */
export function useEffectivePolicies(schoolId: string | undefined) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const results = useQueries({
    queries: POLICY_CHANNELS.map((ch) => ({
      queryKey: qk('chat-policy-effective', scope, schoolId, ch),
      enabled: scope.ready && !!schoolId,
      queryFn: async () => {
        try {
          return await api.call<EffectivePolicy>(
            api.client.GET('/tenant/chat-policies:effective', { params: { query: { school_id: schoolId!, channel_type: ch } }, headers: scope.headers }),
          );
        } catch (e) {
          if (e instanceof ApiError && e.status === 404) return null; // no rule above: the channel is off for the school
          throw e;
        }
      },
    })),
  });
  return { byChannel: Object.fromEntries(POLICY_CHANNELS.map((ch, i) => [ch, results[i]?.data ?? null])) as Record<ChannelType, EffectivePolicy | null>, error: results.find((r) => r.error)?.error, loading: results.some((r) => r.isPending) };
}

/** putChatPolicy (scope school): If-Match = version of the document ("0" for the first one), reason mandatory. */
export function usePutSchoolPolicy(schoolId: string | undefined) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ body, version }: { body: ChatPolicyPut; version: number }) =>
      api.call<ChatPolicyDocument>(
        api.client.PUT('/tenant/chat-policies/{scope_type}/{scope_id}', {
          params: { path: { scope_type: 'school', scope_id: schoolId! } },
          body,
          headers: { ...scope.headers, [IF_MATCH_HEADER]: `"${version}"` },
        }),
      ),
    { onSettled: () => qc.invalidateQueries({ queryKey: ['school-lms', 'chat-policy'] }) },
  );
}

/** listTenantSensitiveCategories (read only for the tenant: names, severity, state — never a term). */
/** listTenantSensitiveCategories (read only table): cursor pages, keyset in SQL (owner rule 2026-11-23). */
export function useTenantCategories(enabled: boolean) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useCursorPages<TenantCategory>(
    qk('chat-categories', scope),
    (cursor) => api.call<Page<TenantCategory>>(api.client.GET('/tenant/chat/categories', { params: { query: pageQuery(cursor) }, headers: scope.headers })),
    { enabled: enabled && scope.ready },
  );
}

export function useClubs(f: { schoolId?: string; status?: Schemas['ClubStatus'] }) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const query = { ...(f.schoolId ? { school_id: f.schoolId } : {}), ...(f.status ? { status: f.status } : {}) };
  return useCursorPages<Club>(
    qk('clubs', scope, query),
    (cursor) => api.call<Page<Club>>(api.client.GET('/social/clubs', { params: { query: { ...query, ...pageQuery(cursor) } }, headers: scope.headers })),
    { enabled: scope.ready && !!f.schoolId },
  );
}

/** listSchoolClubMembers (F122 T061): members of a club of the school, cursor pages in SQL ("Tải thêm"). */
export function useClubMembers(clubId: string | undefined) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useCursorPages<ClubMember>(
    qk('club-members', scope, clubId),
    (cursor) =>
      api.call<Page<ClubMember>>(
        api.client.GET('/tenant/social/clubs/{club_id}/members', { params: { path: { club_id: clubId! }, query: pageQuery(cursor) }, headers: scope.headers }),
      ),
    { enabled: scope.ready && !!clubId },
  );
}

/** removeSchoolClubMember (F122 T061): reason required, Idempotency-Key per intent; reloads members and clubs. */
export function useRemoveClubMember() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ clubId, userId, reason }: { clubId: string; userId: string; reason: string }, intent) =>
      api.call(
        api.client.POST('/tenant/social/clubs/{club_id}/members/{user_id}:remove', {
          params: { path: { club_id: clubId, user_id: userId } },
          body: { reason },
          headers: { ...intent, ...scope.headers },
        }),
      ),
    {
      onSuccess: () => {
        void qc.invalidateQueries({ queryKey: ['school-lms', 'club-members'] });
        void qc.invalidateQueries({ queryKey: ['school-lms', 'clubs'] });
      },
    },
  );
}

export function useClubActions() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const qc = useQueryClient();
  const reload = () => qc.invalidateQueries({ queryKey: ['school-lms', 'clubs'] });
  return {
    approve: useIdempotentMutation(
      (clubId: string, intent) => api.call<Club>(api.client.POST('/social/clubs/{club_id}:approve', { params: { path: { club_id: clubId } }, headers: { ...intent, ...scope.headers } })),
      { onSettled: reload },
    ),
    archive: useIdempotentMutation(
      (clubId: string, intent) => api.call(api.client.POST('/social/clubs/{club_id}:archive', { params: { path: { club_id: clubId } }, headers: { ...intent, ...scope.headers } })),
      { onSettled: reload },
    ),
  };
}
