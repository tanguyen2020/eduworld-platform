// Screen of `menu.live_ops.live_dashboard` (F080 T040, F080-AC-19, F080-AC-20, ADMIN_PORTAL §3.14, §3.15): the menu,
// its path and permission (game.liveops.read) come from the generated catalog — the MenuGate shows the 403 page and never
// mounts this screen for anyone else (Safety Moderator, tenant roles: F080-AC-24). Two tabs, each shown only when its
// catalog action is granted and turned on (ADR-0090): "Tổng quan" (getLiveOpsOverview, auto refresh every 20 s,
// "Cập nhật lúc HH:mm:ss" from `as_of`; a refused/failed read shows the error and NO numbers) and "Giám sát sự kiện".
import { Alert, Flex, Spin, Tabs, Typography } from 'antd';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';
import { TenantFilter } from '../../../shell/TenantControls';
import { LIVE_REFRESH_MS, useLiveOpsOverview } from '../api/liveops';
import { EventMonitorPage } from '../events/EventMonitorPage';
import { LiveOpsError, PageTitle, useActionEnabled } from '../shared/ui';
import { KpiCards } from './KpiCards';

function OverviewTab() {
  const { t } = useTranslation();
  const q = useLiveOpsOverview();
  let body;
  if (q.isPending) body = <Spin aria-label={t('app.loading')} />;
  else if (q.isError) body = <LiveOpsError error={q.error} />;
  else if (!q.data.kpis.length) body = <Alert type="info" showIcon message={t('liveOps.dashboard.empty')} />;
  else body = <KpiCards kpis={q.data.kpis} />;
  return (
    <Flex vertical gap="middle" data-testid="live-overview">
      <Flex wrap gap="middle" align="center">
        <TenantFilter />
        {q.data && !q.isError ? (
          <Typography.Text type="secondary" data-testid="live-as-of">
            {t('liveOps.dashboard.updatedAt', { time: dayjs(q.data.as_of).format('HH:mm:ss') })}
          </Typography.Text>
        ) : null}
        <Typography.Text type="secondary">{t('liveOps.dashboard.refreshEvery', { seconds: LIVE_REFRESH_MS / 1000 })}</Typography.Text>
      </Flex>
      {body}
    </Flex>
  );
}

export function LiveDashboardScreen({ title }: { title: string }) {
  const { t } = useTranslation();
  const canView = useActionEnabled('action.live_ops.live_dashboard.view');
  const canStats = useActionEnabled('action.game_ops.events.stats');
  const items = [
    ...(canView ? [{ key: 'overview', label: t('liveOps.dashboard.tab'), children: <OverviewTab /> }] : []),
    ...(canStats ? [{ key: 'events', label: t('liveOps.events.tab'), children: <EventMonitorPage /> }] : []),
  ];
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      {items.length ? <Tabs items={items} destroyOnHidden /> : <Alert type="info" showIcon message={t('shell.featureDisabled.title')} />}
    </section>
  );
}
