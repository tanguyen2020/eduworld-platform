// Menu "Tích hợp — Keycloak · SIS · APISIX (tham chiếu)" (F059 US7, F059-AC-22/23, FR-SIS-007, FR-AUTH-012,
// ADMIN_PORTAL §10): read only. Tab "Keycloak · APISIX" waits for `getIntegrationOverview` (delta T009, not in the
// contract — Câu hỏi mở F059-Q2) and says so; configuration changes go through realm-as-code / route-as-code (F004).
// Tab "SIS" (action tenants.sis_connections) lists the SIS connections of every tenant with their schools and sync runs.
// No input that writes anything, no secret, credential or token: only schema fields are rendered.
import { Alert, Tabs, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { INTEGRATION_OVERVIEW_AVAILABLE } from '../api/integrations';
import { PageTitle } from '../ui';
import { SisConnectionsTab } from './SisConnectionsTab';

export function IntegrationsPage({ title }: { title: string }) {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const canSis = !!view && actionStatus('action.platform_admin.tenants.sis_connections', view) === 'enabled';
  const items = [
    {
      key: 'reference',
      label: t('platform.integrations.tabReference'),
      children: (
        <section>
          {INTEGRATION_OVERVIEW_AVAILABLE ? null : <Alert type="info" showIcon message={t('platform.integrations.overviewUnavailable')} />}
          <Typography.Paragraph type="secondary">{t('platform.integrations.asCode')}</Typography.Paragraph>
        </section>
      ),
    },
    ...(canSis ? [{ key: 'sis', label: t('platform.integrations.tabSis'), children: <SisConnectionsTab /> }] : []),
  ];
  return (
    <section>
      <PageTitle title={title} />
      <Typography.Paragraph type="secondary">{t('platform.integrations.readOnly')}</Typography.Paragraph>
      <Tabs items={items} destroyOnHidden />
    </section>
  );
}
