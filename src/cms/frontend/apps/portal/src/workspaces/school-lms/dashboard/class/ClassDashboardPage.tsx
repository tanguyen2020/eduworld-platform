// Class page of the dashboard (F056 T028; F056-AC-08, AC-09, AC-10, AC-22; SCHOOL_LMS §8.3): getClassDashboard with the
// range and subject of the overview kept (URL), breadcrumb school → class → student, the LO heatmap of the class
// (MasteryHeatmap: level symbol + token color, exact count in tooltip/aria) and the students of the class (cursor pages
// ≤ dashboard_page_size_max; names from listTenantClassStudents, searched with the OpenAPI `q` limit). "Giao ôn tập theo
// LO" opens the F055 wizard prefilled with mode=practice, the LO, the class and the students the server lists as weak
// on that LO (`students_needing_attention[].weak_objective_ids`) — 2 actions to assign. Out of scope → 403/404 pages.
import { DashboardMetric, DashboardWidget, MASTERY_LEVELS, MasteryHeatmap, type HeatmapRow, type MasteryLevel } from '@eduworld/ops-ui/dashboard';
import { Breadcrumb, Button, Col, Flex, Input, Row, Tag, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { ActionGate } from '../../../../permissions/ActionGate';
import { useClassName, useClassStudents } from '../../classes/useClasses';
import { FIELDS } from '../../common/fieldSchemas';
import { ResponsiveTable } from '../../common/ResponsiveTable';
import { useClassDashboard, type ClassDashboard } from '../api/dashboardApi';
import { DashboardPageError, DASHBOARD_BASE, practiceWizardHref, ReasonTag, useWidgetError } from '../DashboardUi';
import { DashboardFilterBar, useDashboardFilters } from '../DashboardFilterBar';
import { filtersToSearch } from '../model/filters';
import { studentHref } from '../overview/OverviewPage';

type StudentRow = ClassDashboard['students']['data'][number];

export function ClassDashboardPage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { classId } = useParams();
  const { filters, setFilters } = useDashboardFilters();
  const q = useClassDashboard(classId, filters);
  const error = useWidgetError(q.error);
  const [search, setSearch] = useState('');
  const schoolId = q.first?.school_id ?? filters.schoolId;
  const names = useClassStudents(schoolId, classId, search);
  const className = useClassName(schoolId, classId);
  const f = { ...filters, classId };
  if (q.error && !error?.code.startsWith('DASHBOARD_')) return <DashboardPageError error={q.error} />;
  const d = q.first;
  const pct = (v: number | null | undefined) => (v === null || v === undefined ? '' : new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 0 }).format(v));
  const nameOf = (id: string) => names.items.find((s) => s.student_id === id)?.display_name;
  const searching = search.trim().length >= 2;
  const shown = searching ? q.students.filter((s) => names.items.some((n) => n.student_id === s.student_id)) : q.students;
  const weakStudents = (objectiveId: string) => (d?.students_needing_attention ?? []).filter((s) => s.student_id && s.weak_objective_ids?.includes(objectiveId)).map((s) => s.student_id!);
  const levelLabel = (l: MasteryLevel) => t(`dashboard.mastery.${l}`);
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
          <Button size="small" onClick={() => navigate(practiceWizardHref({ objectiveId: o.objective_id, classId, studentIds: weakStudents(o.objective_id!) }))}>
            {t('dashboard.weak.assignPractice')}
          </Button>
        </ActionGate>
      ) : undefined,
    };
  });
  const columns: ColumnType<StudentRow>[] = [
    { key: 'name', title: t('dashboard.student.name'), render: (_, s) => <Link to={studentHref(f, s.student_id, classId)}>{nameOf(s.student_id) ?? t('dashboard.student.unnamed')}</Link> },
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
  return (
    <Flex vertical gap="middle" data-testid="dashboard-class">
      <Breadcrumb
        items={[
          { title: <Link to={`${DASHBOARD_BASE}?${filtersToSearch({ ...filters, classId: undefined }).toString()}`}>{t('dashboard.breadcrumb.school')}</Link> },
          { title: className || t('dashboard.class.unnamed') },
        ]}
      />
      <DashboardFilterBar filters={f} onChange={(n) => setFilters({ ...n, classId })} showClass={false} />
      <DashboardWidget testId="widget-class-summary" title={t('dashboard.class.summary', { name: className })} meta={d} loading={q.isLoading} error={error}>
        {d ? (
          <Row gutter={[16, 8]}>
            <Col xs={12} md={6}>
              <DashboardMetric label={t('dashboard.class.activeStudents')} value={d.active_students_7d} partial={d.is_partial} />
            </Col>
            <Col xs={12} md={6}>
              <DashboardMetric label={t('dashboard.assignments.open')} value={d.assignments_open} partial={d.is_partial} />
            </Col>
            <Col xs={12} md={6}>
              <DashboardMetric label={t('dashboard.student.accuracy')} value={d.accuracy} format="percent" partial={d.is_partial} />
            </Col>
            <Col xs={12} md={6}>
              <DashboardMetric label={t('dashboard.class.rapidGuess')} value={d.rapid_guess_ratio} format="percent" partial={d.is_partial} />
            </Col>
          </Row>
        ) : null}
      </DashboardWidget>
      <DashboardWidget testId="widget-heatmap" title={t('dashboard.class.heatmap')} meta={d} loading={q.isLoading} error={error} empty={!!d && rows.length === 0} emptyHint={t('dashboard.empty.heatmap')}>
        <MasteryHeatmap
          caption={t('dashboard.class.heatmapCaption', { name: className })}
          rowHeader={t('dashboard.class.objective')}
          columns={MASTERY_LEVELS.map((l) => ({ key: l, label: levelLabel(l) }))}
          rows={rows}
          actionHeader={t('dashboard.class.actions')}
        />
      </DashboardWidget>
      <DashboardWidget testId="widget-students" title={t('dashboard.class.students')} loading={q.isLoading} error={error}>
        <Flex vertical gap="small">
          <Input.Search
            allowClear
            aria-label={t('dashboard.class.searchStudent')}
            placeholder={t('dashboard.class.searchStudent')}
            maxLength={FIELDS.classStudentSearch.maxLength}
            onSearch={setSearch}
            onChange={(e) => !e.target.value && setSearch('')}
            style={{ maxWidth: '100%' }}
          />
          <ResponsiveTable<StudentRow>
            rowKey="student_id"
            columns={columns}
            data={shown}
            loading={q.isFetching}
            cardTitle={(s) => <Link to={studentHref(f, s.student_id, classId)}>{nameOf(s.student_id) ?? t('dashboard.student.unnamed')}</Link>}
            hasMore={q.hasMore}
            onLoadMore={q.loadMore}
          />
          {searching && !names.isFetching && shown.length === 0 ? <Typography.Text type="secondary">{t('dashboard.class.noMatch')}</Typography.Text> : null}
        </Flex>
      </DashboardWidget>
    </Flex>
  );
}
