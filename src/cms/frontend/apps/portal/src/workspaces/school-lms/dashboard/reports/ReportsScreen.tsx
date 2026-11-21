// Screen of `menu.school_lms.reports` (F056 T037, T040; F056-AC-14…AC-17, AC-23): two tabs, each shown only when its
// catalog action is granted and turned on (ADR-0090) — "Xuất báo cáo" (createLmsReport: report type, format and the
// scope the type needs; asynchronous job, link with expiry; reference reports labelled "Tham khảo") and "Tổng hợp đơn
// vị" (Tenant Admin: getTenantLearningSummary by school × grade × subject, "< 5" for suppressed cells, export through
// createTenantReport). Numbers are the server's; cells are never recomputed. ADR-0095: no course or class — the course
// results and the class skill map (`skill_map` needs a `class_id` until WP-L2 reworks it by grade) are not offered;
// reference levels are per school and subject.
import { DashboardMetric, DashboardQueueProvider, DashboardRangePicker, DashboardWidget, SuppressedCell, type DashboardRange } from '@eduworld/ops-ui/dashboard';
import { Col, Flex, Form, Row, Select, Table, Tabs } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { localizedName } from '../../common/format';
import { SchoolSelect } from '../../common/SchoolSelect';
import { SingleTenantGate } from '../../common/scope';
import { PageTitle } from '../../common/ui';
import { useAssignments } from '../../lms/api/assignments';
import { useActionEnabled } from '../../lms/LmsUi';
import { useSchoolList } from '../../schools/useSchools';
import { useSubjects, type AssessmentPeriod } from '../api/dashboardApi';
import { useTenantLearningSummary, type LearningSummary, type LmsReportRequest } from '../api/reportsApi';
import { useWidgetError } from '../DashboardUi';
import { defaultRange, subjectLabel, todayIso } from '../model/filters';
import { DASHBOARD_CONFIG, RANGE_LIMITS } from '../model/limits';
import { RegulationDisclaimer } from './RegulationDisclaimer';
import { LmsReportExport, TenantReportExport } from './ReportExport';

/** Report types offered by the portal (ADR-0095: not `course_results`, not the class `skill_map`). */
type ReportType = Exclude<LmsReportRequest['report_type'], 'course_results' | 'skill_map'>;
type Format = LmsReportRequest['format'];

/** Formats offered per report type (SCHOOL_LMS §10 table); the server validates the request anyway. */
export const REPORT_FORMATS: Record<ReportType, Format[]> = {
  assignment_results: ['xlsx', 'csv'],
  regulation_reference: ['xlsx', 'pdf'],
  school_monthly: ['pdf'],
};
const TYPES = Object.keys(REPORT_FORMATS) as ReportType[];
const PERIODS: AssessmentPeriod[] = ['mid_term_1', 'end_term_1', 'mid_term_2', 'end_term_2'];

interface ExportForm {
  type: ReportType;
  format: Format;
  schoolId?: string;
  assignmentId?: string;
  subjectId?: string;
  period: AssessmentPeriod;
  range: DashboardRange;
}

/** The request of the form, or null while a parameter required by the type is missing (contract `LmsReportRequest`). */
export function toLmsReportRequest(v: ExportForm): LmsReportRequest | null {
  const base = { report_type: v.type, format: v.format };
  switch (v.type) {
    case 'assignment_results':
      return v.assignmentId ? { ...base, assignment_id: v.assignmentId } : null;
    case 'regulation_reference':
      return v.schoolId && v.subjectId ? { ...base, school_id: v.schoolId, subject_id: v.subjectId, period: v.period } : null;
    case 'school_monthly':
      return v.schoolId ? { ...base, school_id: v.schoolId, from: v.range.from, to: v.range.to } : null;
  }
}

function ExportTab() {
  const { t, i18n } = useTranslation();
  const [v, setV] = useState<ExportForm>({ type: 'assignment_results', format: 'xlsx', period: 'end_term_1', range: defaultRange(todayIso()) });
  const set = (p: Partial<ExportForm>) => setV((o) => ({ ...o, ...p }));
  const needs = (k: 'school' | 'assignment' | 'subject' | 'period' | 'range') =>
    ({
      assignment_results: ['assignment'],
      regulation_reference: ['school', 'subject', 'period'],
      school_monthly: ['school', 'range'],
    })[v.type].includes(k);
  const show = { school: needs('school'), assignment: needs('assignment'), subject: needs('subject'), period: needs('period'), range: needs('range') };
  const assignments = useAssignments({});
  const subjects = useSubjects();
  const request = toLmsReportRequest(v);
  return (
    <Flex vertical gap="middle" data-testid="export-tab">
      <Flex wrap gap="small" align="end">
        <Form.Item label={t('dashboard.export.type')} style={{ marginBottom: 0 }}>
          <Select<ReportType>
            aria-label={t('dashboard.export.type')}
            value={v.type}
            onChange={(type) => set({ type, format: REPORT_FORMATS[type][0]! })}
            options={TYPES.map((x) => ({ value: x, label: t(`dashboard.export.types.${x}`) }))}
            popupMatchSelectWidth={false}
          />
        </Form.Item>
        <Form.Item label={t('dashboard.export.format')} style={{ marginBottom: 0 }}>
          <Select<Format> aria-label={t('dashboard.export.format')} value={v.format} onChange={(format) => set({ format })} options={REPORT_FORMATS[v.type].map((f) => ({ value: f, label: f.toUpperCase() }))} />
        </Form.Item>
        {show.school ? (
          <Form.Item label={t('portal.school_lms.common.school')} style={{ marginBottom: 0 }}>
            <SchoolSelect value={v.schoolId} onChange={(schoolId) => set({ schoolId })} />
          </Form.Item>
        ) : null}
        {show.assignment ? (
          <Form.Item label={t('dashboard.export.assignment')} style={{ marginBottom: 0 }}>
            <Select aria-label={t('dashboard.export.assignment')} value={v.assignmentId} onChange={(assignmentId: string) => set({ assignmentId })} options={assignments.items.map((a) => ({ value: a.assignment_id, label: a.title }))} popupMatchSelectWidth={false} />
          </Form.Item>
        ) : null}
        {show.subject ? (
          <Form.Item label={t('dashboard.filter.subject')} style={{ marginBottom: 0 }}>
            <Select aria-label={t('dashboard.filter.subject')} value={v.subjectId} onChange={(subjectId: string) => set({ subjectId })} options={(subjects.data ?? []).map((s) => ({ value: s.subject_id, label: subjectLabel(s, i18n.language) }))} popupMatchSelectWidth={false} />
          </Form.Item>
        ) : null}
        {show.period ? (
          <Form.Item label={t('dashboard.reference.period')} style={{ marginBottom: 0 }}>
            <Select<AssessmentPeriod> aria-label={t('dashboard.reference.period')} value={v.period} onChange={(period) => set({ period })} options={PERIODS.map((p) => ({ value: p, label: t(`dashboard.reference.periods.${p}`) }))} popupMatchSelectWidth={false} />
          </Form.Item>
        ) : null}
        {show.range ? <DashboardRangePicker value={v.range} limits={RANGE_LIMITS} onChange={(range) => set({ range })} /> : null}
      </Flex>
      {v.type === 'regulation_reference' ? <RegulationDisclaimer /> : null}
      <LmsReportExport request={request} />
    </Flex>
  );
}

type Cell = NonNullable<LearningSummary['cells']>[number];

function TenantSummaryTab() {
  const { t, i18n } = useTranslation();
  const [schoolIds, setSchoolIds] = useState<string[]>([]);
  const [grade, setGrade] = useState<number | undefined>();
  const [range, setRange] = useState<DashboardRange>(() => defaultRange(todayIso()));
  const q = useTenantLearningSummary({ schoolIds, grade, from: range.from, to: range.to, grain: range.grain });
  const error = useWidgetError(q.error);
  const schools = useSchoolList();
  const subjects = useSubjects();
  const d = q.data;
  const pct = (v: number | null | undefined) => (v === null || v === undefined ? '' : new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 0 }).format(v));
  const n = (v: number | null | undefined) => (v === null || v === undefined ? '' : new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 1 }).format(v));
  const hidden = (c: Cell, v: string) => (c.suppressed ? <SuppressedCell /> : v);
  const schoolName = (id?: string) => localizedName(schools.items.find((s) => s.school_id === id)?.name, i18n.language);
  const subjectName = (id: string) => {
    const s = subjects.data?.find((x) => x.subject_id === id);
    return s ? subjectLabel(s, i18n.language) : '';
  };
  return (
    <Flex vertical gap="middle" data-testid="tenant-summary">
      <Flex wrap gap="small" align="end">
        <Form.Item label={t('dashboard.summary.schools')} style={{ marginBottom: 0 }}>
          <Select
            mode="multiple"
            allowClear
            aria-label={t('dashboard.summary.schools')}
            placeholder={t('dashboard.summary.allSchools')}
            value={schoolIds}
            onChange={(ids: string[]) => setSchoolIds(ids.slice(0, DASHBOARD_CONFIG.dashboard_page_size_max))}
            options={schools.items.map((s) => ({ value: s.school_id, label: localizedName(s.name, i18n.language) }))}
            style={{ minWidth: '100%' }}
            popupMatchSelectWidth={false}
          />
        </Form.Item>
        <Form.Item label={t('dashboard.filter.grade')} style={{ marginBottom: 0 }}>
          <Select allowClear aria-label={t('dashboard.filter.grade')} placeholder={t('dashboard.filter.allGrades')} value={grade} onChange={(g?: number) => setGrade(g)} options={Array.from({ length: 12 }, (_, i) => ({ value: i + 1, label: t('dashboard.filter.gradeN', { n: i + 1 }) }))} popupMatchSelectWidth={false} />
        </Form.Item>
        <DashboardRangePicker value={range} limits={RANGE_LIMITS} onChange={setRange} />
      </Flex>
      <TenantReportExport request={{ report_type: 'learning_summary', format: 'xlsx', from: range.from, to: range.to, ...(schoolIds.length ? { school_ids: schoolIds } : {}), ...(grade ? { grade_level: grade } : {}) }} />
      <DashboardWidget testId="widget-tenant-summary" title={t('dashboard.summary.title')} meta={d} loading={q.isPending} error={error} empty={!!d && !(d.cells ?? []).length} emptyHint={t('dashboard.empty.summary')}>
        {d ? (
          <Flex vertical gap="middle">
            <Row gutter={[16, 8]}>
              <Col xs={12} md={6}>
                <DashboardMetric label={t('dashboard.metric.activeStudents')} value={d.active_students} partial={d.is_partial} />
              </Col>
              <Col xs={12} md={6}>
                <DashboardMetric label={t('dashboard.student.accuracy')} value={d.accuracy} format="percent" partial={d.is_partial} />
              </Col>
              <Col xs={12} md={6}>
                <DashboardMetric label={t('dashboard.summary.minutes')} value={d.active_minutes_avg} format="minutes" partial={d.is_partial} />
              </Col>
              <Col xs={12} md={6}>
                <DashboardMetric label={t('dashboard.assignments.completion')} value={d.assignment_completion_rate} format="percent" partial={d.is_partial} />
              </Col>
            </Row>
            <Table<Cell>
              size="small"
              pagination={false}
              scroll={{ x: true }}
              rowKey={(c) => `${c.school_id}-${c.grade_level}-${c.subject_id}`}
              dataSource={d.cells ?? []}
              columns={[
                { key: 'school', title: t('portal.school_lms.common.school'), render: (_, c) => schoolName(c.school_id) },
                { key: 'grade', title: t('dashboard.filter.grade'), render: (_, c) => t('dashboard.filter.gradeN', { n: c.grade_level }) },
                { key: 'subject', title: t('dashboard.filter.subject'), render: (_, c) => subjectName(c.subject_id) },
                { key: 'students', title: t('dashboard.summary.students'), render: (_, c) => hidden(c, n(c.students)) },
                { key: 'active', title: t('dashboard.metric.activeStudents'), render: (_, c) => hidden(c, n(c.active_students)) },
                { key: 'accuracy', title: t('dashboard.student.accuracy'), render: (_, c) => hidden(c, pct(c.accuracy)) },
                { key: 'minutes', title: t('dashboard.summary.minutes'), render: (_, c) => hidden(c, n(c.active_minutes_avg)) },
                { key: 'attempts', title: t('dashboard.summary.attempts'), render: (_, c) => hidden(c, n(c.attempts_completed)) },
              ]}
            />
          </Flex>
        ) : null}
      </DashboardWidget>
    </Flex>
  );
}

export function ReportsScreen({ title }: { title: string }) {
  const { t } = useTranslation();
  const canExport = useActionEnabled('action.school_lms.reports.export');
  const canSummary = useActionEnabled('action.school_lms.reports.view');
  const items = [
    ...(canExport ? [{ key: 'export', label: t('dashboard.reports.exportTab'), children: <ExportTab /> }] : []),
    ...(canSummary ? [{ key: 'summary', label: t('dashboard.reports.summaryTab'), children: <TenantSummaryTab /> }] : []),
  ];
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>
        {() => <DashboardQueueProvider limit={DASHBOARD_CONFIG.dashboard_max_concurrent_per_user}>{items.length ? <Tabs items={items} destroyInactiveTabPane /> : null}</DashboardQueueProvider>}
      </SingleTenantGate>
    </section>
  );
}
