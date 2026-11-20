// Assignment hooks (F057 T020, F057-AC-05/06, ADR-0029): listTenantAssignments (cursor), assignToTenants and
// unassignFromTenants (optional `remove_default`, OQ F045-9) with one Idempotency-Key per user intent. Platform data: no X-Tenant-Id (platform scope).
import { useQueryClient } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { pageQuery, useCursorList, usePlatformApi, type CursorPage, type ResourceType, type Schemas } from './common';

export type TenantAssignment = Schemas['TenantAssignment'];
export type TenantAssignmentRequest = Schemas['TenantAssignmentRequest'];
export interface UnassignRequest {
  resource_type: ResourceType;
  resource_id: string;
  tenant_ids: string[];
  reason?: string;
  /** Also soft-delete the default assignment rule (`apply_to_new_tenants`) of the resource (OQ F045-9). */
  remove_default?: boolean;
}

/**
 * Types offered in the "Gán cho tenant" picker: platform content of learning/game/competition (ADMIN_PORTAL §3.7).
 * A UI choice list only — the backend decides what is assignable (platformadmin.Assignable).
 */
export const ASSIGNABLE_TYPES: readonly ResourceType[] = [
  'curriculum', 'lesson', 'question', 'question_set', 'assessment_blueprint', 'media', 'assignment', 'course', 'learning_path',
  'challenge', 'challenge_template', 'tournament_template', 'quest', 'minigame_config', 'item',
];

const key = (rt: ResourceType, id: string) => ['platform', 'assignments', rt, id];

export function useAssignments(resourceType: ResourceType, resourceId: string) {
  const api = usePlatformApi();
  return useCursorList<TenantAssignment>(key(resourceType, resourceId), (cursor) =>
    api.call<CursorPage<TenantAssignment>>(
      api.client.GET('/admin/tenant-assignments', { params: { query: { resource_type: resourceType, resource_id: resourceId, ...pageQuery(cursor) } } }),
    ),
  );
}

export function useAssign() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (body: TenantAssignmentRequest, headers) => api.call(api.client.POST('/admin/tenant-assignments', { body, headers })),
    { onSuccess: (_d, v) => qc.invalidateQueries({ queryKey: key(v.resource_type, v.resource_id) }) },
  );
}

export function useUnassign() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (body: UnassignRequest, headers) => api.call(api.client.DELETE('/admin/tenant-assignments', { body, headers })),
    { onSuccess: (_d, v) => qc.invalidateQueries({ queryKey: key(v.resource_type, v.resource_id) }) },
  );
}
