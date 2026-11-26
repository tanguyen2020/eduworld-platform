// KPI cards of the Live dashboard (F080 T040, F080-AC-20, DESIGN_SYSTEM §4.3): one card per KPI the server returns, in
// its order (unknown codes rendered generically). The status gives the top border colour from the Admin theme tokens
// (DashboardWidget accent) AND an icon + label, never colour alone (DESIGN_SYSTEM §4.4). A missing value is "Không có
// dữ liệu", never 0. When "báo cáo chờ xử lý" is at warning/incident and the user sees the Kiểm duyệt menu, the card
// links to it. Presentational only: every number and status comes from module game (AGENTS R1).
import type { Accent } from '@eduworld/ops-ui';
import { DashboardWidget } from '@eduworld/ops-ui/dashboard';
import { CheckCircleOutlined, CloseCircleOutlined, QuestionCircleOutlined, WarningOutlined } from '@ant-design/icons';
import { Col, Flex, Row, Statistic, Tag, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { menuByKey } from '../../../permissions/menuRegistry';
import type { KpiStatus, LiveOpsKpi } from '../api/liveops';
import { useMenuVisible } from '../shared/ui';

const ACCENT: Record<KpiStatus, Accent | undefined> = { normal: 'success', warning: 'warning', incident: 'error', unknown: undefined };
const TAG: Record<KpiStatus, string> = { normal: 'success', warning: 'warning', incident: 'error', unknown: 'default' };
const ICON: Record<KpiStatus, ReactNode> = {
  normal: <CheckCircleOutlined />,
  warning: <WarningOutlined />,
  incident: <CloseCircleOutlined />,
  unknown: <QuestionCircleOutlined />,
};

const MODERATION = 'menu.live_ops.moderation_cases' as const;

/** Formatted value of a KPI in the UI language with its unit; null → undefined (the card says "Không có dữ liệu"). */
export function useKpiValue() {
  const { t, i18n } = useTranslation();
  return (value: number | null | undefined, unit: string | undefined): string | undefined => {
    if (value === null || value === undefined) return undefined;
    const u = (unit ?? '').toLowerCase();
    if (u === 'ratio') return new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 1 }).format(value);
    const n = new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 2 }).format(value);
    return u && i18n.exists(`liveOps.unit.${u}`) ? t(`liveOps.unit.${u}`, { value: n }) : n;
  };
}

function KpiCard({ kpi }: { kpi: LiveOpsKpi }) {
  const { t, i18n } = useTranslation();
  const format = useKpiValue();
  const moderationVisible = useMenuVisible(MODERATION);
  const label = i18n.exists(`liveOps.kpi.${kpi.code}`) ? t(`liveOps.kpi.${kpi.code}`) : kpi.code;
  const value = format(kpi.value, kpi.unit);
  const alert = kpi.status === 'warning' || kpi.status === 'incident';
  return (
    <DashboardWidget title={label} testId={`kpi-${kpi.code}`} accent={ACCENT[kpi.status]}>
      <Statistic
        valueRender={() => <span data-testid={`kpi-value-${kpi.code}`}>{value ?? <Typography.Text type="secondary">{t('liveOps.dashboard.noValue')}</Typography.Text>}</span>}
      />
      <Flex wrap gap="small" align="center">
        <Tag color={TAG[kpi.status]} icon={ICON[kpi.status]} data-status={kpi.status}>
          {t(`liveOps.status.${kpi.status}`)}
        </Tag>
        {kpi.threshold_warning != null || kpi.threshold_incident != null ? (
          <Typography.Text type="secondary">
            {t('liveOps.dashboard.thresholds', { warning: format(kpi.threshold_warning, kpi.unit) ?? '—', incident: format(kpi.threshold_incident, kpi.unit) ?? '—' })}
          </Typography.Text>
        ) : null}
      </Flex>
      {kpi.breakdown?.length ? (
        <ul aria-label={t('liveOps.dashboard.breakdown', { kpi: label })}>
          {kpi.breakdown.map((b) => (
            <li key={b.label}>
              <Typography.Text type="secondary">
                {b.label}: {format(b.value, kpi.unit) ?? t('liveOps.dashboard.noValue')}
              </Typography.Text>
            </li>
          ))}
        </ul>
      ) : null}
      {kpi.code === 'reports_pending' && alert && moderationVisible ? (
        <Link to={menuByKey(MODERATION)!.path} data-testid="kpi-moderation-link">
          {t('liveOps.dashboard.openModeration')}
        </Link>
      ) : null}
    </DashboardWidget>
  );
}

export function KpiCards({ kpis }: { kpis: LiveOpsKpi[] }) {
  return (
    <Row gutter={[16, 16]} data-testid="kpi-cards">
      {kpis.map((k) => (
        <Col key={k.code} xs={24} md={12} xl={6}>
          <KpiCard kpi={k} />
        </Col>
      ))}
    </Row>
  );
}
