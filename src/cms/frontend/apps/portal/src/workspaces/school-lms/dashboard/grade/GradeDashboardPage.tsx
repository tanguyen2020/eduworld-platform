// Grade page of the dashboard (F056 T028; F056-AC-08, AC-09, AC-10; SCHOOL_LMS §8.2–§8.3; ADR-0095): getGradeDashboard
// for one grade of the school with the range and subject of the overview kept (URL), breadcrumb school → grade →
// student, the LO heatmap of the grade (MasteryHeatmap: level symbol + token color, exact count in tooltip/aria), the
// assignment progress of the grade and its students (cursor pages ≤ dashboard_page_size_max; names from
// listTenantGradeStudents, searched on the server). A grade under the small-cell threshold is `suppressed`: totals "< 5",
// no heatmap, students still listed (FR-ANL-016). "Giao ôn tập theo LO" opens the F055 wizard prefilled with
// mode=practice, the LO, the school, subject and grade, and the students the server lists as weak on that LO.
import { DashboardMetric, DashboardWidget, MASTERY_LEVELS, MasteryHeatmap, SuppressedCell, type HeatmapRow, type MasteryLevel } from '@eduworld/ops-ui/dashboard';
import { Alert, Breadcrumb, Button, Col, Flex, Input, Row, Tag, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { ActionGate } from '../../../../permissions/ActionGate';
import { FIELDS } from '../../common/fieldSchemas';
import { ResponsiveTable } from '../../common/ResponsiveTable';
import { useGradeStudents } from '../../lms/api/targets';
import { useGradeDashboard, type GradeDashboard } from '../api/dashboardApi';
import { DashboardPageError, DASHBOARD_BASE, practiceWizardHref, ReasonTag, useWidgetError } from '../DashboardUi';
import { DashboardFilterBar, useDashboardFilters } from '../DashboardFilterBar';
import { filtersToSearch, GRADES } from '../model/filters';
import { studentHref } from '../overview/OverviewPage';

type StudentRow = GradeDashboard['students']['data'][number];

export function GradeDashboardPage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const params = useParams();
  const grade = GRADES.find((g) => String(g) === params.grade);
  const { filters, setFilters } = useDashboardFilters();
  const f = { ...filters, grade };
  const q = useGradeDashboard(filters.schoolId, grade, f);
  const error = useWidgetError(q.error);
  const [search, setSearch] = useState('');
  const names = useGradeStudents(filters.schoolId, grade, search);
  if (q.error && !error?.code.startsWith('DASHBOARD_')) return <DashboardPageError error={q.error} />;
  const d = q.first;
  const pct = (v: number | null | undefined) => (v === null || v === undefined ? '' : new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 0 }).format(v));
  const nameOf = (id: string) => names.items.find((s) => s.student_id === id)?.display_name;
  const searching = search.trim().length >= 2;
  const shown = searching ? q.students.filter((s) => names.items.some((n) => n.student_id === s.student_id)) : q.students;
  const weakStudents = (objectiveId: string) => (d?.students_needing_attention ?? []).filter((s) => s.student_id && s.weak_objective_ids?.includes(objectiveId)).map((s) => s.student_id!);
  const levelLabel = (l: MasteryLevel) => t(`dashboard.mastery.${l}`);
  const gradeName = grade ? t('dashboard.filter.gradeN', { n: grade }) : '';
  const rows: HeatmapRow[] = (d?.objective_distribution ?? []).map((o) => {
    const code = o.yccd_ref ? `${o.code ?? ''} (${o.yccd_ref})` : (o.code ?? '');
    return {
      key: o.objective_id ?? code,
      header: code,
      cells: MASTERY_LEVELS.map((level) => {
        const n = o.level_counts?.[level] ?? 0;
        return { level, text: String(n), muted: n === 0, description: t('dashboard.heatmap.cell', { lo: code, level: levelLabel(level), count: n }) };
      }),
      action: o.objective_id ? (
        <ActionGate actionKey="action.school_lms.assignments.edit">
          <Button size="small" onClick={() => navigate(practiceWizardHref({ objectiveId: o.objective_id, schoolId: filters.schoolId, subjectId: filters.subjectId, grade, studentIds: weakStudents(o.objective_id!) }))}>
            {t('dashboard.weak.assignPractice')}
          </Button>
        </ActionGate>
      ) : undefined,
    };
  });
  const link = (s: StudentRow) => (
    <Link to={studentHref(f, s.student_id)} state={{ name: nameOf(s.student_id) }}>
      {nameOf(s.student_id) ?? t('dashboard.student.unnamed')}
    </Link>
  );
  const columns: ColumnType<StudentRow>[] = [
    { key: 'name', title: t('dashboard.student.name'), render: (_, s) => link(s) },
    { key: 'activities', title: t('dashboard.student.activities'), render: (_, s) => s.activities_completed ?? '' },
    { key: 'lessons', title: t('dashboard.student.lessons'), render: (_, s) => s.lessons_completed ?? '' },
    { key: 'minutes', title: t('dashboard.student.minutes'), render: (_, s) => s.active_minutes ?? '' },
    { key: 'accuracy', title: t('dashboard.student.accuracy'), render: (_, s) => pct(s.accuracy) },
    {
      key: 'flags',
      title: t('dashboard.student.flags'),
      render: (_, s) => (
        <Flex wrap gap="small">
          {(s.reasons ?? []).map((r) => (
            <ReasonTag key={r} reason={r} />
          ))}
          {s.rapid_guess_flag && !s.reasons?.includes('rapid_guess') ? <Tag>{t('dashboard.reason.rapid_guess')}</Tag> : null}
        </Flex>
      ),
    },
  ];
  const metric = (label: string, value: number | null | undefined, percent = false) =>
    d?.suppressed ? (
      <Flex vertical>
        <Typography.Text type="secondary">{label}</Typography.Text>
        <SuppressedCell />
      </Flex>
    ) : (
      <DashboardMetric label={label} value={value} format={percent ? 'percent' : undefined} partial={d?.is_partial} />
    );
  return (
    <Flex vertical gap="middle" data-testid="dashboard-grade">
      <Breadcrumb items={[{ title: <Link to={`${DASHBOARD_BASE}?${filtersToSearch(f).toString()}`}>{t('dashboard.breadcrumb.school')}</Link> }, { title: gradeName }]} />
      <DashboardFilterBar filters={f} onChange={(n) => (n.grade && n.grade !== grade ? navigate(`${DASHBOARD_BASE}/grades/${n.grade}?${filtersToSearch(n).toString()}`) : setFilters({ ...n, grade }))} />
      {!filters.schoolId ? <Alert type="info" showIcon message={t('dashboard.pickSchool')} /> : null}
      <DashboardWidget testId="widget-grade-summary" title={t('dashboard.grade.summary', { name: gradeName })} meta={d} loading={q.isLoading} error={error}>
        {d ? (
          <Flex vertical gap="small">
            {d.suppressed ? <Alert type="info" showIcon data-testid="grade-suppressed" message={t('dashboard.grade.suppressed')} /> : null}
            <Row gutter={[16, 8]}>
              <Col xs={12} md={6}>
                {metric(t('dashboard.metric.activeStudents'), d.active_students_7d)}
              </Col>
              <Col xs={12} md={6}>
                <DashboardMetric label={t('dashboard.assignments.open')} value={d.assignments_open} partial={d.is_partial} />
              </Col>
              <Col xs={12} md={6}>
                {metric(t('dashboard.student.accuracy'), d.accuracy, true)}
              </Col>
              <Col xs={12} md={6}>
                {metric(t('dashboard.metric.rapidGuess'), d.rapid_guess_ratio, true)}
              </Col>
            </Row>
          </Flex>
        ) : null}
      </DashboardWidget>
      <DashboardWidget testId="widget-heatmap" title={t('dashboard.grade.heatmap')} meta={d} loading={q.isLoading} error={error} empty={!!d && rows.length === 0} emptyHint={t('dashboard.empty.heatmap')}>
        <MasteryHeatmap
          caption={t('dashboard.grade.heatmapCaption', { name: gradeName })}
          rowHeader={t('dashboard.grade.objective')}
          columns={MASTERY_LEVELS.map((l) => ({ key: l, label: levelLabel(l) }))}
          rows={rows}
          actionHeader={t('dashboard.analysis.actions')}
        />
      </DashboardWidget>
      <DashboardWidget testId="widget-students" title={t('dashboard.grade.students', { name: gradeName })} loading={q.isLoading} error={error}>
        <Flex vertical gap="small">
          <Input.Search
            allowClear
            aria-label={t('dashboard.grade.searchStudent')}
            placeholder={t('dashboard.grade.searchStudent')}
            maxLength={FIELDS.gradeStudentSearch.maxLength}
            onSearch={setSearch}
            onChange={(e) => !e.target.value && setSearch('')}
            style={{ maxWidth: '100%' }}
          />
          <ResponsiveTable<StudentRow> rowKey="student_id" columns={columns} data={shown} loading={q.isFetching} cardTitle={link} hasMore={q.hasMore} onLoadMore={q.loadMore} />
          {searching && !names.isFetching && shown.length === 0 ? <Typography.Text type="secondary">{t('dashboard.grade.noMatch')}</Typography.Text> : null}
        </Flex>
      </DashboardWidget>
    </Flex>
  );
}
