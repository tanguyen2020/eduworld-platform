// Display permissions of workspace A (F079-AC-01, AC-03): which write/review/publish buttons a user sees. Display only:
// every request is authorized by module game (AGENTS R20; reviewer ≠ author is enforced by the server).
import { useMe } from '../../../shared/api/hooks/useMe';
import { usePermissions } from '../../../permissions/usePermissions';
import type { ConfigTypeDef } from '../registry/configTypes';

export const REVIEW_PERMISSION = 'game.content.review';
export const PUBLISH_PERMISSION = 'game.content.publish';

export function useGameAccess(def: ConfigTypeDef) {
  const { view } = usePermissions();
  const me = useMe();
  const perms = view?.permissions ?? [];
  return {
    write: perms.includes(def.writePermission),
    review: perms.includes(REVIEW_PERMISSION),
    publish: perms.includes(PUBLISH_PERMISSION),
    userId: me.data?.user_id,
  };
}
