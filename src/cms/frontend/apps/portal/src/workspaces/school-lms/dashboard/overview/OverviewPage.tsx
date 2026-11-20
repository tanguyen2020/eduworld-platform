// Overview of the School Learning Dashboard (F056 T020; F056-AC-01, AC-02, AC-04, AC-05, AC-07; SCHOOL_LMS §8.1, §8.7):
// one getSchoolDashboard call (school_id, from, to, grade/class/subject) answers the 5 weekly questions, shown in this
// order: ① open / due-soon assignments, ② LOs to consolidate, ③ students needing support, ④ self-study after
// recommendations, ⑤ safety notifications (count only + link, never chat content). Every widget has its own skeleton,
// "Cập nhật lúc", grain and "Tạm tính" (ops-ui DashboardWidget); numbers are the server's (AGENTS R1, ADR-0062).
// Students are never ranked by score (no score column, server order kept). Mobile: widgets stack as cards.
import { DashboardChart, DashboardMetric, DashboardWidget, type DashboardMetaView } from '@eduworld/ops-ui/dashboard';
import { Alert, Button, Col, Flex, List, Progress, Row, Space, Typography } from 'antd';
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

const metaOf = (d: SchoolDashboard | undefined): DashboardMetaView | undefined => (d ? { data_as_of: d.data_as_of, grain: d.grain, is_partial: d.is_partial } : undefined);

/** Link to a class page keeping range, subject and grade (F056-AC-08). */
export function classHref(f: DashboardFilters, classId: string): string {
  return `${DASHBOARD_BASE}/classes/${classId}?${filtersToSearch({ ...f, classId }).toString()}`;
}

export function studentHref(f: DashboardFilters, studentId: string, classId?: string): string {
  return `${DASHBOARD_BASE}/students/${studentId}?${filtersToSearch({ ...f, classId: classId ?? f.classId }).toString()}`;
}

function AssignmentsWidget({ w, f, partial }: { w: Widgets; f: DashboardFilters; partial: boolean }) {
  const { t } = useTranslation();
  const className = (id: string) => w.classes?.find((c) => c.class_id === id)?.name ?? '';
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
              {(a.by_class ?? []).map((c) => (
                <Flex key={c.class_id} gap="small" align="center" wrap>
                  <Link to={classHref(f, c.class_id)}>{className(c.class_id) || t('dashboard.class.unnamed')}</Link>
                  <Progress percent={c.total ? Math.round((c.submitted / c.total) * 100) : 0} size="small" style={{ flex: 1 }} format={() => t('dashboard.assignments.submitted', { submitted: c.submitted, total: c.total })} />
                </Flex>
              ))}
            </Flex>
          </List.Item>
        )}
      />
      {w.classes?.length ? (
        <List
          size="small"
          header={t('dashboard.assignments.byClass')}
          dataSource={w.classes}
          renderItem={(c) => (
            <List.Item key={c.class_id} extra={<DashboardMetric label={t('dashboard.assignments.completion')} value={c.assignment_completion_rate} format="percent" partial={partial} />}>
              <Link to={classHref(f, c.class_id)}>{c.name || t('dashboard.class.unnamed')}</Link>
            </List.Item>
          )}
        />
      ) : null}
    </Flex>
  );
}

function WeakObjectivesWidget({ w, f }: { w: Widgets; f: DashboardFilters }) {
  const { t, i18n } = useTranslation();
  const className = (id: string) => w.classes?.find((c) => c.class_id === id)?.name ?? '';
  const pct = (v?: number) => (v === undefined ? '' : new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 0 }).format(v));
  return (
    <List
      size="small"
      dataSource={w.weak_objectives ?? []}
      renderItem={(o) => (
        <List.Item
          key={o.objective_id}
          actions={[
            ...(o.class_ids?.[0]
              ? [
                  <Link key="map" to={classHref(f, o.class_ids[0])}>
                    {t('dashboard.weak.skillMap')}
                  </Link>,
                ]
              : []),
            <ActionGate key="assign" actionKey="action.school_lms.assignments.edit">
              <Link to={practiceWizardHref({ objectiveId: o.objective_id, classId: o.class_ids?.[0] })}>{t('dashboard.weak.assignPractice')}</Link>
            </ActionGate>,
          ]}
        >
          <Flex vertical>
            <Typography.Text strong>{o.yccd_ref ? t('dashboard.weak.codeRef', { code: o.code, ref: o.yccd_ref }) : o.code}</Typography.Text>
            <Typography.Text type="secondary">
              {t('dashboard.weak.belowProficient', { ratio: pct(o.below_proficient_ratio), classes: (o.class_ids ?? []).map(className).filter(Boolean).join(', ') })}
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
              <Link to={studentHref(f, s.student_id, s.class_id)}>{s.display_name || t('dashboard.student.unnamed')}</Link>
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
  const frame = (key: string, n: number, body: (w: Widgets) => JSX.Element, empty?: (w: Widgets) => boolean) => (
    <DashboardWidget
      testId={`widget-${key}`}
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
    assignments: frame('assignments', 1, (x) => <AssignmentsWidget w={x} f={filters} partial={partial} />, (x) => !x.assignments_open && !x.assignments?.length && !x.classes?.length),
    weak: frame('weak', 2, (x) => <WeakObjectivesWidget w={x} f={filters} />, (x) => !x.weak_objectives?.length),
    attention: frame('attention', 3, (x) => <AttentionWidget w={x} f={filters} />, (x) => !x.students_needing_attention?.length),
    selfStudy: frame('selfStudy', 4, (x) => <SelfStudyWidget w={x} partial={partial} />),
    safety: frame('safety', 5, (x) => <SafetyWidget w={x} />),
  };
  return (
    <Flex vertical gap="middle" data-testid="dashboard-overview">
      <DashboardFilterBar filters={filters} onChange={setFilters} />
      {!filters.schoolId ? <Alert type="info" showIcon message={t('dashboard.pickSchool')} /> : null}
      <Space wrap>
        <ActionGate actionKey="action.school_lms.assignments.edit">
          <Button type="primary" onClick={() => navigate(`${ASSIGNMENTS_BASE}/new`)}>
            {t('dashboard.action.newAssignment')}
          </Button>
        </ActionGate>
        <Button onClick={() => navigate(`${DASHBOARD_BASE}/reference?${filtersToSearch(filters).toString()}`)}>{t('dashboard.action.reference')}</Button>
      </Space>
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
