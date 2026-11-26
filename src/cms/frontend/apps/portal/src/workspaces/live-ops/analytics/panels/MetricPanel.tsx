// Metric panels of Game Analytics (F080 T044, F080-AC-21, AC-22, AC-23): one DashboardWidget per panel of GAME_ANALYTICS
// §7.1 reading getGameMetrics for the panel's codes through the page queue (independent skeleton, error and meta line:
// grain label, "Cập nhật lúc", "Tạm tính", "Chưa có số liệu trước ngày …"). Without a dimension filter each metric is a
// tile with its latest server value (and the previous period when "So sánh kỳ trước" is on — a second request with the
// shifted range) plus a trend chart of the first metric; with a "cắt theo" filter the series per dimension value form a
// table. Suppressed points show the shared SuppressedCell ("< 5"); nothing is summed or recomputed client-side.
import { DashboardChart, DashboardMetric, DashboardWidget, SuppressedCell, formatMetric } from '@eduworld/ops-ui/dashboard';
import { Col, Flex, Row, Table, Typography } from 'antd';
import dayjs from 'dayjs';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useWidgetError } from '../../shared/ui';
import { useGameMetrics, type GameMetricSeries, type GameMetricsResponse } from '../api';
import { analyticsQuery, displayValue, PANEL_METRICS, previousRange, unitFormat, type AnalyticsFilters, type PanelKey } from '../model';

type Point = GameMetricSeries['points'][number];

/** Latest point carrying a value or a suppression (the server's last period). */
export function latestPoint(series: GameMetricSeries | undefined): Point | undefined {
  if (!series) return undefined;
  for (let i = series.points.length - 1; i >= 0; i--) {
    const p = series.points[i]!;
    if (p.suppressed || (p.value !== null && p.value !== undefined)) return p;
  }
  return undefined;
}

export function useMetricLabel() {
  const { t, i18n } = useTranslation();
  return (code: string) => (i18n.exists(`liveOps.metric.${code}`) ? t(`liveOps.metric.${code}`) : code);
}

/** Text of one server value in its unit (tables, chart labels). */
export function useFormatValue() {
  const { t, i18n } = useTranslation();
  return (value: number, unit: string | undefined): string => {
    const f = unitFormat(unit);
    const v = displayValue(value, unit);
    if (f === 'minutes') return t('liveOps.analytics.minutes', { value: formatMetric(v, 'number', i18n.language) });
    return formatMetric(v, f, i18n.language);
  };
}

/** Tiles of a set of metrics (latest value, optional previous period). */
export function MetricTiles({ codes, data, previous, partial }: { codes: string[]; data: GameMetricsResponse; previous?: GameMetricsResponse; partial: boolean }) {
  const { t } = useTranslation();
  const label = useMetricLabel();
  const format = useFormatValue();
  return (
    <Row gutter={[16, 8]}>
      {codes.map((code) => {
        const series = data.data.find((s) => s.metric_code === code && !s.dims);
        const point = latestPoint(series);
        const prevSeries = previous?.data.find((s) => s.metric_code === code && !s.dims);
        const prev = latestPoint(prevSeries);
        return (
          <Col key={code} xs={12} md={8} data-testid={`metric-${code}`}>
            <DashboardMetric
              label={label(code)}
              value={point?.value == null ? point?.value : displayValue(point.value, series?.unit)}
              format={unitFormat(series?.unit)}
              suppressed={!!point?.suppressed}
              partial={partial}
              testId={`metric-value-${code}`}
            />
            {previous ? (
              <Typography.Text type="secondary" data-testid={`metric-prev-${code}`}>
                {t('liveOps.analytics.previous')}: {prev?.suppressed ? <SuppressedCell /> : prev?.value != null ? format(prev.value, prevSeries?.unit) : t('liveOps.dashboard.noValue')}
              </Typography.Text>
            ) : null}
          </Col>
        );
      })}
    </Row>
  );
}

/** Table of series cut by one dimension: a row per dimension value, a column per metric (latest value). */
export function DimensionTable({ codes, data }: { codes: string[]; data: GameMetricsResponse }) {
  const { t } = useTranslation();
  const label = useMetricLabel();
  const format = useFormatValue();
  const keys = [...new Set(data.data.filter((s) => s.dims).map((s) => JSON.stringify(s.dims)))];
  const rows = keys.map((k) => ({ key: k, dims: JSON.parse(k) as Record<string, string> }));
  return (
    <Table
      size="small"
      pagination={false}
      scroll={{ x: true }}
      data-testid="dimension-table"
      dataSource={rows}
      columns={[
        { key: 'dim', title: t('liveOps.analytics.filter.dimension'), render: (_, r) => Object.values(r.dims).join(' · ') },
        ...codes.map((code) => ({
          key: code,
          title: label(code),
          render: (_: unknown, r: (typeof rows)[number]): ReactNode => {
            const s = data.data.find((x) => x.metric_code === code && JSON.stringify(x.dims) === r.key);
            const p = latestPoint(s);
            if (p?.suppressed) return <SuppressedCell />;
            return p?.value != null ? format(p.value, s?.unit) : '';
          },
        })),
      ]}
    />
  );
}

/** Trend of one series (line), with a textual summary for screen readers (DESIGN_SYSTEM §4.4). */
export function MetricTrend({ series }: { series: GameMetricSeries | undefined }) {
  const label = useMetricLabel();
  const format = useFormatValue();
  if (!series || series.points.length < 2) return null;
  const categories = series.points.map((p) => dayjs(p.period_start).format('DD/MM'));
  const values = series.points.map((p) => (p.suppressed || p.value == null ? null : displayValue(p.value, series.unit)));
  const summary = series.points.map((p, i) => `${categories[i]}: ${p.suppressed ? '< 5' : p.value != null ? format(p.value, series.unit) : '—'}`).join(', ');
  return <DashboardChart kind="line" categories={categories} values={values} ariaLabel={`${label(series.metric_code)} — ${summary}`} testId={`trend-${series.metric_code}`} />;
}

/** The meta line of a widget from an analytics response (range start for "Chưa có số liệu trước ngày …"). */
export const metaOf = (d: { grain: 'day' | 'week' | 'month'; data_as_of: string; is_partial: boolean; available_from?: string | null } | undefined, from: string) =>
  d ? { grain: d.grain, data_as_of: d.data_as_of, is_partial: d.is_partial, available_from: d.available_from ?? null, from } : undefined;

export function MetricPanel({ panel, filters, title }: { panel: PanelKey; filters: AnalyticsFilters; title: string }) {
  const { t } = useTranslation();
  const codes = PANEL_METRICS[panel];
  const q = useGameMetrics(panel, codes, analyticsQuery(filters));
  const prev = useGameMetrics(`${panel}-prev`, codes, analyticsQuery(filters, { range: previousRange(filters.range) }), filters.compare);
  const error = useWidgetError(q.error);
  const d = q.data;
  const cut = !!d?.data.some((s) => s.dims);
  return (
    <DashboardWidget testId={`panel-${panel}`} title={title} meta={metaOf(d, filters.range.from)} loading={q.isPending} error={error} empty={!!d && !d.data.length} emptyHint={t('liveOps.analytics.empty')}>
      {d ? (
        <Flex vertical gap="middle">
          {cut ? <DimensionTable codes={codes} data={d} /> : <MetricTiles codes={codes} data={d} previous={filters.compare ? prev.data : undefined} partial={d.is_partial} />}
          {!cut ? <MetricTrend series={d.data.find((s) => s.metric_code === codes[0])} /> : null}
        </Flex>
      ) : null}
    </DashboardWidget>
  );
}
