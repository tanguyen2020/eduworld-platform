// Reference levels TT27/TT22 (F056-AC-16; SCHOOL_LMS §10): getRegulationReferenceReport for one school, subject and
// assessment period (no class filter — ADR-0095) — level per student as computed by the server, "Chưa đủ dữ liệu" when the
// server says `insufficient_data`, the low-reliability warning of official exams, always under the reference label.
import { Alert, Breadcrumb, Flex, Form, Select, Tag, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { ResponsiveTable } from '../../common/ResponsiveTable';
import { useRegulationReference, type AssessmentPeriod, type RegulationReferenceReport } from '../api/dashboardApi';
import { DashboardPageError, DASHBOARD_BASE } from '../DashboardUi';
import { DashboardFilterBar, useDashboardFilters } from '../DashboardFilterBar';
import { filtersToSearch } from '../model/filters';
import { RegulationDisclaimer } from './RegulationDisclaimer';

type Item = RegulationReferenceReport['items'][number];
const PERIODS: AssessmentPeriod[] = ['mid_term_1', 'end_term_1', 'mid_term_2', 'end_term_2'];

export function ReferenceLevelsPage() {
  const { t, i18n } = useTranslation();
  const { filters, setFilters } = useDashboardFilters();
  const [period, setPeriod] = useState<AssessmentPeriod>('end_term_1');
  const q = useRegulationReference({ schoolId: filters.schoolId, subjectId: filters.subjectId, period });
  // the report carries no display name and the class student list is gone (ADR-0095): rows are numbered until WP-L2
  const nameOf = (i: Item) => t('dashboard.student.numbered', { n: q.items.indexOf(i) + 1 });
  const pct = (v: number | undefined) => (v === undefined ? '' : new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 0 }).format(v));
  const columns: ColumnType<Item>[] = [
    { key: 'student', title: t('dashboard.student.name'), render: (_, i) => nameOf(i) },
    { key: 'level', title: t('dashboard.reference.level'), render: (_, i) => <Tag data-testid="reference-level">{t(`dashboard.reference.levels.${i.level_code}`)}</Tag> },
    { key: 'coverage', title: t('dashboard.reference.coverage'), render: (_, i) => pct(i.coverage_ratio) },
    { key: 'rapid', title: t('dashboard.metric.rapidGuess'), render: (_, i) => pct(i.rapid_guess_ratio) },
    { key: 'warn', title: t('dashboard.reference.reliability'), render: (_, i) => (i.low_reliability_exam_ids?.length ? <Tag color="warning">{t('dashboard.reference.lowReliability')}</Tag> : null) },
  ];
  return (
    <Flex vertical gap="middle" data-testid="dashboard-reference">
      <Breadcrumb items={[{ title: <Link to={`${DASHBOARD_BASE}?${filtersToSearch(filters).toString()}`}>{t('dashboard.breadcrumb.school')}</Link> }, { title: t('dashboard.action.reference') }]} />
      <RegulationDisclaimer serverText={q.first?.disclaimer} />
      <DashboardFilterBar filters={filters} onChange={setFilters} />
      <Form.Item label={t('dashboard.reference.period')} style={{ marginBottom: 0 }}>
        <Select<AssessmentPeriod> aria-label={t('dashboard.reference.period')} value={period} onChange={setPeriod} options={PERIODS.map((p) => ({ value: p, label: t(`dashboard.reference.periods.${p}`) }))} popupMatchSelectWidth={false} />
      </Form.Item>
      {!filters.subjectId ? <Alert type="info" showIcon message={t('dashboard.reference.pickSubject')} /> : null}
      {q.error ? <DashboardPageError error={q.error} /> : null}
      {filters.subjectId && !q.error ? (
        <ResponsiveTable<Item> rowKey="student_id" columns={columns} data={q.items} loading={q.isLoading} cardTitle={nameOf} hasMore={q.hasMore} onLoadMore={q.loadMore} />
      ) : null}
      {filters.subjectId && !q.isLoading && !q.error && q.items.length === 0 ? <Typography.Text type="secondary">{t('dashboard.metric.noData')}</Typography.Text> : null}
    </Flex>
  );
}
