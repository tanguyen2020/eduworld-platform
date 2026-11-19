// Game login history across tenants (F059 T041, F059-AC-18/19, FR-GAME-013, ADR-0031): listLoginEventsCrossTenant
// (`tenant_ids[]`, `student_id`, `from`, `to`, cursor — never the whole list). The contract has no IP (ADR-0031).
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { PAGE_SIZE } from '../limits';
import { useCursorList, usePlatformApi, type CursorPage, type Schemas } from './common';

export type LoginEvent = Schemas['LoginEvent'];
export type LoginEndReason = Schemas['LoginEndReason'];

/** i18n key of an `end_reason` (Vietnamese label, F059-AC-18); null = session still open. */
export const endReasonKey = (reason: LoginEndReason | null | undefined): string =>
  reason ? `platform.loginEvents.endReason.${reason}` : 'platform.loginEvents.endReason.open';

export function useLoginEvents(f: { from: string; to: string; studentId?: string }) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const query = scope.listQuery({ from: f.from, to: f.to, ...(f.studentId ? { student_id: f.studentId } : {}) });
  return useCursorList<LoginEvent>(['platform', 'login-events', query], (cursor) =>
    api.call<CursorPage<LoginEvent>>(
      api.client.GET('/admin/login-events', { params: { query: { ...query, limit: PAGE_SIZE, ...(cursor ? { cursor } : {}) } } }),
    ),
  );
}
