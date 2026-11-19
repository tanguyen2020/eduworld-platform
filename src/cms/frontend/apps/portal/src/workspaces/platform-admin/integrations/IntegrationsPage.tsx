// Menu "Tích hợp — Keycloak · SIS · APISIX (tham chiếu)" (F059 US7, F059-AC-22/23, FR-SIS-007, FR-AUTH-012,
// ADMIN_PORTAL §10): read only. Tab "Keycloak · APISIX" (action integrations.view) shows getIntegrationOverview
// (OQ F059-2); configuration changes go through realm-as-code / route-as-code (F004). Tab "SIS" (action
// tenants.sis_connections) lists the SIS connections of every tenant with their schools and sync runs.
// No input that writes anything, no secret, credential or token: only schema fields are rendered.
import { Tabs, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { PageTitle } from '../ui';
import { IntegrationOverviewTab } from './IntegrationOverviewTab';
import { SisConnectionsTab } from './SisConnectionsTab';

export function IntegrationsPage({ title }: { title: string }) {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const canOverview = !!view && actionStatus('action.platform_admin.integrations.view', view) === 'enabled';
  const canSis = !!view && actionStatus('action.platform_admin.tenants.sis_connections', view) === 'enabled';
  const items = [
    { key: 'reference', label: t('platform.integrations.tabReference'), children: <IntegrationOverviewTab enabled={canOverview} /> },
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
