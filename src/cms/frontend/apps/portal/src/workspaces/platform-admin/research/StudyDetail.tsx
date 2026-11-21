// Study detail (F035 US8, F035-AC-35): description, activate/complete, classes and pairs, waves, withdrawals, the
// efficacy report and the pseudonymised dataset export. The report is computed by the backend job; the page only
// shows what the API returns (aggregates, small cells already hidden — R1, R16).
import { Alert, App as AntApp, Button, DatePicker, Descriptions, Form, Modal, Popconfirm, Radio, Select, Space, Table, Tabs, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import type { Dayjs } from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useParams } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useTenantDirectory } from '../api/tenants';
import { applyFieldErrors, formatTime, LimitedField, PageTitle, TenantCell, TenantSelect } from '../ui';
import {
  ARMS,
  WAVE_KINDS,
  WITHDRAW_SCOPES,
  useAddStudyCohort,
  useCreateReport,
  useExportDataset,
  usePlatformReport,
  useRemoveStudyCohort,
  useReport,
  useScheduleWave,
  useStudy,
  useStudyCohorts,
  useStudyTransition,
  useWaves,
  useWithdraw,
  type EfficacyStudy,
  type EfficacyStudyCohort,
  type EfficacyWave,
  type EfficacyWithdrawal,
} from './api';
import { RESEARCH_LIMITS as L } from './limits';
import { ReportView } from './ReportView';
import { BoundedNumberField, StatusTag, UuidField, UuidListField } from './ResearchForms';

export function StudyDetail() {
  const { t } = useTranslation();
  const { studyId = '' } = useParams();
  const study = useStudy(studyId);
  const s = study.data;
  return (
    <section>
      <Link to=".." relative="path">
        {t('platform.research.common.back')}
      </Link>
      {study.error ? <ProblemMessage error={study.error} /> : null}
      {s ? (
        <>
          <PageTitle title={`${s.code} · ${s.title}`} extra={<StudyActions study={s} />} />
          <Descriptions size="small" column={{ xs: 1, md: 3 }}>
            <Descriptions.Item label={t('platform.research.common.status')}>
              <StatusTag status={s.status} />
            </Descriptions.Item>
            <Descriptions.Item label={t('platform.research.studies.grade')}>{s.grade_level}</Descriptions.Item>
            <Descriptions.Item label={t('platform.research.studies.minWeeks')}>{s.min_intervention_weeks}</Descriptions.Item>
            <Descriptions.Item label={t('platform.research.studies.objectives')}>{s.objective_ids.length}</Descriptions.Item>
            <Descriptions.Item label={t('platform.research.studies.formA')}>
              <Typography.Text code>{s.form_a_blueprint_version_id}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label={t('platform.research.studies.formB')}>
              <Typography.Text code>{s.form_b_blueprint_version_id}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label={t('platform.research.studies.activatedAt')}>{formatTime(s.activated_at)}</Descriptions.Item>
            <Descriptions.Item label={t('platform.research.studies.completedAt')}>{formatTime(s.completed_at)}</Descriptions.Item>
          </Descriptions>
          <Tabs
            items={[
              { key: 'cohorts', label: t('platform.research.cohorts.tab'), children: <CohortsTab study={s} /> },
              { key: 'waves', label: t('platform.research.waves.tab'), children: <WavesTab study={s} /> },
              { key: 'withdraw', label: t('platform.research.withdraw.tab'), children: <WithdrawTab study={s} /> },
              { key: 'report', label: t('platform.research.report.tab'), children: <ReportTab study={s} /> },
            ]}
          />
        </>
      ) : null}
    </section>
  );
}

function StudyActions({ study }: { study: EfficacyStudy }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const activate = useStudyTransition(study.study_id, 'activate');
  const complete = useStudyTransition(study.study_id, 'complete');
  const done = (key: string) => () => message.success(t(key));
  return (
    <Space wrap>
      {activate.error ? <ProblemMessage error={activate.error} /> : null}
      {complete.error ? <ProblemMessage error={complete.error} /> : null}
      {study.status === 'draft' ? (
        <ActionGate actionKey="action.platform_admin.efficacy_studies.edit">
          <Popconfirm title={t('platform.research.studies.activateConfirm')} okText={t('platform.research.studies.activate')} cancelText={t('platform.research.common.cancel')} onConfirm={() => activate.mutate(undefined, { onSuccess: done('platform.research.studies.activated') })}>
            <Button type="primary" loading={activate.isPending}>
              {t('platform.research.studies.activate')}
            </Button>
          </Popconfirm>
        </ActionGate>
      ) : null}
      {study.status === 'active' ? (
        <ActionGate actionKey="action.platform_admin.efficacy_studies.edit">
          <Popconfirm title={t('platform.research.studies.completeConfirm')} okText={t('platform.research.studies.complete')} cancelText={t('platform.research.common.cancel')} onConfirm={() => complete.mutate(undefined, { onSuccess: done('platform.research.studies.completed') })}>
            <Button loading={complete.isPending}>{t('platform.research.studies.complete')}</Button>
          </Popconfirm>
        </ActionGate>
      ) : null}
    </Space>
  );
}

// ---------- cohorts (school × grade of the study, ADR-0095) ----------

function CohortsTab({ study }: { study: EfficacyStudy }) {
  const { t } = useTranslation();
  const directory = useTenantDirectory();
  const list = useStudyCohorts(study.study_id);
  const remove = useRemoveStudyCohort(study.study_id);
  const [adding, setAdding] = useState(false);
  const draft = study.status === 'draft';
  const columns: ColumnsType<EfficacyStudyCohort> = [
    { key: 'tenant', title: t('platform.research.common.tenant'), render: (_, c) => <TenantCell tenantId={c.tenant_id} directory={directory} /> },
    { key: 'school', title: t('platform.research.cohorts.school'), render: (_, c) => c.school_id },
    { key: 'grade', title: t('platform.research.cohorts.grade'), render: (_, c) => t('platform.research.cohorts.gradeN', { n: c.grade_level }) },
    { key: 'conflict', title: t('platform.research.cohorts.conflict'), render: (_, c) => (c.conflict ? <Typography.Text type="warning">{t('platform.research.cohorts.conflictYes')}</Typography.Text> : null) },
    { key: 'pair', title: t('platform.research.cohorts.pair'), render: (_, c) => c.pair_no ?? t('platform.research.common.none') },
    { key: 'arm', title: t('platform.research.cohorts.arm'), render: (_, c) => (c.arm ? t(`platform.research.arm.${c.arm}`) : t('platform.research.cohorts.armRandom')) },
    { key: 'form', title: t('platform.research.cohorts.formOrder'), render: (_, c) => (c.form_order ? t(`platform.research.formOrder.${c.form_order}`) : t('platform.research.common.none')) },
    { key: 'participants', title: t('platform.research.cohorts.participants'), render: (_, c) => c.participant_count ?? 0 },
    { key: 'status', title: t('platform.research.common.status'), render: (_, c) => <StatusTag status={c.status} /> },
    {
      key: 'actions',
      title: t('platform.research.common.actions'),
      render: (_, c) =>
        draft ? (
          <ActionGate actionKey="action.platform_admin.efficacy_studies.edit">
            <Popconfirm title={t('platform.research.cohorts.removeConfirm')} okText={t('platform.research.cohorts.remove')} cancelText={t('platform.research.common.cancel')} onConfirm={() => remove.mutate(c)}>
              <Button size="small" danger>
                {t('platform.research.cohorts.remove')}
              </Button>
            </Popconfirm>
          </ActionGate>
        ) : null,
    },
  ];
  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      {draft ? (
        <ActionGate actionKey="action.platform_admin.efficacy_studies.edit">
          <Button onClick={() => setAdding(true)}>{t('platform.research.cohorts.add')}</Button>
        </ActionGate>
      ) : (
        <Typography.Paragraph type="secondary">{t('platform.research.cohorts.locked')}</Typography.Paragraph>
      )}
      {list.error ? <ProblemMessage error={list.error} /> : null}
      {remove.error ? <ProblemMessage error={remove.error} /> : null}
      <Table rowKey={(c) => `${c.tenant_id}/${c.school_id}`} columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} locale={{ emptyText: t('platform.research.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('platform.research.common.loadMore')}
        </Button>
      ) : null}
      {adding ? <AddCohortDialog study={study} onClose={() => setAdding(false)} /> : null}
    </Space>
  );
}

function AddCohortDialog({ study, onClose }: { study: EfficacyStudy; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm();
  const add = useAddStudyCohort(study.study_id);
  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const { tenant_id: tenantId, ...rest } = v;
    const body = Object.fromEntries(Object.entries(rest).filter(([, x]) => x !== undefined && x !== null && x !== ''));
    add.mutate(
      { tenantId, body: body as never },
      {
        onSuccess: () => {
          message.success(t('platform.research.cohorts.added'));
          onClose();
        },
        onError: (e) => applyFieldErrors(form, e, ['school_id', 'pair_no', 'arm', 'form_order'], t, (k) => i18n.exists(k)),
      },
    );
  };
  return (
    <Modal open title={t('platform.research.cohorts.add')} okText={t('platform.research.common.save')} cancelText={t('platform.research.common.cancel')} confirmLoading={add.isPending} onOk={submit} onCancel={onClose} destroyOnHidden>
      <Typography.Paragraph type="secondary">{t('platform.research.cohorts.addHelp')}</Typography.Paragraph>
      {add.error ? <ProblemMessage error={add.error} /> : null}
      <Form form={form} name="efficacy_cohort" layout="vertical">
        <Form.Item name="tenant_id" label={t('platform.research.common.targetTenant')} rules={[{ required: true, message: t('platform.research.form.required') }]}>
          <TenantSelect ariaLabel={t('platform.research.common.targetTenant')} />
        </Form.Item>
        <UuidField name="school_id" label={t('platform.research.cohorts.schoolId')} />
        <BoundedNumberField name="pair_no" label={t('platform.research.cohorts.pair')} limit={L.pairNo} required={false} />
        <Form.Item name="arm" label={t('platform.research.cohorts.arm')}>
          <Select aria-label={t('platform.research.cohorts.arm')} allowClear placeholder={t('platform.research.cohorts.armRandom')} options={ARMS.map((a) => ({ value: a, label: t(`platform.research.arm.${a}`) }))} />
        </Form.Item>
        <Form.Item name="form_order" label={t('platform.research.cohorts.formOrder')}>
          <Select aria-label={t('platform.research.cohorts.formOrder')} allowClear placeholder={t('platform.research.cohorts.formOrderAuto')} options={(['a_then_b', 'b_then_a'] as const).map((x) => ({ value: x, label: t(`platform.research.formOrder.${x}`) }))} />
        </Form.Item>
      </Form>
    </Modal>
  );
}

// ---------- waves ----------

const sum = (w: EfficacyWave, k: 'invited_count' | 'completed_count' | 'absent_count') => (w.cohorts ?? []).reduce((n, c) => n + c[k], 0);

function WavesTab({ study }: { study: EfficacyStudy }) {
  const { t } = useTranslation();
  const waves = useWaves(study.study_id);
  const [scheduling, setScheduling] = useState(false);
  const columns: ColumnsType<EfficacyWave> = [
    { key: 'kind', title: t('platform.research.waves.kind'), render: (_, w) => t(`platform.research.waveKind.${w.kind}`) },
    { key: 'form', title: t('platform.research.waves.form'), render: (_, w) => (w.form ? t(`platform.research.formName.${w.form}`) : t('platform.research.waves.formByCohort')) },
    { key: 'window', title: t('platform.research.waves.window'), render: (_, w) => `${formatTime(w.window_start)} – ${formatTime(w.window_end)}` },
    { key: 'status', title: t('platform.research.common.status'), render: (_, w) => <StatusTag status={w.status} /> },
    { key: 'sessions', title: t('platform.research.waves.sessions'), render: (_, w) => w.cohorts?.length ?? 0 },
    {
      key: 'counts',
      title: t('platform.research.waves.counts'),
      render: (_, w) => t('platform.research.waves.countsValue', { invited: sum(w, 'invited_count'), completed: sum(w, 'completed_count'), absent: sum(w, 'absent_count') }),
    },
  ];
  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      <Typography.Paragraph type="secondary">{t('platform.research.waves.noScores')}</Typography.Paragraph>
      <ActionGate actionKey="action.platform_admin.efficacy_studies.schedule_waves">
        <Button onClick={() => setScheduling(true)} disabled={study.status === 'completed' || study.status === 'cancelled'}>
          {t('platform.research.waves.schedule')}
        </Button>
      </ActionGate>
      {waves.error ? <ProblemMessage error={waves.error} /> : null}
      <Table rowKey="wave_id" columns={columns} dataSource={waves.data?.items ?? []} loading={waves.isFetching} pagination={false} locale={{ emptyText: t('platform.research.common.empty') }} />
      {scheduling ? <ScheduleWaveDialog study={study} onClose={() => setScheduling(false)} /> : null}
    </Space>
  );
}

function ScheduleWaveDialog({ study, onClose }: { study: EfficacyStudy; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<{ kind: EfficacyWave['kind']; form?: 'a' | 'b'; window: [Dayjs, Dayjs]; duration_minutes?: number }>();
  const kind = Form.useWatch('kind', form);
  const schedule = useScheduleWave(study.study_id);
  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    schedule.mutate(
      {
        kind: v.kind,
        window_start: v.window[0].toISOString(),
        window_end: v.window[1].toISOString(),
        ...(v.kind === 'validation' && v.form ? { form: v.form } : {}),
        ...(v.duration_minutes ? { duration_seconds: v.duration_minutes * 60 } : {}),
      },
      {
        onSuccess: () => {
          message.success(t('platform.research.waves.scheduled'));
          onClose();
        },
        onError: (e) => applyFieldErrors(form, e, ['kind', 'form'], t, (k) => i18n.exists(k)),
      },
    );
  };
  return (
    <Modal open title={t('platform.research.waves.schedule')} okText={t('platform.research.common.save')} cancelText={t('platform.research.common.cancel')} confirmLoading={schedule.isPending} onOk={submit} onCancel={onClose} destroyOnHidden>
      <Typography.Paragraph type="secondary">{t('platform.research.waves.scheduleHelp', { weeks: study.min_intervention_weeks })}</Typography.Paragraph>
      {schedule.error ? <ProblemMessage error={schedule.error} /> : null}
      <Form form={form} name="efficacy_wave" layout="vertical" initialValues={{ kind: 'pre' }}>
        <Form.Item name="kind" label={t('platform.research.waves.kind')} rules={[{ required: true }]}>
          <Radio.Group options={WAVE_KINDS.map((k) => ({ value: k, label: t(`platform.research.waveKind.${k}`) }))} />
        </Form.Item>
        {kind === 'validation' ? (
          <Form.Item name="form" label={t('platform.research.waves.form')} rules={[{ required: true, message: t('platform.research.form.required') }]}>
            <Radio.Group options={(['a', 'b'] as const).map((f) => ({ value: f, label: t(`platform.research.formName.${f}`) }))} />
          </Form.Item>
        ) : null}
        <Form.Item name="window" label={t('platform.research.waves.window')} rules={[{ required: true, message: t('platform.research.form.required') }]}>
          <DatePicker.RangePicker showTime format="DD/MM/YYYY HH:mm" aria-label={t('platform.research.waves.window')} />
        </Form.Item>
        <BoundedNumberField name="duration_minutes" label={t('platform.research.waves.durationMinutes')} limit={{ source: L.waveDuration.source, minimum: 1, maximum: 240 }} required={false} />
      </Form>
    </Modal>
  );
}

// ---------- withdrawals ----------

function WithdrawTab({ study }: { study: EfficacyStudy }) {
  const { t, i18n } = useTranslation();
  const [form] = Form.useForm();
  const scope = Form.useWatch('scope', form) as (typeof WITHDRAW_SCOPES)[number] | undefined;
  const withdraw = useWithdraw(study.study_id);
  const [result, setResult] = useState<EfficacyWithdrawal | null>(null);
  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const body = {
      scope: v.scope,
      reason: v.reason.trim(),
      ...(v.school_request_ref?.trim() ? { school_request_ref: v.school_request_ref.trim() } : {}),
      ...(v.scope === 'school' ? { school_id: v.school_id } : {}),
      ...(v.scope === 'cohort' ? { school_id: v.school_id } : {}),
      ...(v.scope === 'students' ? { student_ids: v.student_ids } : {}),
    };
    withdraw.mutate(
      { tenantId: v.tenant_id, body },
      { onSuccess: setResult, onError: (e) => applyFieldErrors(form, e, ['scope', 'school_id', 'student_ids', 'reason', 'school_request_ref'], t, (k) => i18n.exists(k)) },
    );
  };
  return (
    <ActionGate actionKey="action.platform_admin.efficacy_studies.withdraw">
      <Form form={form} name="efficacy_withdraw" layout="vertical" initialValues={{ scope: 'students' }} style={{ maxWidth: 640 }}>
        <Typography.Paragraph type="secondary">{t('platform.research.withdraw.help')}</Typography.Paragraph>
        {withdraw.error ? <ProblemMessage error={withdraw.error} /> : null}
        {result ? <Alert type="success" showIcon message={t('platform.research.withdraw.done', { count: result.affected_count, sessions: result.cancelled_exam_session_count ?? 0 })} /> : null}
        <Form.Item name="tenant_id" label={t('platform.research.common.targetTenant')} rules={[{ required: true, message: t('platform.research.form.required') }]}>
          <TenantSelect ariaLabel={t('platform.research.common.targetTenant')} />
        </Form.Item>
        <Form.Item name="scope" label={t('platform.research.withdraw.scope')} rules={[{ required: true }]}>
          <Radio.Group options={WITHDRAW_SCOPES.map((s) => ({ value: s, label: t(`platform.research.withdraw.scopeKind.${s}`) }))} />
        </Form.Item>
        {scope === 'school' || scope === 'cohort' ? <UuidField name="school_id" label={t('platform.research.withdraw.schoolId')} /> : null}
        {scope === 'students' ? <UuidListField name="student_ids" label={t('platform.research.withdraw.studentIds')} limit={L.withdrawStudents} required /> : null}
        <LimitedField name="reason" label={t('platform.research.common.reason')} limit={L.withdrawReason} required textarea />
        <LimitedField name="school_request_ref" label={t('platform.research.withdraw.requestRef')} limit={L.schoolRequestRef} />
        <Button danger onClick={submit} loading={withdraw.isPending}>
          {t('platform.research.withdraw.submit')}
        </Button>
      </Form>
    </ActionGate>
  );
}

// ---------- report and dataset ----------

function ReportTab({ study }: { study: EfficacyStudy }) {
  const { t } = useTranslation();
  const [form] = Form.useForm<{ excluded?: string[] }>();
  const create = useCreateReport(study.study_id);
  const [reportId, setReportId] = useState<string>();
  const report = useReport(study.study_id, reportId);
  const exportDataset = useExportDataset(study);
  const [jobId, setJobId] = useState<string>();
  const job = usePlatformReport(jobId);
  const generate = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    create.mutate(v.excluded ?? [], { onSuccess: (r) => setReportId(r.report_id) });
  };
  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      <ActionGate actionKey="action.platform_admin.efficacy_studies.report">
        <Form form={form} name="efficacy_report" layout="vertical" style={{ maxWidth: 640 }}>
          <UuidListField name="excluded" label={t('platform.research.report.excluded')} limit={L.excludedQuestions} />
          <Button type="primary" onClick={generate} loading={create.isPending}>
            {t('platform.research.report.generate')}
          </Button>
        </Form>
      </ActionGate>
      {create.error ? <ProblemMessage error={create.error} /> : null}
      {report.error ? <ProblemMessage error={report.error} /> : null}
      {report.data ? <ReportView report={report.data} /> : null}
      <ActionGate actionKey="action.platform_admin.reports.export">
        <Space direction="vertical">
          <Typography.Title level={4}>{t('platform.research.dataset.title')}</Typography.Title>
          <Typography.Paragraph type="secondary">{t('platform.research.dataset.help')}</Typography.Paragraph>
          <Button onClick={() => exportDataset.mutate(undefined, { onSuccess: (j) => setJobId(j.report_id) })} loading={exportDataset.isPending}>
            {t('platform.research.dataset.export')}
          </Button>
          {exportDataset.error ? <ProblemMessage error={exportDataset.error} /> : null}
          {job.data ? (
            <Space>
              <StatusTag status={job.data.status} />
              {job.data.status === 'succeeded' && job.data.download_url ? (
                <Typography.Link href={job.data.download_url} target="_blank" rel="noreferrer">
                  {t('platform.research.dataset.download', { rows: job.data.row_count ?? 0, expires: formatTime(job.data.download_expires_at) })}
                </Typography.Link>
              ) : null}
            </Space>
          ) : null}
        </Space>
      </ActionGate>
    </Space>
  );
}
