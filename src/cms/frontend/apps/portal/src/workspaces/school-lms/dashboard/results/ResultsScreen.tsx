// Screen of `menu.school_lms.results` (F056 T032; F056-AC-12, F056-AC-13; SCHOOL_LMS §8.6, §10; ADR-0095): the
// assignments of the scope filtered by school, subject and grade (listAssignments — no course any more) with their
// targets, completion and mean self-scored result → "Phân tích" of one assignment (getExamSessionReport of the exam
// session behind a graded assignment, export of the assignment results through createLmsReport). No XP, rating,
// competition points or manual grade input; numbers are the server's.
import { DashboardQueueProvider } from '@eduworld/ops-ui/dashboard';
import { Flex, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Route, Routes } from 'react-router-dom';
import { formatDateTime } from '../../common/format';
import { ResponsiveTable } from '../../common/ResponsiveTable';
import { SingleTenantGate } from '../../common/scope';
import { PageTitle } from '../../common/ui';
import { useAssignments, type Assignment, type AssignmentFilters } from '../../lms/api/assignments';
import { AssignmentFilterBar } from '../../lms/assignments/AssignmentFilterBar';
import { LmsTag, TargetsText } from '../../lms/LmsUi';
import { ItemAnalysisPage } from '../analysis/ItemAnalysisPage';
import { DashboardPageError, RESULTS_BASE } from '../DashboardUi';
import { DASHBOARD_CONFIG } from '../model/limits';

function AssignmentResultsList() {
  const { t, i18n } = useTranslation();
  const [filters, setFilters] = useState<AssignmentFilters>({});
  const list = useAssignments(filters);
  const score = (v: number | null | undefined) => (v === null || v === undefined ? '' : new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 1 }).format(v));
  const link = (a: Assignment) => (
    <Link to={`${RESULTS_BASE}/${a.assignment_id}`} aria-label={t('dashboard.results.openAnalysis', { title: a.title })}>
      {a.title}
    </Link>
  );
  const columns: ColumnType<Assignment>[] = [
    { key: 'title', title: t('lms.assignment.title'), render: (_, a) => link(a) },
    { key: 'targets', title: t('lms.wizard.steps.targets'), render: (_, a) => <TargetsText targets={a.targets} /> },
    { key: 'mode', title: t('lms.wizard.mode'), render: (_, a) => t(`lms.mode.${a.mode}`) },
    { key: 'status', title: t('lms.assignment.status'), render: (_, a) => <LmsTag group="assignmentStatus" value={a.status} /> },
    { key: 'due', title: t('lms.wizard.dueAt'), render: (_, a) => formatDateTime(a.due_at) },
    { key: 'done', title: t('lms.assignment.completion'), render: (_, a) => (a.completion ? `${a.completion.submitted ?? 0}/${a.completion.assigned ?? 0}` : '') },
    { key: 'score', title: t('dashboard.results.averageScore'), render: (_, a) => <span data-testid="score">{score(a.completion?.average_score)}</span> },
  ];
  return (
    <Flex vertical gap="middle" data-testid="assignment-results">
      <Typography.Text type="secondary">{t('dashboard.results.pickAssignment')}</Typography.Text>
      <Flex wrap gap="small" align="end" data-testid="results-filters">
        <AssignmentFilterBar value={filters} onChange={setFilters} />
      </Flex>
      {list.error ? <DashboardPageError error={list.error} /> : null}
      <ResponsiveTable<Assignment> rowKey="assignment_id" columns={columns} data={list.items} loading={list.isFetching} cardTitle={link} hasMore={list.hasMore} onLoadMore={list.loadMore} />
      <Typography.Text type="secondary">{t('dashboard.results.scale')}</Typography.Text>
    </Flex>
  );
}

export function ResultsScreen({ title }: { title: string }) {
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>
        {() => (
          <DashboardQueueProvider limit={DASHBOARD_CONFIG.dashboard_max_concurrent_per_user}>
            <Routes>
              <Route index element={<AssignmentResultsList />} />
              <Route path=":assignmentId" element={<ItemAnalysisPage />} />
            </Routes>
          </DashboardQueueProvider>
        )}
      </SingleTenantGate>
    </section>
  );
}
