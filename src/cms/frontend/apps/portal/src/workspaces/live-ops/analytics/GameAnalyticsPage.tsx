// Screen of `menu.live_ops.game_analytics` (F080 T044, F080-AC-21…AC-23, GAME_ANALYTICS §7.1, ADMIN_PORTAL §3.14): the
// menu and its permission (game.analytics.read) come from the generated catalog — the MenuGate shows the 403 page to
// anyone else (F080-AC-24). The 11 panels are independent widgets sharing one DashboardQueue (≤ 4 requests in flight);
// panels are shown only while `action.live_ops.game_analytics.view` is granted and on. "Xuất CSV" (behind
// `<ActionGate actionKey="action.live_ops.game_analytics.export">`) creates an asynchronous job for the chosen panel and
// notifies when the link is ready. On a phone only Tổng quan and Chơi lành mạnh are shown, read-only (ADMIN_PORTAL §3.14).
import { SearchSelect } from '@eduworld/ops-ui';
import { DashboardQueueProvider, ExportButton, presetRange, type DashboardRange } from '@eduworld/ops-ui/dashboard';
import { App as AntApp, Alert, Col, Flex, Row } from 'antd';
import dayjs from 'dayjs';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ApiError } from '../../../shared/api/middleware/problem';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { useTenantFilter } from '../../../shell/tenantFilterStore';
import { DASHBOARD_CONFIG } from '../limits';
import { PageTitle, useActionEnabled } from '../shared/ui';
import { useAnalyticsExport } from './api';
import { AnalyticsFilterBar } from './AnalyticsFilterBar';
import { analyticsQuery, exportOf, PANELS, type AnalyticsFilters, type PanelKey } from './model';
import { ContentPanel } from './panels/ContentPanel';
import { MetricPanel } from './panels/MetricPanel';
import { OnboardingPanel, PerformancePanel, RetentionPanel } from './panels/ReportPanels';

/** Phone layout limit (ADMIN_PORTAL §1.5: `md` 768 px). */
const DESKTOP_QUERY = '(min-width: 768px)';
const MOBILE_PANELS: PanelKey[] = ['overview', 'wellbeing'];

export function defaultAnalyticsRange(today = dayjs().format('YYYY-MM-DD')): DashboardRange {
  return { preset: '30d', ...presetRange('30d', today)! };
}

function Panel({ panel, filters }: { panel: PanelKey; filters: AnalyticsFilters }) {
  const { t } = useTranslation();
  const title = t(`liveOps.analytics.panel.${panel}`);
  switch (panel) {
    case 'retention':
      return <RetentionPanel filters={filters} title={title} />;
    case 'onboarding':
      return <OnboardingPanel filters={filters} title={title} />;
    case 'quests':
    case 'challenges':
      return <ContentPanel panel={panel} filters={filters} title={title} />;
    case 'performance':
      return <PerformancePanel filters={filters} title={title} />;
    default:
      return <MetricPanel panel={panel} filters={filters} title={title} />;
  }
}

function AnalyticsExport({ filters }: { filters: AnalyticsFilters }) {
  const { t } = useTranslation();
  const { notification } = AntApp.useApp();
  const [panel, setPanel] = useState<PanelKey>('overview');
  const exp = useAnalyticsExport();
  const notified = useRef<string | null>(null);
  useEffect(() => {
    if (exp.job?.status === 'succeeded' && notified.current !== exp.job.job_id) {
      notified.current = exp.job.job_id;
      notification.success({ message: t('liveOps.analytics.export.ready') });
    }
  }, [exp.job, notification, t]);
  const { report, params } = exportOf(panel);
  const errorMessage =
    exp.error instanceof ApiError ? (exp.error.code === 'EXPORT_LIMIT_REACHED' ? t('liveOps.analytics.export.limit') : t(exp.error.i18nKey)) : exp.error ? t('error.generic') : undefined;
  const job = exp.job ? { status: exp.job.status, download_url: exp.job.download_url, download_expires_at: exp.job.expires_at, error: exp.job.error_code } : null;
  return (
    <Flex wrap gap="small" align="start" data-testid="analytics-export">
      <SearchSelect<PanelKey>
        aria-label={t('liveOps.analytics.export.panel')}
        value={panel}
        onChange={setPanel}
        options={PANELS.map((p) => ({ value: p, label: t(`liveOps.analytics.panel.${p}`) }))}
        popupMatchSelectWidth={false}
      />
      <ExportButton
        label={t('liveOps.analytics.export.button')}
        pending={exp.pending}
        job={job}
        errorMessage={errorMessage}
        onExport={() => exp.create({ report, format: 'csv', params: { ...analyticsQuery(filters, { clientVersions: report === 'performance' }), ...params } })}
      />
    </Flex>
  );
}

export function GameAnalyticsScreen({ title }: { title: string }) {
  const { t } = useTranslation();
  const desktop = useMediaQuery(DESKTOP_QUERY);
  const canView = useActionEnabled('action.live_ops.game_analytics.view');
  const tenantIds = useTenantFilter();
  const [filters, setFilters] = useState<Omit<AnalyticsFilters, 'tenantIds'>>({ range: defaultAnalyticsRange(), values: [], clientVersions: [], compare: false });
  const effective: AnalyticsFilters = { ...filters, tenantIds };
  const panels = desktop ? PANELS : MOBILE_PANELS;
  return (
    <section aria-labelledby="ew-menu-title">
      <Flex vertical gap="middle">
        <PageTitle
          title={title}
          extra={
            desktop && canView ? (
              <ActionGate actionKey="action.live_ops.game_analytics.export">
                <AnalyticsExport filters={effective} />
              </ActionGate>
            ) : null
          }
        />
        {!canView ? (
          <Alert type="info" showIcon message={t('shell.featureDisabled.title')} />
        ) : (
          <>
            {desktop ? <AnalyticsFilterBar value={effective} onChange={(next) => setFilters({ range: next.range, dimension: next.dimension, values: next.values, clientVersions: next.clientVersions, compare: next.compare })} /> : null}
            <DashboardQueueProvider limit={DASHBOARD_CONFIG.dashboard_max_concurrent_per_user}>
              <Row gutter={[16, 16]} data-testid="analytics-panels">
                {panels.map((p) => (
                  <Col key={p} xs={24} xl={p === 'retention' || p === 'performance' || p === 'quests' || p === 'challenges' ? 24 : 12}>
                    <Panel panel={p} filters={effective} />
                  </Col>
                ))}
              </Row>
            </DashboardQueueProvider>
          </>
        )}
      </Flex>
    </section>
  );
}
