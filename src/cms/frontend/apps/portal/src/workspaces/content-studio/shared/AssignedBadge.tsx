// AssignedBadge (F053-AC-11, AC-29, ADR-0029): platform content assigned to the tenant is shown "Từ EduWorld" and
// edited by nobody in the tenant (read only).
import { Tag } from 'antd';
import { useTranslation } from 'react-i18next';

export function AssignedBadge() {
  const { t } = useTranslation();
  return (
    <Tag color="blue" data-testid="assigned-badge">
      {t('content_studio.assigned')}
    </Tag>
  );
}
