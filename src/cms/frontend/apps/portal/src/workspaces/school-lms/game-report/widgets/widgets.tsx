// Widgets of "Hoạt động game" (F080 T061, F080-AC-28, AC-29, GAME_ANALYTICS §7.2): Tổng quan, Tiến trình, Xã hội, Chơi
// lành mạnh, Thiết bị — each its own DashboardWidget with its own loading/error/empty state (one shared summary query).
// Every number is the server's: `null` = cell of fewer than 5 students → the shared SuppressedCell "< 5" (never summed or
// derived client-side). Play time appears only as the median session length (Tổng quan) and as wellbeing rates; there is
// no economy number, no student list and no "total play time" (AC-ANL-109, AGENTS R16, R19). The device widget says
// "Không có dữ liệu" when the tenant turned product telemetry off (no distribution returned).
import { DashboardChart, DashboardMetric, DashboardWidget, SuppressedCell, type DashboardWidgetError } from '@eduworld/ops-ui/dashboard';
import { Col, Empty, Flex, Row, Table, Typography } from 'antd';
import dayjs from 'dayjs';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import type { TenantGameSummary } from '../api/tenantGameSummary';

/** Prefix of the keys of this screen (trailing dot: the i18n key check treats it as a prefix). */
const K = 'portal.school_lms.game_report.';

export interface WidgetProps {
  data: TenantGameSummary | undefined;
  loading: boolean;
  error: DashboardWidgetError | null;
}

const metaOf = (d: TenantGameSummary | undefined) => (d ? { data_as_of: d.data_as_of, grain: d.grain, is_partial: d.is_partial, from: d.from, available_from: d.available_from ?? null } : undefined);

/** One number of the summary: `null` = suppressed ("< 5"), absent = no data yet. */
function Metric({ label, value, format = 'number', d, testId }: { label: string; value: number | null | undefined; format?: 'number' | 'percent' | 'minutes'; d: TenantGameSummary; testId: string }) {
  return <DashboardMetric label={label} value={value} format={format} suppressed={value === null} partial={d.is_partial} testId={testId} />;
}

function Frame({ testId, title, props, children }: { testId: string; title: string; props: WidgetProps; children: (d: TenantGameSummary) => ReactNode }) {
  return (
    <DashboardWidget testId={testId} title={title} meta={metaOf(props.data)} loading={props.loading} error={props.error}>
      {props.data ? children(props.data) : null}
    </DashboardWidget>
  );
}

function usePct() {
  const { i18n } = useTranslation();
  return (v: number | null | undefined): ReactNode => (v === null ? <SuppressedCell /> : v === undefined ? '' : new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 0 }).format(v));
}
function useNum() {
  const { i18n } = useTranslation();
  return (v: number | null | undefined): ReactNode => (v === null ? <SuppressedCell /> : v === undefined ? '' : new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 1 }).format(v));
}

export function OverviewWidget(props: WidgetProps) {
  const { t } = useTranslation();
  return (
    <Frame testId="game-widget-overview" title={t(`${K}overview.title`)} props={props}>
      {(d) => {
        const series = d.active_students_series ?? [];
        return (
          <Flex vertical gap="middle">
            <Row gutter={[16, 8]}>
              <Col xs={12} md={8}>
                <Metric d={d} label={t(`${K}overview.activeStudents`)} value={d.active_students} testId="gm-active-students" />
              </Col>
              <Col xs={12} md={8}>
                <Metric d={d} label={t(`${K}overview.dauAvg`)} value={d.dau_avg} testId="gm-dau-avg" />
              </Col>
              <Col xs={12} md={8}>
                <Metric d={d} label={t(`${K}overview.retentionD7`)} value={d.retention_d7} format="percent" testId="gm-retention-d7" />
              </Col>
              <Col xs={12} md={8}>
                <Metric d={d} label={t(`${K}overview.activeDays3`)} value={d.active_days_3plus_rate} format="percent" testId="gm-active-days" />
              </Col>
              <Col xs={12} md={8}>
                <Metric
                  d={d}
                  label={t(`${K}overview.sessionMedian`)}
                  value={d.session_duration_p50_seconds == null ? d.session_duration_p50_seconds : d.session_duration_p50_seconds / 60}
                  format="minutes"
                  testId="gm-session-median"
                />
              </Col>
              <Col xs={12} md={8}>
                <Metric d={d} label={t(`${K}overview.chapter0`)} value={d.chapter0_completion_rate} format="percent" testId="gm-chapter0" />
              </Col>
            </Row>
            {series.length > 1 ? (
              <DashboardChart
                kind="line"
                categories={series.map((p) => dayjs(p.t).format('DD/MM'))}
                values={series.map((p) => p.value ?? null)}
                ariaLabel={`${t(`${K}overview.activeStudents`)} — ${series.map((p) => `${dayjs(p.t).format('DD/MM')}: ${p.value === null ? '< 5' : (p.value ?? '—')}`).join(', ')}`}
                testId="gm-active-series"
              />
            ) : null}
            {series.some((p) => p.value === null) ? (
              <Typography.Text type="secondary">
                {t(`${K}overview.seriesSuppressed`)} <SuppressedCell />
              </Typography.Text>
            ) : null}
          </Flex>
        );
      }}
    </Frame>
  );
}

export function ProgressionWidget(props: WidgetProps & { schoolName: (id?: string) => string }) {
  const { t } = useTranslation();
  const pct = usePct();
  const num = useNum();
  return (
    <Frame testId="game-widget-progression" title={t(`${K}progression.title`)} props={props}>
      {(d) => (
        <Flex vertical gap="middle">
          <Row gutter={[16, 8]}>
            <Col xs={12}>
              <Metric d={d} label={t(`${K}progression.questCompletion`)} value={d.quest_completion_rate} format="percent" testId="gm-quest-completion" />
            </Col>
            <Col xs={12}>
              <Metric d={d} label={t(`${K}progression.eventParticipation`)} value={d.event_participation_rate} format="percent" testId="gm-event-participation" />
            </Col>
          </Row>
          {d.by_school?.length ? (
            <Table
              size="small"
              pagination={false}
              scroll={{ x: true }}
              data-testid="gm-by-school"
              rowKey={(r) => r.school_id ?? ''}
              dataSource={d.by_school}
              columns={[
                { key: 'school', title: t('portal.school_lms.common.school'), render: (_, r) => props.schoolName(r.school_id) },
                { key: 'active', title: t(`${K}overview.activeStudents`), render: (_, r) => num(r.active_students) },
                { key: 'quest', title: t(`${K}progression.questCompletion`), render: (_, r) => pct(r.quest_completion_rate) },
              ]}
            />
          ) : null}
          {d.by_grade_band?.length ? (
            <Table
              size="small"
              pagination={false}
              scroll={{ x: true }}
              data-testid="gm-by-grade-band"
              rowKey="grade_band"
              dataSource={d.by_grade_band}
              columns={[
                { key: 'band', title: t(`${K}filter.gradeBand`), render: (_, r) => t(`${K}gradeBand.${r.grade_band}`) },
                { key: 'active', title: t(`${K}overview.activeStudents`), render: (_, r) => num(r.active_students) },
                { key: 'quest', title: t(`${K}progression.questCompletion`), render: (_, r) => pct(r.quest_completion_rate) },
              ]}
            />
          ) : null}
        </Flex>
      )}
    </Frame>
  );
}

export function SocialWidget(props: WidgetProps) {
  const { t } = useTranslation();
  return (
    <Frame testId="game-widget-social" title={t(`${K}social.title`)} props={props}>
      {(d) => <Metric d={d} label={t(`${K}social.socialPlay`)} value={d.social_play_rate} format="percent" testId="gm-social-play" />}
    </Frame>
  );
}

export function WellbeingWidget(props: WidgetProps) {
  const { t } = useTranslation();
  return (
    <Frame testId="game-widget-wellbeing" title={t(`${K}wellbeing.title`)} props={props}>
      {(d) => (
        <Row gutter={[16, 8]}>
          <Col xs={12} md={8}>
            <Metric d={d} label={t(`${K}wellbeing.overReminder`)} value={d.sessions_over_reminder_rate} format="percent" testId="gm-over-reminder" />
          </Col>
          <Col xs={12} md={8}>
            <Metric d={d} label={t(`${K}wellbeing.overSecondReminder`)} value={d.sessions_over_second_reminder_rate} format="percent" testId="gm-over-second-reminder" />
          </Col>
          <Col xs={12} md={8}>
            <Metric d={d} label={t(`${K}wellbeing.lateNight`)} value={d.late_night_play_rate} format="percent" testId="gm-late-night" />
          </Col>
        </Row>
      )}
    </Frame>
  );
}

export function DeviceWidget(props: WidgetProps) {
  const { t } = useTranslation();
  const pct = usePct();
  return (
    <Frame testId="game-widget-device" title={t(`${K}device.title`)} props={props}>
      {(d) => {
        const platforms = d.platform_distribution ?? [];
        const tiers = d.device_tier_distribution ?? [];
        if (!platforms.length && !tiers.length) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t(`${K}device.noData`)} />;
        return (
          <Row gutter={[16, 8]}>
            <Col xs={24} md={12}>
              <Typography.Text strong>{t(`${K}device.platform`)}</Typography.Text>
              <ul data-testid="gm-platforms">
                {platforms.map((p) => (
                  <li key={p.key}>
                    {t(`${K}platform.${p.key}`)}: {pct(p.share)}
                  </li>
                ))}
              </ul>
            </Col>
            <Col xs={24} md={12}>
              <Typography.Text strong>{t(`${K}device.tier`)}</Typography.Text>
              <ul data-testid="gm-tiers">
                {tiers.map((p) => (
                  <li key={p.key}>
                    {t(`${K}tier.${p.key}`, { defaultValue: p.key })}: {pct(p.share)}
                  </li>
                ))}
              </ul>
            </Col>
          </Row>
        );
      }}
    </Frame>
  );
}
