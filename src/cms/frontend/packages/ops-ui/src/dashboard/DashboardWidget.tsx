// DashboardWidget (F056 T008, F056-AC-02, F056-AC-04, ADMIN_PORTAL §3.14.1): frame of one dashboard widget — own skeleton
// (widgets load independently), "Cập nhật lúc HH:mm" from `data_as_of`, the grain label the server used, "Tạm tính"
// when `is_partial`, an empty state with guidance, and the refused-query hints by Problem `code`
// (DASHBOARD_RANGE_TOO_LARGE → narrow the range, DASHBOARD_QUERY_TIMEOUT → narrow the scope, DASHBOARD_BUSY → busy).
// DashboardMetric shows one number: "Đang tính số liệu…" instead of 0 while the period is not closed, "< 5" when
// suppressed. Presentational only: every number comes from the server (AGENTS R1, ADR-0062).
import { Alert, Card, Empty, Flex, Skeleton, Statistic, Tag, Typography } from 'antd';
import dayjs from 'dayjs';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';
import { SuppressedCell } from './SuppressedCell';

export type DashboardGrain = 'day' | 'week' | 'month';

/** `DashboardMeta` of the API (DASHBOARD_DATA §8). */
export interface DashboardMetaView {
  data_as_of: string;
  grain: DashboardGrain;
  is_partial: boolean;
}

/** Problem `code`s a widget explains itself; any other error shows `message`. */
export const DASHBOARD_HINT_CODES = ['DASHBOARD_RANGE_TOO_LARGE', 'DASHBOARD_QUERY_TIMEOUT', 'DASHBOARD_BUSY'] as const;

export interface DashboardWidgetError {
  code: string;
  /** Translated message for codes outside DASHBOARD_HINT_CODES (403/404/…), with the trace id if any. */
  message?: ReactNode;
}

export interface DashboardWidgetProps {
  title: ReactNode;
  testId?: string;
  meta?: DashboardMetaView;
  loading?: boolean;
  error?: DashboardWidgetError | null;
  /** No data yet (e.g. start of the school year): guidance instead of zeros. */
  empty?: boolean;
  emptyHint?: ReactNode;
  extra?: ReactNode;
  children?: ReactNode;
}

const hintKey: Record<(typeof DASHBOARD_HINT_CODES)[number], string> = {
  DASHBOARD_RANGE_TOO_LARGE: 'dashboard.error.rangeTooLarge',
  DASHBOARD_QUERY_TIMEOUT: 'dashboard.error.queryTimeout',
  DASHBOARD_BUSY: 'dashboard.error.busy',
};

/** "Cập nhật lúc HH:mm" · grain · "Tạm tính" (F056-AC-02). */
function DashboardMetaLine({ meta }: { meta: DashboardMetaView }) {
  const { t } = useTranslation(OPS_UI_NS);
  return (
    <Flex wrap gap="small" align="center" data-testid="dashboard-meta">
      <Typography.Text type="secondary">{t('dashboard.meta.updatedAt', { time: dayjs(meta.data_as_of).format('HH:mm') })}</Typography.Text>
      <Tag>{t(`dashboard.grain.${meta.grain}`)}</Tag>
      {meta.is_partial ? <Tag color="warning">{t('dashboard.meta.partial')}</Tag> : null}
    </Flex>
  );
}

export function DashboardWidget({ title, testId, meta, loading, error, empty, emptyHint, extra, children }: DashboardWidgetProps) {
  const { t } = useTranslation(OPS_UI_NS);
  let body: ReactNode;
  if (loading)
    body = (
      <div data-testid="dashboard-widget-skeleton">
        <Skeleton active paragraph={{ rows: 3 }} />
      </div>
    );
  else if (error) {
    const key = (DASHBOARD_HINT_CODES as readonly string[]).includes(error.code) ? hintKey[error.code as keyof typeof hintKey] : undefined;
    body = <Alert type={error.code === 'DASHBOARD_BUSY' ? 'warning' : 'error'} showIcon role="alert" message={key ? t(key) : (error.message ?? t('dashboard.error.generic'))} />;
  } else if (empty) body = <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyHint ?? t('dashboard.empty')} />;
  else body = children;
  return (
    <Card title={title} extra={extra} data-testid={testId} aria-busy={loading ? true : undefined}>
      <Flex vertical gap="small">
        {meta && !loading && !error ? <DashboardMetaLine meta={meta} /> : null}
        {body}
      </Flex>
    </Card>
  );
}

export type MetricFormat = 'number' | 'percent' | 'minutes';

export interface DashboardMetricProps {
  label: ReactNode;
  value: number | null | undefined;
  format?: MetricFormat;
  /** The period is not closed yet (`is_partial`): a missing value is "being computed", never 0. */
  partial?: boolean;
  suppressed?: boolean;
  testId?: string;
}

/** Formats a server number for display (vi-VN by default); ratios are 0..1. */
export function formatMetric(value: number, format: MetricFormat, locale: string): string {
  if (format === 'percent') return new Intl.NumberFormat(locale, { style: 'percent', maximumFractionDigits: 0 }).format(value);
  return new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(value);
}

export function DashboardMetric({ label, value, format = 'number', partial, suppressed, testId }: DashboardMetricProps) {
  const { t, i18n } = useTranslation(OPS_UI_NS);
  let shown: ReactNode;
  if (suppressed) shown = <SuppressedCell />;
  else if (value === null || value === undefined) shown = <Typography.Text type="secondary">{partial ? t('dashboard.metric.computing') : t('dashboard.metric.noData')}</Typography.Text>;
  else shown = format === 'minutes' ? t('dashboard.metric.minutes', { value: formatMetric(value, 'number', i18n.language) }) : formatMetric(value, format, i18n.language);
  return <Statistic title={label} valueRender={() => <span data-testid={testId}>{shown}</span>} />;
}
