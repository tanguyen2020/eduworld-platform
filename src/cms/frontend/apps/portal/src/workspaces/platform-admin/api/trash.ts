// Trash hooks (F057 T028, F057-AC-09/10, ADR-0028, AGENTS R21): listTrash of every tenant + platform (`resource_type`
// required, `tenant_ids[]` from the System Admin filter, cursor) and restoreTrashItem (reason, Idempotency-Key,
// X-Tenant-Id = tenant of the item, none for platform data). There is no hard delete anywhere.
import { useQueryClient } from '@tanstack/react-query';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { pageQuery, useCursorList, usePlatformApi, type CursorPage, type ResourceType, type Schemas } from './common';

export type TrashItem = Schemas['TrashItem'];

/** Types listed in the trash filter: every ResourceType of the contract. */
export const TRASH_TYPES: readonly ResourceType[] = [
  'curriculum', 'lesson', 'learning_material', 'question', 'question_set', 'assessment_blueprint', 'media', 'assignment', 'course', 'learning_path',
  'activity', 'challenge', 'challenge_template', 'tournament_template', 'quest', 'minigame_config', 'item', 'school_settings',
  'tenant_settings', 'user', 'student',
];

export function useTrash(resourceType: ResourceType) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const query = scope.listQuery({ resource_type: resourceType });
  return useCursorList<TrashItem>(['platform', 'trash', query], (cursor) =>
    api.call<CursorPage<TrashItem>>(api.client.GET('/admin/trash', { params: { query: { ...query, ...pageQuery(cursor) } } })),
  );
}

export function useRestore() {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ item, reason }: { item: Pick<TrashItem, 'resource_type' | 'resource_id' | 'tenant_id'>; reason: string }, headers) =>
      api.call(
        api.client.POST('/admin/trash/{resource_type}/{resource_id}:restore', {
          params: { path: { resource_type: item.resource_type, resource_id: item.resource_id } },
          body: { reason },
          headers: { ...headers, ...(item.tenant_id ? scope.writeHeaders(item.tenant_id) : {}) },
        }),
      ),
    { onSuccess: () => qc.invalidateQueries({ queryKey: ['platform', 'trash'] }) },
  );
}
