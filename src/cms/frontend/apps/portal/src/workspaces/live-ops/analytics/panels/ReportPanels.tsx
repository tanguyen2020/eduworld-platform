// Report panels of Game Analytics (F080 T044, F080-AC-21, F080-AC-23, GAME_ANALYTICS §7.1): Retention (cohort heatmap of
// getGameRetention — the cell shade comes from Admin theme tokens AND the rate is written in the cell, never colour
// alone; null = suppressed → "< 5"), Onboarding (getGameFunnel 'onboarding' + chapter0_completion) and Hiệu năng thiết
// bị (getGamePerformance, the only read that takes client versions). No duration column here: play time lives in
// Chơi lành mạnh only (AC-ANL-109).
import { DashboardWidget, SuppressedCell } from '@eduworld/ops-ui/dashboard';
import { Flex, Table, theme } from 'antd';
import dayjs from 'dayjs';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useWidgetError } from '../../shared/ui';
import { useGameFunnel, useGameMetrics, useGamePerformance, useGameRetention, type PerformanceReportRow, type RetentionMatrix } from '../api';
import { analyticsQuery, PANEL_METRICS, type AnalyticsFilters } from '../model';
import { metaOf, MetricTiles } from './MetricPanel';

function usePct() {
  const { i18n } = useTranslation();
  return (v: number | null | undefined) => (v == null ? '' : new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 1 }).format(v));
}
function useNum() {
  const { i18n } = useTranslation();
  return (v: number | null | undefined, digits = 1) => (v == null ? '' : new Intl.NumberFormat(i18n.language, { maximumFractionDigits: digits }).format(v));
}

type Cohort = RetentionMatrix['cohorts'][number];

export function RetentionPanel({ filters, title }: { filters: AnalyticsFilters; title: string }) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const pct = usePct();
  const num = useNum();
  const q = useGameRetention(analyticsQuery(filters));
  const error = useWidgetError(q.error);
  const d = q.data;
  const offsets = [...new Set((d?.cohorts ?? []).flatMap((c) => (c.retention ?? []).map((r) => r.day_offset ?? 0)))].sort((a, b) => a - b);
  // four shades of the primary colour tokens, light → strong
  const shades = [token.colorPrimaryBg, token.colorPrimaryBgHover, token.colorPrimaryBorder, token.colorPrimaryBorderHover];
  const shade = (rate: number) => shades[Math.min(shades.length - 1, Math.floor(rate * shades.length))];
  return (
    <DashboardWidget testId="panel-retention" title={title} meta={metaOf(d, filters.range.from)} loading={q.isPending} error={error} empty={!!d && !d.cohorts.length} emptyHint={t('liveOps.analytics.empty')}>
      {d ? (
        <Table<Cohort>
          size="small"
          pagination={false}
          scroll={{ x: true }}
          rowKey="cohort_date"
          data-testid="retention-heatmap"
          dataSource={d.cohorts}
          columns={[
            { key: 'cohort', title: t('liveOps.analytics.retention.cohort'), render: (_, c) => dayjs(c.cohort_date).format('DD/MM/YYYY') },
            { key: 'size', title: t('liveOps.analytics.retention.size'), render: (_, c) => (c.cohort_size === null ? <SuppressedCell /> : num(c.cohort_size, 0)) },
            ...offsets.map((o) => ({
              key: `d${o}`,
              title: t('liveOps.analytics.retention.day', { n: o }),
              onCell: (c: Cohort) => {
                const r = c.retention?.find((x) => x.day_offset === o);
                return r?.rate != null ? { style: { background: shade(r.rate) } } : {};
              },
              render: (_: unknown, c: Cohort): ReactNode => {
                const r = c.retention?.find((x) => x.day_offset === o);
                if (!r) return '';
                return r.rate === null ? <SuppressedCell /> : pct(r.rate);
              },
            })),
          ]}
        />
      ) : null}
    </DashboardWidget>
  );
}

export function OnboardingPanel({ filters, title }: { filters: AnalyticsFilters; title: string }) {
  const { t, i18n } = useTranslation();
  const pct = usePct();
  const num = useNum();
  const query = analyticsQuery(filters);
  const funnel = useGameFunnel('onboarding', query);
  const metric = useGameMetrics('onboarding', PANEL_METRICS.onboarding, query);
  const error = useWidgetError(funnel.error ?? metric.error);
  const d = funnel.data;
  const step = (code: string) => (i18n.exists(`liveOps.analytics.funnelStep.${code}`) ? t(`liveOps.analytics.funnelStep.${code}`) : code);
  return (
    <DashboardWidget testId="panel-onboarding" title={title} meta={metaOf(d, filters.range.from)} loading={funnel.isPending || metric.isPending} error={error} empty={!!d && !d.steps.length && !metric.data?.data.length} emptyHint={t('liveOps.analytics.empty')}>
      {d ? (
        <Flex vertical gap="middle">
          {metric.data ? <MetricTiles codes={PANEL_METRICS.onboarding} data={metric.data} partial={metric.data.is_partial} /> : null}
          <Table
            size="small"
            pagination={false}
            rowKey="step_code"
            data-testid="onboarding-funnel"
            dataSource={d.steps}
            columns={[
              { key: 'step', title: t('liveOps.analytics.funnel.step'), render: (_, s) => step(s.step_code) },
              { key: 'count', title: t('liveOps.analytics.funnel.count'), render: (_, s) => (s.count === null ? <SuppressedCell /> : num(s.count, 0)) },
              { key: 'conversion', title: t('liveOps.analytics.funnel.conversion'), render: (_, s) => (s.conversion_rate === null ? <SuppressedCell /> : pct(s.conversion_rate)) },
            ]}
          />
        </Flex>
      ) : null}
    </DashboardWidget>
  );
}

export function PerformancePanel({ filters, title }: { filters: AnalyticsFilters; title: string }) {
  const { t } = useTranslation();
  const pct = usePct();
  const num = useNum();
  const q = useGamePerformance(analyticsQuery(filters, { clientVersions: true }));
  const error = useWidgetError(q.error);
  const d = q.data;
  return (
    <DashboardWidget testId="panel-performance" title={title} meta={metaOf(d, filters.range.from)} loading={q.isPending} error={error} empty={!!d && !d.data.length} emptyHint={t('liveOps.analytics.empty')}>
      {d ? (
        <Table<PerformanceReportRow>
          size="small"
          pagination={false}
          scroll={{ x: true }}
          rowKey={(r) => `${r.platform}-${r.device_tier}-${r.preset ?? ''}-${r.renderer ?? ''}-${r.client_version ?? ''}`}
          data-testid="performance-table"
          dataSource={d.data}
          columns={[
            { key: 'platform', title: t('liveOps.analytics.perf.platform'), render: (_, r) => r.platform },
            { key: 'tier', title: t('liveOps.analytics.perf.tier'), render: (_, r) => t(`liveOps.analytics.dim.device_tier.${r.device_tier}`) },
            { key: 'preset', title: t('liveOps.analytics.perf.preset'), render: (_, r) => r.preset ?? '' },
            { key: 'renderer', title: t('liveOps.analytics.perf.renderer'), render: (_, r) => r.renderer ?? '' },
            { key: 'version', title: t('liveOps.analytics.perf.version'), render: (_, r) => r.client_version ?? '' },
            { key: 'sessions', title: t('liveOps.analytics.perf.sessions'), render: (_, r) => num(r.sessions, 0) },
            { key: 'fps50', title: t('liveOps.analytics.perf.fpsP50'), render: (_, r) => num(r.fps_p50) },
            { key: 'fps10', title: t('liveOps.analytics.perf.fpsP10'), render: (_, r) => num(r.fps_p10) },
            { key: 'budget', title: t('liveOps.analytics.perf.budget'), render: (_, r) => pct(r.fps_budget_pass_rate) },
            { key: 'load', title: t('liveOps.analytics.perf.load'), render: (_, r) => (r.load_time_p50_ms == null ? '' : t('liveOps.analytics.seconds', { value: num(r.load_time_p50_ms / 1000) })) },
            { key: 'crash', title: t('liveOps.analytics.perf.crashFree'), render: (_, r) => pct(r.crash_free_sessions) },
          ]}
        />
      ) : null}
    </DashboardWidget>
  );
}
