// Screen of `menu.school_lms.results` (F056 T032; F056-AC-12, F056-AC-13; SCHOOL_LMS §8.6, §10): courses of the scope →
// tab "Kết quả" of a course (getCourseResults: one column per graded assignment, self-scored `score_10` on the
// `score_scale`, submission status, "làm quá nhanh" mark; cursor pages ≤ dashboard_page_size_max; no XP, rating,
// competition points or manual grade input) → tab "Phân tích" of a graded assignment (getExamSessionReport of its exam
// session). Export of the course table through createLmsReport (job + signed link).
import { Flex, Tag, Tooltip, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { Link, Route, Routes, useParams } from 'react-router-dom';
import { ResponsiveTable } from '../../common/ResponsiveTable';
import { SingleTenantGate } from '../../common/scope';
import { PageTitle } from '../../common/ui';
import { useCourses, type Course } from '../../lms/api/courses';
import { courseLabel } from '../../lms/assignments/wizard/StepTargets';
import { DashboardQueueProvider } from '@eduworld/ops-ui/dashboard';
import { useCourseResults, type CourseResults } from '../api/resultsApi';
import { DashboardPageError, RESULTS_BASE } from '../DashboardUi';
import { DASHBOARD_CONFIG } from '../model/limits';
import { LmsReportExport } from '../reports/ReportExport';
import { ItemAnalysisPage } from '../analysis/ItemAnalysisPage';

type Row = CourseResults['rows'][number];

function CourseListPage() {
  const { t, i18n } = useTranslation();
  const courses = useCourses({ status: 'active' });
  const columns: ColumnType<Course>[] = [
    { key: 'name', title: t('lms.course.name'), render: (_, c) => <Link to={`${RESULTS_BASE}/${c.course_id}`}>{courseLabel(c, i18n.language)}</Link> },
    { key: 'students', title: t('lms.course.students'), render: (_, c) => c.student_count ?? '' },
  ];
  return (
    <Flex vertical gap="middle">
      <Typography.Text type="secondary">{t('dashboard.results.pickCourse')}</Typography.Text>
      {courses.error ? <DashboardPageError error={courses.error} /> : null}
      <ResponsiveTable<Course>
        rowKey="course_id"
        columns={columns}
        data={courses.items}
        loading={courses.isFetching}
        cardTitle={(c) => <Link to={`${RESULTS_BASE}/${c.course_id}`}>{courseLabel(c, i18n.language)}</Link>}
        hasMore={courses.hasMore}
        onLoadMore={courses.loadMore}
      />
    </Flex>
  );
}

export function CourseResultsTab() {
  const { t, i18n } = useTranslation();
  const { courseId } = useParams();
  const r = useCourseResults(courseId);
  if (r.error) return <DashboardPageError error={r.error} />;
  const score = (v: number) => new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 1 }).format(v);
  const columns: ColumnType<Row>[] = [
    { key: 'student', title: t('dashboard.student.name'), fixed: 'left', render: (_, row) => row.display_name ?? t('dashboard.student.unnamed') },
    ...r.columns.map(
      (c): ColumnType<Row> => ({
        key: c.assignment_id,
        title: (
          <Link to={`${RESULTS_BASE}/${courseId}/analysis/${c.assignment_id}`} aria-label={t('dashboard.results.openAnalysis', { title: c.title })}>
            {c.title}
          </Link>
        ),
        render: (_, row) => {
          const s = row.scores.find((x) => x.assignment_id === c.assignment_id);
          if (!s) return '';
          return (
            <Flex gap="small" align="center" wrap>
              {s.score_10 === null || s.score_10 === undefined ? <Tag>{t(`dashboard.submission.${s.status ?? 'not_started'}`)}</Tag> : <Typography.Text data-testid="score">{score(s.score_10)}</Typography.Text>}
              {s.too_fast ? (
                <Tooltip title={t('dashboard.results.tooFastHint')}>
                  <Tag color="warning">{t('dashboard.reason.rapid_guess')}</Tag>
                </Tooltip>
              ) : null}
            </Flex>
          );
        },
      }),
    ),
  ];
  return (
    <Flex vertical gap="middle" data-testid="course-results">
      <Flex justify="space-between" wrap gap="small" align="center">
        <Typography.Text type="secondary">{t('dashboard.results.scale')}</Typography.Text>
        <LmsReportExport request={courseId ? { report_type: 'course_results', format: 'xlsx', course_id: courseId } : null} />
      </Flex>
      <ResponsiveTable<Row> rowKey="student_id" columns={columns} data={r.rows} loading={r.isFetching} cardTitle={(row) => row.display_name ?? ''} hasMore={r.hasMore} onLoadMore={r.loadMore} />
      {!r.isLoading && r.columns.length === 0 ? <Typography.Text type="secondary">{t('dashboard.results.noGraded')}</Typography.Text> : null}
      <Link to={RESULTS_BASE}>{t('dashboard.results.back')}</Link>
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
              <Route index element={<CourseListPage />} />
              <Route path=":courseId" element={<CourseResultsTab />} />
              <Route path=":courseId/analysis/:assignmentId" element={<ItemAnalysisPage />} />
            </Routes>
          </DashboardQueueProvider>
        )}
      </SingleTenantGate>
    </section>
  );
}
