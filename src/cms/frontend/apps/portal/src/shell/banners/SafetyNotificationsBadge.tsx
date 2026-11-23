// Badge of the menu "Thông báo an toàn của trường" (F122-AC-18): number of notifications waiting for the school's
// acknowledgement (first page; "50+" beyond). Tenant staff holding moderation.school_notification.read only.
import { Badge } from 'antd';
import { useTranslation } from 'react-i18next';
import { usePermissions } from '../../permissions/usePermissions';
import { usePendingSafetyNotifications } from '../../shared/chatSafety/useModerationStatus';

export function SafetyNotificationsBadge() {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const pending = usePendingSafetyNotifications(!!view?.permissions.includes('moderation.school_notification.read'));
  if (!pending.count) return null;
  const label = pending.more ? `${pending.count}+` : String(pending.count);
  return <Badge count={label} size="small" offset={[4, -2]} aria-label={t('chatSafety.notifications.badge', { count: label })} data-testid="safety-notifications-badge" />;
}
