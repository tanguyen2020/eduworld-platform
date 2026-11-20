// Tenant audit (F052 T036, T046, T060; F052-AC-18, F052-AC-23): listTenantAuditLogs filtered by school, actor, action
// (exact match on the server), action groups (`action_prefix[]`, OR between groups, AND with `action` — OQ F052-2) and
// date range. Moderation entries are excluded by the server; the client also drops any
// `moderation.*` row it might receive (defence in depth, AGENTS R16, ADR-0072).
import { PAGE_SIZE, qk, useApi, usePagedList, type Schemas } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';

export type AuditEntry = Schemas['AuditLogEntry'];

export interface AuditFilters {
  schoolId?: string;
  actorId?: string;
  action?: string;
  /** `action_prefix[]`: dotted action groups (`role` matches `role.created`), OR between them. */
  actionPrefixes?: readonly string[];
  /** Required by the API (ISO date-time). */
  from: string;
  to: string;
}

export const isModeration = (e: Pick<AuditEntry, 'action' | 'target_type'>) => e.action.startsWith('moderation.') || e.target_type === 'moderation_case';

/** Action groups of the permission log: roles and UserRoles written by module identity (OQ F052-2). */
export const PERMISSION_AUDIT_PREFIXES = ['role', 'user_role'] as const;

/** Audit actions of roles and UserRoles written by module identity (role.<op>, user_role.<op>) — optional narrowing. */
export const PERMISSION_AUDIT_ACTIONS = ['user_role.granted', 'user_role.revoked', 'user_role.expired', 'role.created', 'role.updated', 'role.deleted'] as const;

export function useAuditLogs(f: AuditFilters, enabled = true) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return usePagedList<AuditEntry>(
    qk('audit', scope, f.schoolId ?? '', f.actorId ?? '', f.action ?? '', (f.actionPrefixes ?? []).join(','), f.from, f.to),
    async (cursor) => {
      const res = await api.call(
        api.client.GET('/tenant/audit-logs', {
          params: {
            query: {
              limit: PAGE_SIZE,
              ...(cursor ? { cursor } : {}),
              ...(f.schoolId ? { school_id: f.schoolId } : {}),
              ...(f.actorId ? { actor_id: f.actorId } : {}),
              ...(f.action ? { action: f.action } : {}),
              ...(f.actionPrefixes?.length ? { action_prefix: [...f.actionPrefixes] } : {}),
              from: f.from,
              to: f.to,
            },
          },
          headers: scope.headers,
        }),
      );
      return { items: res.data.filter((e) => !isModeration(e)), page: res.page };
    },
    scope.ready && enabled,
  );
}
