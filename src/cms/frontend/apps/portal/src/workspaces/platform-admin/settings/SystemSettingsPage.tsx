// Menu "Cấu hình hệ thống" (F057 US6, ADMIN_PORTAL §3.10): student cleanup settings, cleanup runs awaiting approval,
// gameplay chat retention and the platform curfew. Viewing needs `action.platform_admin.system_settings.view`;
// editing (forms enabled, Save shown) `…system_settings.edit`; approving a run `…approve_cleanup`.
import { Alert, Space } from 'antd';
import { useTranslation } from 'react-i18next';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { ForbiddenPage } from '../../../shell/StatusPages';
import { PageTitle } from '../ui';
import { ChatRetentionCard } from './ChatRetentionPage';
import { CleanupRunsCard } from './CleanupRunsPage';
import { PlatformCurfewSettingsCard } from './PlatformCurfewSettingsCard';
import { StudentCleanupSettingsCard } from './StudentCleanupSettingsPage';

export function SystemSettingsPage({ title }: { title: string }) {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const can = (k: Parameters<typeof actionStatus>[0]) => !!view && actionStatus(k, view) === 'enabled';
  if (!can('action.platform_admin.system_settings.view')) return <ForbiddenPage />;
  const canEdit = can('action.platform_admin.system_settings.edit');
  return (
    <section>
      <PageTitle title={title} />
      {canEdit ? null : <Alert type="info" showIcon message={t('platform.settings.readOnly')} />}
      <Space direction="vertical" style={{ width: '100%' }}>
        <StudentCleanupSettingsCard canEdit={canEdit} />
        <CleanupRunsCard />
        <ChatRetentionCard canEdit={canEdit} />
        <PlatformCurfewSettingsCard canEdit={canEdit} />
      </Space>
    </section>
  );
}
