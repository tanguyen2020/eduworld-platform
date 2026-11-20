// Tab "Keycloak · APISIX" (F059 T046, F059-AC-22/23, FR-AUTH-012, ADMIN_PORTAL §10): getIntegrationOverview, read only —
// Keycloak issuer/realm/clients (id + purpose), MFA of platform routes and the max auth age; APISIX public/internal
// gateway URLs ("Chưa cấu hình" when null) and the route groups prefix → gateway → service generated at build time from
// the route-as-code file (OQ F059-12; "Chưa có dữ liệu" when the list is empty); SIS connections by status. Only named schema fields are rendered (no secret, no field
// outside the schema); configuration changes go through realm-as-code / route-as-code (F004).
import { Card, Descriptions, Space, Table, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useIntegrationOverview, type IntegrationOverview } from '../api/integrations';
import { formatTime } from '../ui';

function UrlValue({ url }: { url: string | null | undefined }) {
  const { t } = useTranslation();
  return url ? <Typography.Text code>{url}</Typography.Text> : <Typography.Text type="secondary">{t('platform.integrations.notConfigured')}</Typography.Text>;
}

type RouteGroup = IntegrationOverview['apisix']['route_groups'][number];

function RouteGroups({ groups }: { groups: RouteGroup[] }) {
  const { t } = useTranslation();
  return (
    <section aria-label={t('platform.integrations.routeGroups')} data-testid="route-groups">
      <Typography.Text strong>{t('platform.integrations.routeGroups')}</Typography.Text>
      <Table<RouteGroup>
        size="small"
        rowKey={(g) => `${g.gateway} ${g.prefix}`}
        dataSource={groups}
        pagination={false}
        locale={{ emptyText: t('platform.integrations.noData') }}
        columns={[
          { key: 'prefix', title: t('platform.integrations.routePrefix'), render: (_, g) => <Typography.Text code>{g.prefix}</Typography.Text> },
          { key: 'gateway', title: t('platform.integrations.routeGateway'), render: (_, g) => <Tag>{t(`platform.integrations.gatewayKind.${g.gateway}`)}</Tag> },
          { key: 'service', title: t('platform.integrations.routeService'), render: (_, g) => <Typography.Text code>{g.service}</Typography.Text> },
        ]}
      />
    </section>
  );
}

export function IntegrationOverviewTab({ enabled }: { enabled: boolean }) {
  const { t } = useTranslation();
  const overview = useIntegrationOverview(enabled);
  const o = overview.data;
  return (
    <section aria-label={t('platform.integrations.tabReference')}>
      <Typography.Paragraph type="secondary">{t('platform.integrations.asCode')}</Typography.Paragraph>
      {overview.error ? <ProblemMessage error={overview.error} /> : null}
      {o ? (
        <Space direction="vertical" size="middle">
          <Card title={t('platform.integrations.keycloak')} size="small">
            <Descriptions column={1} size="small">
              <Descriptions.Item label={t('platform.integrations.issuer')}>
                <Typography.Text code>{o.keycloak.issuer_url}</Typography.Text>
              </Descriptions.Item>
              <Descriptions.Item label={t('platform.integrations.realm')}>
                <Typography.Text code>{o.keycloak.realm}</Typography.Text>
              </Descriptions.Item>
              <Descriptions.Item label={t('platform.integrations.clients')}>
                <Space wrap>
                  {o.keycloak.clients.map((c) => (
                    <span key={c.client_id}>
                      <Typography.Text code>{c.client_id}</Typography.Text> <Tag>{t(`platform.integrations.clientPurpose.${c.purpose}`)}</Tag>
                    </span>
                  ))}
                </Space>
              </Descriptions.Item>
              <Descriptions.Item label={t('platform.integrations.platformMfa')}>{t(o.keycloak.platform_mfa_required ? 'platform.integrations.yes' : 'platform.integrations.no')}</Descriptions.Item>
              <Descriptions.Item label={t('platform.integrations.maxAuthAge')}>{t('platform.integrations.seconds', { count: o.keycloak.platform_auth_max_age_seconds })}</Descriptions.Item>
            </Descriptions>
          </Card>
          <Card title={t('platform.integrations.apisix')} size="small">
            <Descriptions column={1} size="small">
              <Descriptions.Item label={t('platform.integrations.publicGateway')}>
                <UrlValue url={o.apisix.public_base_url} />
              </Descriptions.Item>
              <Descriptions.Item label={t('platform.integrations.internalGateway')}>
                <UrlValue url={o.apisix.internal_base_url} />
              </Descriptions.Item>
            </Descriptions>
            <RouteGroups groups={o.apisix.route_groups ?? []} />
          </Card>
          <Card title={t('platform.integrations.tabSis')} size="small">
            <Descriptions column={1} size="small">
              <Descriptions.Item label={t('platform.integrations.connectionsTotal')}>{o.sis.connections_total}</Descriptions.Item>
              <Descriptions.Item label={t('platform.integrations.connectionsByStatus')}>
                <Space wrap>
                  {Object.entries(o.sis.connections_by_status).map(([status, count]) => (
                    <Tag key={status}>
                      {t(`platform.integrations.status.${status}`, { defaultValue: status })}: {count}
                    </Tag>
                  ))}
                </Space>
              </Descriptions.Item>
            </Descriptions>
          </Card>
          <Typography.Text type="secondary">{t('platform.integrations.generatedAt', { at: formatTime(o.generated_at) })}</Typography.Text>
        </Space>
      ) : null}
    </section>
  );
}
