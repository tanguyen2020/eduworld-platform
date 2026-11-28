// Overview of the School Learning Dashboard (F056 T020; F056-AC-01, AC-02, AC-04, AC-05, AC-07; SCHOOL_LMS §8.1, §8.7):
// one getSchoolDashboard call (school_id, from, to, grade/subject) answers the 5 weekly questions, shown in this
// order: ① open / due-soon assignments, ② LOs to consolidate, ③ students needing support, ④ self-study after
// recommendations, ⑤ safety notifications (count only + link, never chat content). Every widget has its own skeleton,
// "Cập nhật lúc", grain and "Tạm tính" (ops-ui DashboardWidget); numbers are the server's (AGENTS R1, ADR-0062).
// Students are never ranked by score (no score column, server order kept). Mobile: widgets stack as cards. Drill-down
// is school → grade (grade page, getGradeDashboard) → student (ADR-0095): grades of the school with their completion and
// accuracy, progress of each assignment by grade, the grades of each weak LO; cells under the small-cell threshold are
// shown "< 5" (`suppressed`, FR-ANL-016). No class anywhere.
// Look (DESIGN_SYSTEM §4.2, direction B "Sổ tay giáo viên", owner 2026-11-21): greeting by time of day, prominent
// primary action "+ Giao bài mới", each widget a KPI card with a 4 px accent top border — no number is invented here.
import type { Accent } from '@eduworld/ops-ui';
import { DashboardChart, DashboardMetric, DashboardWidget, SuppressedCell, type DashboardMetaView } from '@eduworld/ops-ui/dashboard';
import { PlusOutlined } from '@ant-design/icons';
import { Alert, Button, Col, Flex, List, Progress, Row, Space, Tag, Typography } from 'antd';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from 'react-router-dom';
import { ActionGate } from '../../../../permissions/ActionGate';
import { menuStatus } from '../../../../permissions/menuRegistry';
import { usePermissions } from '../../../../permissions/usePermissions';
import { formatDateTime } from '../../common/format';
import { useSchoolDashboard, type SchoolDashboard } from '../api/dashboardApi';
import { DASHBOARD_BASE, practiceWizardHref, ReasonTag, SAFETY_BASE, useWidgetError, ASSIGNMENTS_BASE } from '../DashboardUi';
import { DashboardFilterBar, useDashboardFilters } from '../DashboardFilterBar';
import { filtersToSearch, type DashboardFilters } from '../model/filters';

type Widgets = SchoolDashboard['widgets'];

const metaOf = (d: SchoolDashboard | undefined): DashboardMetaView | undefined =>
  d ? { data_as_of: d.data_as_of, grain: d.grain, is_partial: d.is_partial, from: d.from, available_from: d.available_from, replica_lag_warning: d.replica_lag_warning } : undefined;

/** Link to the page of one grade keeping school, subject and range (F056-AC-08). */
export function gradeHref(f: DashboardFilters, grade: number): string {
  return `${DASHBOARD_BASE}/grades/${grade}?${filtersToSearch({ ...f, grade }).toString()}`;
}

/** Link to a student page keeping school, grade, subject and range (F056-AC-08). */
export function studentHref(f: DashboardFilters, studentId: string): string {
  return `${DASHBOARD_BASE}/students/${studentId}?${filtersToSearch(f).toString()}`;
}

export type Greeting = 'morning' | 'noon' | 'afternoon' | 'evening';

/** Greeting of the overview header by the viewer's local hour (presentation only). */
export function greetingOf(hour: number): Greeting {
  if (hour < 11) return 'morning';
  if (hour < 13) return 'noon';
  if (hour < 18) return 'afternoon';
  return 'evening';
}

/** Accent of each widget card (DESIGN_SYSTEM §4.1 "Thẻ KPI/widget"): semantic colours of the theme. */
export const WIDGET_ACCENTS: Record<'assignments' | 'weak' | 'attention' | 'selfStudy' | 'safety', Accent> = {
  assignments: 'primary',
  weak: 'color.subject.english',
  attention: 'warning',
  selfStudy: 'success',
  safety: 'info',
};

function AssignmentsWidget({ w, f, partial }: { w: Widgets; f: DashboardFilters; partial: boolean }) {
  const { t } = useTranslation();
  const gradeLabel = (g: number) => (g ? t('dashboard.filter.gradeN', { n: g }) : t('dashboard.grade.unknown'));
  return (
    <Flex vertical gap="middle">
      <Row gutter={[16, 8]}>
        <Col xs={12} md={8}>
          <DashboardMetric label={t('dashboard.assignments.open')} value={w.assignments_open} partial={partial} testId="metric-assignments-open" />
        </Col>
        <Col xs={12} md={8}>
          <DashboardMetric label={t('dashboard.assignments.dueSoon')} value={w.assignments_due_soon} partial={partial} testId="metric-due-soon" />
        </Col>
        <Col xs={24} md={8}>
          <DashboardMetric label={t('dashboard.assignments.completion')} value={w.assignment_completion_rate} format="percent" partial={partial} testId="metric-completion" />
        </Col>
      </Row>
      <List
        size="small"
        dataSource={w.assignments ?? []}
        locale={{ emptyText: t('dashboard.assignments.none') }}
        renderItem={(a) => (
          <List.Item key={a.assignment_id}>
            <Flex vertical gap="small" style={{ width: '100%' }}>
              <Typography.Text strong>{t('dashboard.assignments.item', { title: a.title, due: formatDateTime(a.due_at) })}</Typography.Text>
              {a.total ? (
                <Progress
                  percent={Math.round((a.submitted / a.total) * 100)}
                  size="small"
                  // accessible name of the progressbar (axe aria-progressbar-name, WCAG 4.1.2)
                  aria-label={t('dashboard.assignments.progressLabel', { title: a.title, submitted: a.submitted, total: a.total })}
                  format={() => t('dashboard.assignments.submitted', { submitted: a.submitted, total: a.total })}
                />
              ) : null}
              {(a.by_grade ?? []).map((g) => (
                <Flex key={g.grade_level} gap="small" align="center" wrap data-testid="progress-by-grade">
                  {g.grade_level ? <Link to={gradeHref(f, g.grade_level)}>{gradeLabel(g.grade_level)}</Link> : <Typography.Text>{gradeLabel(0)}</Typography.Text>}
                  {g.suppressed || g.total == null || g.submitted == null ? (
                    <SuppressedCell />
                  ) : (
                    <Typography.Text>{t('dashboard.assignments.submitted', { submitted: g.submitted, total: g.total })}</Typography.Text>
                  )}
                </Flex>
              ))}
            </Flex>
          </List.Item>
        )}
      />
      {w.grades?.length ? (
        <List
          size="small"
          header={t('dashboard.assignments.byGrade')}
          dataSource={w.grades}
          renderItem={(g) => (
            <List.Item
              key={g.grade_level}
              data-testid="grade-row"
              extra={g.suppressed ? <SuppressedCell /> : <DashboardMetric label={t('dashboard.assignments.completion')} value={g.assignment_completion_rate} format="percent" partial={partial} />}
            >
              <Link to={gradeHref(f, g.grade_level)}>{gradeLabel(g.grade_level)}</Link>
            </List.Item>
          )}
        />
      ) : null}
    </Flex>
  );
}

function WeakObjectivesWidget({ w, f }: { w: Widgets; f: DashboardFilters }) {
  const { t, i18n } = useTranslation();
  const pct = (v?: number) => (v === undefined ? '' : new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 0 }).format(v));
  return (
    <List
      size="small"
      dataSource={w.weak_objectives ?? []}
      renderItem={(o) => (
        <List.Item
          key={o.objective_id}
          actions={[
            ...(o.grade_levels?.length === 1
              ? [
                  <Link key="map" to={gradeHref(f, o.grade_levels[0]!)}>
                    {t('dashboard.weak.skillMap')}
                  </Link>,
                ]
              : []),
            <ActionGate key="assign" actionKey="action.school_lms.assignments.edit">
              <Link to={practiceWizardHref({ objectiveId: o.objective_id, schoolId: f.schoolId, subjectId: f.subjectId, grade: f.grade ?? (o.grade_levels?.length === 1 ? o.grade_levels[0] : undefined) })}>{t('dashboard.weak.assignPractice')}</Link>
            </ActionGate>,
          ]}
        >
          <Flex vertical>
            <Typography.Text strong>{o.yccd_ref ? t('dashboard.weak.codeRef', { code: o.code, ref: o.yccd_ref }) : o.code}</Typography.Text>
            <Typography.Text type="secondary">
              {t('dashboard.weak.belowProficient', { ratio: pct(o.below_proficient_ratio) })}
              {o.grade_levels?.length ? ` · ${o.grade_levels.map((g) => t('dashboard.filter.gradeN', { n: g })).join(', ')}` : ''}
            </Typography.Text>
          </Flex>
        </List.Item>
      )}
    />
  );
}

function AttentionWidget({ w, f }: { w: Widgets; f: DashboardFilters }) {
  const { t } = useTranslation();
  const list = w.students_needing_attention ?? [];
  return (
    <Flex vertical gap="small">
      <Typography.Text>{t('dashboard.attention.total', { count: w.students_needing_attention_total ?? list.length })}</Typography.Text>
      <List
        size="small"
        dataSource={list}
        renderItem={(s) => (
          <List.Item key={s.student_id}>
            <Flex wrap gap="small" align="center">
              <Link to={studentHref(f, s.student_id)} state={{ name: s.display_name }}>
                {s.display_name || t('dashboard.student.unnamed')}
              </Link>
              {s.grade_level ? <Tag>{t('dashboard.filter.gradeN', { n: s.grade_level })}</Tag> : null}
              {s.reasons.map((r) => (
                <ReasonTag key={r} reason={r} />
              ))}
            </Flex>
          </List.Item>
        )}
      />
    </Flex>
  );
}

function SelfStudyWidget({ w, partial }: { w: Widgets; partial: boolean }) {
  const { t, i18n } = useTranslation();
  const s = w.self_study;
  const series = w.active_minutes_series ?? [];
  return (
    <Flex vertical gap="middle">
      <Row gutter={[16, 8]}>
        <Col xs={24} md={8}>
          <DashboardMetric label={t('dashboard.selfStudy.avgMinutes')} value={s?.avg_active_minutes_per_student} format="minutes" partial={partial} testId="metric-avg-minutes" />
        </Col>
        <Col xs={24} md={8}>
          <DashboardMetric label={t('dashboard.selfStudy.started')} value={s?.recommendation_started_rate} format="percent" partial={partial} testId="metric-reco-started" />
        </Col>
        <Col xs={24} md={8}>
          <DashboardMetric label={t('dashboard.selfStudy.improved')} value={s?.objectives_improved_after_recommendation} partial={partial} testId="metric-improved" />
        </Col>
      </Row>
      {series.length ? (
        <DashboardChart
          kind="bar"
          testId="active-minutes-chart"
          categories={series.map((p) => dayjs(p.period_start).format('DD/MM'))}
          values={series.map((p) => (p.suppressed ? null : (p.value ?? null)))}
          ariaLabel={t('dashboard.selfStudy.chartAria', {
            values: series
              .map((p) => `${dayjs(p.period_start).format('DD/MM')}: ${p.suppressed ? t('dashboard.suppressedShort') : p.value === null || p.value === undefined ? t('dashboard.noValue') : new Intl.NumberFormat(i18n.language).format(p.value)}`)
              .join('; '),
          })}
        />
      ) : null}
    </Flex>
  );
}

function SafetyWidget({ w }: { w: Widgets }) {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const canOpen = !!view && menuStatus('menu.school_lms.safety_notifications', view) === 'visible';
  const n = w.safety_notifications_unacknowledged;
  return (
    <Flex gap="middle" align="center" wrap>
      <DashboardMetric label={t('dashboard.safety.pending')} value={n} testId="metric-safety" />
      {canOpen ? <Link to={SAFETY_BASE}>{t('dashboard.safety.open')}</Link> : null}
    </Flex>
  );
}

export function OverviewPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { filters, setFilters } = useDashboardFilters();
  const q = useSchoolDashboard(filters);
  const error = useWidgetError(q.error);
  const w = q.data?.widgets;
  const meta = metaOf(q.data);
  const partial = !!q.data?.is_partial;
  const loading = q.isPending && !!filters.schoolId;
  const frame = (key: keyof typeof WIDGET_ACCENTS, n: number, body: (w: Widgets) => JSX.Element, empty?: (w: Widgets) => boolean) => (
    <DashboardWidget
      testId={`widget-${key}`}
      accent={WIDGET_ACCENTS[key]}
      title={t(`dashboard.widget.${key}`, { n })}
      meta={meta}
      loading={loading}
      error={error}
      empty={!!w && !!empty?.(w)}
      emptyHint={t(`dashboard.empty.${key}`)}
    >
      {w ? body(w) : null}
    </DashboardWidget>
  );
  const widgets = {
    assignments: frame('assignments', 1, (x) => <AssignmentsWidget w={x} f={filters} partial={partial} />, (x) => !x.assignments_open && !x.assignments?.length && !x.grades?.length),
    weak: frame('weak', 2, (x) => <WeakObjectivesWidget w={x} f={filters} />, (x) => !x.weak_objectives?.length),
    attention: frame('attention', 3, (x) => <AttentionWidget w={x} f={filters} />, (x) => !x.students_needing_attention?.length),
    selfStudy: frame('selfStudy', 4, (x) => <SelfStudyWidget w={x} partial={partial} />),
    safety: frame('safety', 5, (x) => <SafetyWidget w={x} />),
  };
  return (
    <Flex vertical gap="middle" data-testid="dashboard-overview">
      <Flex justify="space-between" align="flex-end" wrap gap="middle" data-testid="dashboard-greeting">
        <div>
          <Typography.Title level={2} style={{ margin: 0 }}>
            {t(`dashboard.greeting.${greetingOf(dayjs().hour())}`)}
          </Typography.Title>
          <Typography.Text type="secondary">{t('dashboard.greeting.lead')}</Typography.Text>
        </div>
        <Space wrap>
          <Button onClick={() => navigate(`${DASHBOARD_BASE}/reference?${filtersToSearch(filters).toString()}`)}>{t('dashboard.action.reference')}</Button>
          <ActionGate actionKey="action.school_lms.assignments.edit">
            <Button type="primary" size="large" icon={<PlusOutlined aria-hidden />} onClick={() => navigate(`${ASSIGNMENTS_BASE}/new`)}>
              {t('dashboard.action.newAssignment')}
            </Button>
          </ActionGate>
        </Space>
      </Flex>
      <DashboardFilterBar filters={filters} onChange={setFilters} />
      {!filters.schoolId ? <Alert type="info" showIcon message={t('dashboard.pickSchool')} /> : null}
      {filters.schoolId ? (
        <Row gutter={[16, 16]}>
          <Col xs={24} xl={12}>
            {widgets.assignments}
          </Col>
          <Col xs={24} xl={12}>
            {widgets.weak}
          </Col>
          <Col xs={24} xl={12}>
            {widgets.attention}
          </Col>
          <Col xs={24} xl={12}>
            {widgets.selfStudy}
          </Col>
          <Col xs={24}>{widgets.safety}</Col>
        </Row>
      ) : null}
    </Flex>
  );
}
