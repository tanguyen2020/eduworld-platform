// Screen of `menu.platform_admin.reports` (F056 T040; F056-AC-18, FR-ANL-012, FR-CMS-013): the System Admin's learning
// summary across tenants — getPlatformLearningSummary with `tenant_ids[]` from the topbar tenant filter (F057; no
// X-Tenant-Id in platform scope), a Tenant column in every table, "< 5" for suppressed cells (SuppressedCell), QA tenants
// excluded by the server listed; export through createPlatformReport / getPlatformReport (job + signed link). Internal
// gateway + platform role are enforced by the gateway/backend (AGENTS R20); numbers are the server's.
import {
  DashboardMetric,
  DashboardQueueProvider,
  DashboardRangePicker,
  DashboardWidget,
  ExportButton,
  SuppressedCell,
  useDashboardQueue,
  type DashboardRange,
} from '@eduworld/ops-ui/dashboard';
import { useQuery } from '@tanstack/react-query';
import { Alert, Col, Flex, Row, Space, Table } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { fetchAllAsPage } from '../../../shared/api/fetchAllPages';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { TenantFilter } from '../../../shell/TenantControls';
import { useWidgetError } from '../../school-lms/dashboard/DashboardUi';
import { pollWhileRunning, type LearningSummary, type ReportJob, type ReportRequest } from '../../school-lms/dashboard/api/reportsApi';
import { defaultRange, subjectLabel, todayIso } from '../../school-lms/dashboard/model/filters';
import { DASHBOARD_CONFIG, DASHBOARD_QUERY_TIMING, RANGE_LIMITS } from '../../school-lms/dashboard/model/limits';
import { useExportErrorMessage } from '../../school-lms/dashboard/reports/ReportExport';
import type { Subject } from '../../school-lms/dashboard/api/dashboardApi';
import { useTenantDirectory } from '../api/tenants';
import { usePlatformApi } from '../api/common';
import { PageTitle, TenantCell } from '../ui';

type Cell = NonNullable<LearningSummary['cells']>[number];
type ByTenant = NonNullable<LearningSummary['by_tenant']>[number];

function usePlatformSummary(range: DashboardRange) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const queue = useDashboardQueue();
  const query = scope.listQuery({ from: range.from, to: range.to, ...(range.grain ? { grain: range.grain } : {}) });
  return useQuery({
    queryKey: ['platform', 'learning-summary', query],
    queryFn: () => queue.run(() => api.call<LearningSummary>(api.client.GET('/admin/analytics/learning/summary', { params: { query } } as never))),
    ...DASHBOARD_QUERY_TIMING,
  });
}

/** Subject names of the platform catalogue (listCmsSubjects); without access the column shows the id. */
function useSubjectNames() {
  const api = usePlatformApi();
  const { i18n } = useTranslation();
  const q = useQuery({
    queryKey: ['platform', 'subjects'],
    queryFn: () => fetchAllAsPage<Subject>((query) => api.call(api.client.GET('/cms/learning/subjects', { params: { query } }))),
    staleTime: 10 * 60_000,
  });
  return (id: string) => {
    const s = q.data?.data.find((x) => x.subject_id === id);
    return s ? subjectLabel(s, i18n.language) : id;
  };
}

function usePlatformExport() {
  const api = usePlatformApi();
  const [job, setJob] = useState<ReportJob | null>(null);
  const create = useIdempotentMutation<ReportJob, ReportRequest>((body, intent) => api.call<ReportJob>(api.client.POST('/admin/reports', { body, headers: intent } as never)), { onSuccess: setJob });
  const running = job?.status === 'queued' || job?.status === 'running';
  const poll = useQuery({
    queryKey: ['platform', 'report', job?.report_id],
    queryFn: () => api.call<ReportJob>(api.client.GET('/admin/reports/{report_id}', { params: { path: { report_id: job!.report_id } } } as never)),
    enabled: !!job && running,
    refetchInterval: pollWhileRunning,
  });
  return { job: job ? (poll.data ?? job) : null, create: create.mutate, pending: create.isPending, error: create.error ?? poll.error };
}

function PlatformSummary() {
  const { t, i18n } = useTranslation();
  const scope = useCrossTenantScope();
  const [range, setRange] = useState<DashboardRange>(() => defaultRange(todayIso()));
  const q = usePlatformSummary(range);
  const error = useWidgetError(q.error);
  const directory = useTenantDirectory();
  const exp = usePlatformExport();
  const subjectName = useSubjectNames();
  const exportError = useExportErrorMessage(exp.error);
  const d = q.data;
  const pct = (v: number | null | undefined) => (v === null || v === undefined ? '' : new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 0 }).format(v));
  const n = (v: number | null | undefined) => (v === null || v === undefined ? '' : new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 1 }).format(v));
  const hidden = (c: Cell, v: string) => (c.suppressed ? <SuppressedCell /> : v);
  const request: ReportRequest = { report_type: 'learning_summary', format: 'xlsx', from: range.from, to: range.to, ...(scope.filter.length ? { tenant_ids: [...scope.filter] } : {}) };
  return (
    <Flex vertical gap="middle" data-testid="platform-summary">
      <Space wrap>
        <TenantFilter />
        <DashboardRangePicker value={range} limits={RANGE_LIMITS} onChange={setRange} />
      </Space>
      <ActionGate actionKey="action.platform_admin.reports.export">
        <ExportButton onExport={() => exp.create(request)} pending={exp.pending} job={exp.job} errorMessage={exportError} />
      </ActionGate>
      {d?.excluded_tenant_ids?.length ? <Alert type="info" showIcon message={t('dashboard.summary.excluded', { count: d.excluded_tenant_ids.length })} /> : null}
      <DashboardWidget testId="widget-platform-summary" title={t('dashboard.summary.platformTitle')} meta={d} loading={q.isPending} error={error} empty={!!d && !(d.cells ?? []).length && !(d.by_tenant ?? []).length} emptyHint={t('dashboard.empty.summary')}>
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
                <DashboardMetric label={t('dashboard.summary.attempts')} value={d.attempts_completed} partial={d.is_partial} />
              </Col>
            </Row>
            <Table<ByTenant>
              size="small"
              pagination={false}
              rowKey={(r) => r.tenant_id ?? ''}
              dataSource={d.by_tenant ?? []}
              columns={[
                { key: 'tenant', title: t('platform.common.tenant'), render: (_, r) => <TenantCell tenantId={r.tenant_id} directory={directory} /> },
                { key: 'active', title: t('dashboard.metric.activeStudents'), render: (_, r) => (r.active_students === null || r.active_students === undefined ? <SuppressedCell /> : n(r.active_students)) },
                { key: 'accuracy', title: t('dashboard.student.accuracy'), render: (_, r) => pct(r.accuracy) },
              ]}
            />
            <Table<Cell>
              size="small"
              pagination={false}
              scroll={{ x: true }}
              rowKey={(c) => `${c.tenant_id}-${c.grade_level}-${c.subject_id}`}
              dataSource={d.cells ?? []}
              columns={[
                { key: 'tenant', title: t('platform.common.tenant'), render: (_, c) => <TenantCell tenantId={c.tenant_id} directory={directory} /> },
                { key: 'grade', title: t('dashboard.filter.grade'), render: (_, c) => t('dashboard.filter.gradeN', { n: c.grade_level }) },
                { key: 'subject', title: t('dashboard.filter.subject'), render: (_, c) => subjectName(c.subject_id) },
                { key: 'students', title: t('dashboard.summary.students'), render: (_, c) => hidden(c, n(c.students)) },
                { key: 'active', title: t('dashboard.metric.activeStudents'), render: (_, c) => hidden(c, n(c.active_students)) },
                { key: 'accuracy', title: t('dashboard.student.accuracy'), render: (_, c) => hidden(c, pct(c.accuracy)) },
                { key: 'attempts', title: t('dashboard.summary.attempts'), render: (_, c) => hidden(c, n(c.attempts_completed)) },
              ]}
            />
          </Flex>
        ) : null}
      </DashboardWidget>
    </Flex>
  );
}

export function PlatformReportsPage({ title }: { title: string }) {
  return (
    <section>
      <PageTitle title={title} />
      <DashboardQueueProvider limit={DASHBOARD_CONFIG.dashboard_max_concurrent_per_user}>
        <PlatformSummary />
      </DashboardQueueProvider>
    </section>
  );
}
