// Tab "Phân tích" of a graded assignment (F056 T032; F056-AC-13; SCHOOL_LMS §8.6, EXAM_ENGINE §11): the exam session
// behind the assignment (`Assignment.exam_session_id`) → getExamSessionReport: score distribution (0.5-point buckets,
// chart + text), mean, median, pass rate, per item the p-value of the students WITHOUT `rapid_guess` answers, the
// rapid-guess rate, common wrong choices by `misconception_tag`, average time; the low-reliability and variant-gap
// warnings when the server raises them. "Giao ôn tập cho học sinh sai câu này" opens the F055 wizard in practice mode
// for the school, subject and first grade of the assignment (ADR-0095; the contract does not list who answered an item
// wrong). Assignments without an exam session have no item analysis yet (getAssignmentItemAnalysis is not in the
// contract — spec Clarifications).
import { DashboardChart, DashboardMetric, DashboardWidget } from '@eduworld/ops-ui/dashboard';
import { Alert, Breadcrumb, Button, Col, Flex, Row, Table, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { ActionGate } from '../../../../permissions/ActionGate';
import { useAssignment } from '../../lms/api/assignments';
import { TargetsText } from '../../lms/LmsUi';
import { useExamSessionReport, type ExamItemQuality } from '../api/resultsApi';
import { DashboardPageError, practiceWizardHref, RESULTS_BASE, useWidgetError } from '../DashboardUi';
import { LmsReportExport } from '../reports/ReportExport';

const BUCKET = 0.5;

export function ItemAnalysisPage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { assignmentId } = useParams();
  const assignment = useAssignment(assignmentId);
  const sessionId = assignment.data?.exam_session_id;
  const report = useExamSessionReport(sessionId);
  const error = useWidgetError(report.error);
  if (assignment.error) return <DashboardPageError error={assignment.error} />;
  if (report.error && !error?.code.startsWith('DASHBOARD_')) return <DashboardPageError error={report.error} />;
  const a = assignment.data;
  const r = report.data;
  const num = (v: number, d = 1) => new Intl.NumberFormat(i18n.language, { maximumFractionDigits: d }).format(v);
  const pct = (v: number | null | undefined) => (v === null || v === undefined ? '' : new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 0 }).format(v));
  const grade = (a?.targets ?? []).find((x) => x.target_type === 'grade')?.grade_level;
  const buckets = (r?.score_distribution ?? []).map((_, i) => `${num(i * BUCKET)}–${num((i + 1) * BUCKET)}`);
  const items = r?.items ?? [];
  const assignHref = practiceWizardHref({ schoolId: a?.school_id, subjectId: a?.subject_id, grade });
  return (
    <Flex vertical gap="middle" data-testid="item-analysis">
      <Breadcrumb items={[{ title: <Link to={RESULTS_BASE}>{t('dashboard.results.courses')}</Link> }, { title: a?.title ?? '' }]} />
      {a ? <TargetsText targets={a.targets} /> : null}
      {a && !sessionId ? <Alert type="info" showIcon data-testid="no-item-analysis" message={t('dashboard.analysis.unavailable')} /> : null}
      {a ? <LmsReportExport label={t('dashboard.analysis.exportAssignment')} request={{ report_type: 'assignment_results', format: 'xlsx', assignment_id: a.assignment_id }} /> : null}
      {sessionId ? (
        <DashboardWidget testId="widget-analysis" title={t('dashboard.analysis.title', { title: a?.title ?? '' })} loading={report.isPending} error={error} empty={!!r && !r.computed} emptyHint={t('dashboard.analysis.notComputed')}>
          {r ? (
            <Flex vertical gap="middle">
              {r.reliability_warning ? <Alert type="warning" showIcon data-testid="reliability-warning" message={t('dashboard.analysis.reliabilityWarning')} /> : null}
              {r.variant_gap ? <Alert type="warning" showIcon message={t('dashboard.analysis.variantGap')} /> : null}
              <Row gutter={[16, 8]}>
                <Col xs={12} md={6}>
                  <DashboardMetric label={t('dashboard.analysis.students')} value={r.student_count} />
                </Col>
                <Col xs={12} md={6}>
                  <DashboardMetric label={t('dashboard.analysis.mean')} value={r.mean} />
                </Col>
                <Col xs={12} md={6}>
                  <DashboardMetric label={t('dashboard.analysis.median')} value={r.median} testId="metric-median" />
                </Col>
                <Col xs={12} md={6}>
                  <DashboardMetric label={t('dashboard.analysis.passRate')} value={r.pass_rate} format="percent" testId="metric-pass-rate" />
                </Col>
              </Row>
              <DashboardChart
                kind="bar"
                testId="score-distribution"
                categories={buckets}
                values={r.score_distribution}
                ariaLabel={t('dashboard.analysis.distributionAria', { values: r.score_distribution.map((n, i) => `${buckets[i]}: ${n}`).join('; ') })}
              />
              <Table<ExamItemQuality>
                size="small"
                rowKey="question_version_id"
                pagination={false}
                dataSource={items}
                scroll={{ x: true }}
                columns={[
                  { key: 'pos', title: t('dashboard.analysis.item'), render: (_, _i, idx) => t('dashboard.analysis.itemN', { n: idx + 1 }) },
                  { key: 'p', title: t('dashboard.analysis.pValue'), render: (_, i) => (i.p_value === null || i.p_value === undefined ? '' : num(i.p_value, 2)) },
                  { key: 'rapid', title: t('dashboard.analysis.rapidGuess'), render: (_, i) => pct(i.rapid_guess_rate) },
                  {
                    key: 'wrong',
                    title: t('dashboard.analysis.commonWrong'),
                    render: (_, i) => (
                      <Flex wrap gap="small">
                        {i.top_misconceptions.map((m) => (
                          <Tag key={m.tag}>{t('dashboard.analysis.misconception', { tag: m.tag, count: m.count })}</Tag>
                        ))}
                      </Flex>
                    ),
                  },
                  { key: 'time', title: t('dashboard.analysis.avgTime'), render: (_, i) => (i.avg_time_ms === null || i.avg_time_ms === undefined ? '' : t('dashboard.analysis.seconds', { value: num(i.avg_time_ms / 1000) })) },
                  { key: 'flag', title: t('dashboard.analysis.flag'), render: (_, i) => (i.problem ? <Tag color="warning">{t('dashboard.analysis.problemItem')}</Tag> : null) },
                  {
                    key: 'assign',
                    title: t('dashboard.analysis.actions'),
                    render: () => (
                      <ActionGate actionKey="action.school_lms.assignments.edit">
                        <Button size="small" onClick={() => navigate(assignHref)}>
                          {t('dashboard.analysis.assignWrong')}
                        </Button>
                      </ActionGate>
                    ),
                  },
                ]}
              />
              <Typography.Text type="secondary">{t('dashboard.analysis.pValueNote')}</Typography.Text>
            </Flex>
          ) : null}
        </DashboardWidget>
      ) : null}
    </Flex>
  );
}
