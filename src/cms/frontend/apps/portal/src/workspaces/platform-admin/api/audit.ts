// Audit hooks (F059 T038, F059-AC-15…17, FR-AUD-001/002, ADMIN_PORTAL §7): listPlatformAuditLogs with mandatory
// `from`/`to`, `tenant_ids[]` of the tenant filter (no X-Tenant-Id; none selected = platform entries, as the backend
// answers), `actor_id`, `action`, `resource_type`, `resource_id`, cursor. Read only: there is no write on audit.
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
  resourceType?: string;
  resourceId?: string;
}

/**
 * Actions of the permission log (role.* and user_role.*), as emitted by module identity (F012 plan "Audit action").
 * The audit query matches `action` exactly (no prefix filter yet — delta T007 owner F015, Câu hỏi mở F059-Q3), so the
 * permission log asks for one of these at a time.
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
    action: mode === 'permissions' ? (action && (PERMISSION_AUDIT_ACTIONS as readonly string[]).includes(action) ? action : PERMISSION_AUDIT_ACTIONS[0]) : action,
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
    ...(f.resourceType ? { resource_type: f.resourceType } : {}),
    ...(f.resourceId ? { resource_id: f.resourceId } : {}),
  });
  return useCursorList<AuditLogEntry>(['platform', 'audit', query], (cursor) =>
    api.call<CursorPage<AuditLogEntry>>(
      api.client.GET('/admin/audit-logs', { params: { query: { ...query, limit: LIMITS.pageSize.maximum, ...(cursor ? { cursor } : {}) } } }),
    ),
  );
}
