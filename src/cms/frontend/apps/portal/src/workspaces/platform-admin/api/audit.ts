// Audit hooks (F059 T038, F059-AC-15…17, FR-AUD-001/002, ADMIN_PORTAL §7): listPlatformAuditLogs with mandatory
// `from`/`to`, `tenant_ids[]` of the tenant filter (no X-Tenant-Id; none selected = platform entries, as the backend
// answers), `actor_id`, `action`, `action_prefix[]`, `resource_type`, `resource_id`, cursor. The permission log is one
// query with `action_prefix=role&action_prefix=user_role` (OQ F059-3 resolved 2026-11-20), optionally narrowed to one
// exact `action` (AND). Read only: there is no write on audit.
import dayjs, { type Dayjs } from 'dayjs';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { AUDIT_DEFAULT_HOURS, LIMITS } from '../limits';
import { useCursorList, usePlatformApi, type CursorPage, type Schemas } from './common';

export type AuditLogEntry = Schemas['AuditLogEntry'];

export interface AuditFilter {
  from: string;
  to: string;
  actorId?: string;
  action?: string;
  /** `action_prefix[]` (OR between the values, AND with `action`). */
  actionPrefixes?: readonly string[];
  resourceType?: string;
  resourceId?: string;
}

/** Action groups of the permission log, sent as `action_prefix` values (dot-separated prefixes, F059-AC-16). */
export const PERMISSION_AUDIT_PREFIXES = ['role', 'user_role'] as const;

/**
 * Actions of the permission log (role.* and user_role.*), as emitted by module identity (F012 plan "Audit action"):
 * the optional "Hành động" choice that narrows the permission log to one exact action.
 */
export const PERMISSION_AUDIT_ACTIONS = ['role.created', 'role.updated', 'role.deleted', 'user_role.granted', 'user_role.revoked', 'user_role.expired'] as const;

export const isPermissionAction = (action: string): boolean => /^(role|user_role)\./.test(action);

export type AuditMode = 'system' | 'permissions';

/** Filter of one tab from the URL (`from`/`to` default to the last AUDIT_DEFAULT_HOURS hours). */
export function filterFromParams(params: URLSearchParams, mode: AuditMode, now: Dayjs = dayjs()): AuditFilter {
  const from = params.get('from') ?? now.subtract(AUDIT_DEFAULT_HOURS, 'hour').toISOString();
  const to = params.get('to') ?? now.toISOString();
  const action = params.get('action') ?? undefined;
  return {
    from,
    to,
    actorId: params.get('actor_id') ?? undefined,
    action: mode === 'permissions' ? (action && (PERMISSION_AUDIT_ACTIONS as readonly string[]).includes(action) ? action : undefined) : action,
    ...(mode === 'permissions' ? { actionPrefixes: PERMISSION_AUDIT_PREFIXES } : {}),
    resourceType: params.get('resource_type') ?? undefined,
    resourceId: params.get('resource_id') ?? undefined,
  };
}

export function useAuditLogs(f: AuditFilter) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const query = scope.listQuery({
    from: f.from,
    to: f.to,
    ...(f.actorId ? { actor_id: f.actorId } : {}),
    ...(f.action ? { action: f.action } : {}),
    ...(f.actionPrefixes?.length ? { action_prefix: [...f.actionPrefixes] } : {}),
    ...(f.resourceType ? { resource_type: f.resourceType } : {}),
    ...(f.resourceId ? { resource_id: f.resourceId } : {}),
  });
  return useCursorList<AuditLogEntry>(['platform', 'audit', query], (cursor) =>
    api.call<CursorPage<AuditLogEntry>>(
      api.client.GET('/admin/audit-logs', { params: { query: { ...query, limit: LIMITS.pageSize.maximum, ...(cursor ? { cursor } : {}) } } }),
    ),
  );
}
