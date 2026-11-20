// Menu "Thử nghiệm A/B mastery và gợi ý" (F035 US5/US8, F035-AC-21…AC-27, AC-35, AC-37): experiments list and
// creation (arms, allocation, mastery model version or rule_params), detail with start shadow/live, stop with a
// reason and the results by arm (small cells hidden by the backend, guardrail state). Assignment, guardrail and
// metrics are computed by the backend; the page only calls the API.
import { Alert, App as AntApp, Button, Checkbox, DatePicker, Descriptions, Form, Input, InputNumber, Modal, Popconfirm, Radio, Select, Space, Table, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import dayjs, { type Dayjs } from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Route, Routes, useParams } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { applyFieldErrors, formatTime, LimitedField, PageTitle, TenantSelect } from '../ui';
import {
  EXPERIMENT_KINDS,
  EXPERIMENT_STATUSES,
  RULE_PARAMS,
  useCreateExperiment,
  useExperiment,
  useExperimentResults,
  useExperiments,
  useStartExperiment,
  useStopExperiment,
  type ExperimentArm,
  type LearningExperiment,
  type LearningExperimentKind,
  type LearningExperimentStatus,
} from './api';
import { RESEARCH_LIMITS as L } from './limits';
import { StatusTag, UuidField } from './ResearchForms';

export function LearningExperimentsScreen({ title }: { title: string }) {
  return (
    <Routes>
      <Route index element={<ExperimentList title={title} />} />
      <Route path=":experimentId" element={<ExperimentDetail />} />
    </Routes>
  );
}

function ExperimentList({ title }: { title: string }) {
  const { t } = useTranslation();
  const [status, setStatus] = useState<LearningExperimentStatus | undefined>();
  const [kind, setKind] = useState<LearningExperimentKind | undefined>();
  const [creating, setCreating] = useState(false);
  const list = useExperiments(status, kind);
  const columns: ColumnsType<LearningExperiment> = [
    { key: 'code', title: t('platform.research.experiments.code'), render: (_, e) => <Link to={e.experiment_id}>{e.code}</Link> },
    { key: 'kind', title: t('platform.research.experiments.kind'), render: (_, e) => t(`platform.research.experimentKind.${e.kind}`) },
    { key: 'unit', title: t('platform.research.experiments.unit'), render: (_, e) => t(`platform.research.unit.${e.unit}`) },
    { key: 'status', title: t('platform.research.common.status'), render: (_, e) => <StatusTag status={e.status} /> },
    { key: 'arms', title: t('platform.research.experiments.arms'), render: (_, e) => e.arms.map((a) => `${a.code} ${a.allocation_pct}%`).join(' · ') },
    { key: 'period', title: t('platform.research.experiments.period'), render: (_, e) => `${formatTime(e.start_at)} – ${formatTime(e.end_at)}` },
    { key: 'tenants', title: t('platform.research.studies.tenants'), render: (_, e) => e.tenant_ids.length },
    { key: 'stop', title: t('platform.research.experiments.stopReason'), render: (_, e) => (e.stop_reason ? t(`platform.research.stopReason.${e.stop_reason}`) : null) },
  ];
  return (
    <section>
      <PageTitle title={title} />
      <Typography.Paragraph type="secondary">{t('platform.research.experiments.intro')}</Typography.Paragraph>
      <Space wrap>
        <Select aria-label={t('platform.research.common.status')} allowClear placeholder={t('platform.research.common.allStatuses')} value={status} onChange={setStatus} options={EXPERIMENT_STATUSES.map((x) => ({ value: x, label: t(`platform.research.status.${x}`) }))} />
        <Select aria-label={t('platform.research.experiments.kind')} allowClear placeholder={t('platform.research.experiments.allKinds')} value={kind} onChange={setKind} options={EXPERIMENT_KINDS.map((x) => ({ value: x, label: t(`platform.research.experimentKind.${x}`) }))} />
        <ActionGate actionKey="action.platform_admin.learning_experiments.edit">
          <Button type="primary" onClick={() => setCreating(true)}>
            {t('platform.research.experiments.create')}
          </Button>
        </ActionGate>
      </Space>
      {list.error ? <ProblemMessage error={list.error} /> : null}
      <Table rowKey="experiment_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} locale={{ emptyText: t('platform.research.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('platform.research.common.loadMore')}
        </Button>
      ) : null}
      {creating ? <CreateExperimentDialog onClose={() => setCreating(false)} /> : null}
    </section>
  );
}

interface ArmValues {
  code: string;
  allocation_pct: number;
  is_control?: boolean;
  model_version?: string;
  rule_params?: Partial<Record<(typeof RULE_PARAMS)[number], number | null>>;
}

interface ExperimentValues {
  code: string;
  kind: LearningExperimentKind;
  unit: 'class' | 'student';
  tenant_ids: string[];
  study_id?: string;
  period: [Dayjs, Dayjs];
  arms: ArmValues[];
}

/** Form arm → ExperimentArm: only the parameters typed, none on a control arm (backend refuses them). */
function armOf(a: ArmValues, kind: LearningExperimentKind): ExperimentArm {
  const arm: ExperimentArm = { code: a.code.trim(), allocation_pct: a.allocation_pct, is_control: !!a.is_control };
  if (kind === 'mastery_model' && a.model_version?.trim()) arm.model_version = a.model_version.trim();
  if (kind === 'recommendation_rule' && !a.is_control) {
    const params = Object.fromEntries(Object.entries(a.rule_params ?? {}).filter(([, v]) => v !== undefined && v !== null));
    if (Object.keys(params).length) arm.rule_params = params;
  }
  return arm;
}

function CreateExperimentDialog({ onClose }: { onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<ExperimentValues>();
  const kind = Form.useWatch('kind', form) ?? 'mastery_model';
  const create = useCreateExperiment();
  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    create.mutate(
      {
        code: v.code.trim(),
        kind: v.kind,
        unit: v.unit,
        tenant_ids: v.tenant_ids,
        start_at: v.period[0].toISOString(),
        end_at: v.period[1].toISOString(),
        arms: v.arms.map((a) => armOf(a, v.kind)),
        ...(v.study_id ? { study_id: v.study_id } : {}),
      },
      {
        onSuccess: () => {
          message.success(t('platform.research.experiments.created'));
          onClose();
        },
        onError: (e) => applyFieldErrors(form, e, ['code', 'kind', 'unit', 'tenant_ids', 'study_id', 'arms'], t, (k) => i18n.exists(k)),
      },
    );
  };
  const initialArms: ArmValues[] = [
    { code: 'control', allocation_pct: 50, is_control: true },
    { code: 'test', allocation_pct: 50, is_control: false },
  ];
  return (
    <Modal open width={760} title={t('platform.research.experiments.create')} okText={t('platform.research.common.save')} cancelText={t('platform.research.common.cancel')} confirmLoading={create.isPending} onOk={submit} onCancel={onClose} destroyOnHidden>
      <Typography.Paragraph type="secondary">{t('platform.research.experiments.createHelp')}</Typography.Paragraph>
      {create.error ? <ProblemMessage error={create.error} /> : null}
      <Form form={form} name="learning_experiment" layout="vertical" initialValues={{ kind: 'mastery_model', unit: 'class', arms: initialArms, period: [dayjs().startOf('hour'), dayjs().startOf('hour').add(30, 'day')] }}>
        <LimitedField name="code" label={t('platform.research.experiments.code')} limit={L.experimentCode} required />
        <Form.Item name="kind" label={t('platform.research.experiments.kind')} rules={[{ required: true }]}>
          <Radio.Group options={EXPERIMENT_KINDS.map((k) => ({ value: k, label: t(`platform.research.experimentKind.${k}`) }))} />
        </Form.Item>
        <Form.Item name="unit" label={t('platform.research.experiments.unit')} extra={t('platform.research.experiments.unitHelp')}>
          <Radio.Group options={(['class', 'student'] as const).map((u) => ({ value: u, label: t(`platform.research.unit.${u}`) }))} />
        </Form.Item>
        <Form.Item name="tenant_ids" label={t('platform.research.experiments.tenants')} rules={[{ required: true, message: t('platform.research.form.required') }]}>
          <TenantSelect multiple maxCount={L.experimentTenants.maxItems} ariaLabel={t('platform.research.experiments.tenants')} />
        </Form.Item>
        <UuidField name="study_id" label={t('platform.research.experiments.study')} required={false} />
        <Form.Item name="period" label={t('platform.research.experiments.period')} rules={[{ required: true, message: t('platform.research.form.required') }]}>
          <DatePicker.RangePicker showTime format="DD/MM/YYYY HH:mm" aria-label={t('platform.research.experiments.period')} />
        </Form.Item>
        <Form.List name="arms">
          {(fields, { add, remove }) => (
            <Space direction="vertical" style={{ width: '100%' }}>
              <Typography.Text strong>{t('platform.research.experiments.arms')}</Typography.Text>
              {fields.map((f) => (
                <Space key={f.key} wrap align="start">
                  <Form.Item name={[f.name, 'code']} label={t('platform.research.experiments.armCode')} rules={[{ required: true, pattern: new RegExp(L.armCode.pattern), message: t('platform.research.experiments.armCodeRule') }]}>
                    <Input aria-label={t('platform.research.experiments.armCode')} maxLength={L.armCode.maxLength} />
                  </Form.Item>
                  <Form.Item name={[f.name, 'allocation_pct']} label={t('platform.research.experiments.allocation')} rules={[{ required: true, message: t('platform.research.form.required') }]}>
                    <InputNumber aria-label={t('platform.research.experiments.allocation')} min={L.armAllocation.minimum} max={L.armAllocation.maximum} precision={0} />
                  </Form.Item>
                  <Form.Item name={[f.name, 'is_control']} valuePropName="checked" label={t('platform.research.experiments.control')}>
                    <Checkbox aria-label={t('platform.research.experiments.control')} />
                  </Form.Item>
                  {kind === 'mastery_model' ? (
                    <Form.Item name={[f.name, 'model_version']} label={t('platform.research.experiments.modelVersion')} rules={[{ required: true, message: t('platform.research.form.required') }]}>
                      <Input aria-label={t('platform.research.experiments.modelVersion')} maxLength={L.modelVersion.maxLength} />
                    </Form.Item>
                  ) : (
                    RULE_PARAMS.map((p) => (
                      <Form.Item key={p} name={[f.name, 'rule_params', p]} label={t(`platform.research.ruleParam.${p}`)}>
                        <InputNumber aria-label={t(`platform.research.ruleParam.${p}`)} step={p === 'review_threshold' ? 0.05 : 1} />
                      </Form.Item>
                    ))
                  )}
                  {fields.length > 2 ? (
                    <Button onClick={() => remove(f.name)} aria-label={t('platform.research.experiments.removeArm')}>
                      {t('platform.research.experiments.removeArm')}
                    </Button>
                  ) : null}
                </Space>
              ))}
              {fields.length < 5 ? <Button onClick={() => add({ code: '', allocation_pct: 10, is_control: false })}>{t('platform.research.experiments.addArm')}</Button> : null}
            </Space>
          )}
        </Form.List>
      </Form>
    </Modal>
  );
}

function ExperimentDetail() {
  const { t } = useTranslation();
  const { experimentId = '' } = useParams();
  const q = useExperiment(experimentId);
  const e = q.data;
  return (
    <section>
      <Link to=".." relative="path">
        {t('platform.research.common.back')}
      </Link>
      {q.error ? <ProblemMessage error={q.error} /> : null}
      {e ? (
        <>
          <PageTitle title={e.code} extra={<ExperimentActions experiment={e} />} />
          <Descriptions size="small" column={{ xs: 1, md: 3 }}>
            <Descriptions.Item label={t('platform.research.common.status')}>
              <StatusTag status={e.status} />
            </Descriptions.Item>
            <Descriptions.Item label={t('platform.research.experiments.kind')}>{t(`platform.research.experimentKind.${e.kind}`)}</Descriptions.Item>
            <Descriptions.Item label={t('platform.research.experiments.unit')}>{t(`platform.research.unit.${e.unit}`)}</Descriptions.Item>
            <Descriptions.Item label={t('platform.research.experiments.period')}>{`${formatTime(e.start_at)} – ${formatTime(e.end_at)}`}</Descriptions.Item>
            <Descriptions.Item label={t('platform.research.experiments.revision')}>{e.revision}</Descriptions.Item>
            <Descriptions.Item label={t('platform.research.studies.tenants')}>{e.tenant_ids.length}</Descriptions.Item>
            {e.stop_reason ? <Descriptions.Item label={t('platform.research.experiments.stopReason')}>{t(`platform.research.stopReason.${e.stop_reason}`)}</Descriptions.Item> : null}
          </Descriptions>
          <Table
            rowKey="code"
            size="small"
            pagination={false}
            dataSource={e.arms}
            columns={[
              { key: 'code', title: t('platform.research.experiments.armCode'), render: (_, a) => a.code },
              { key: 'alloc', title: t('platform.research.experiments.allocation'), render: (_, a) => `${a.allocation_pct}%` },
              { key: 'control', title: t('platform.research.experiments.control'), render: (_, a) => (a.is_control ? t('platform.research.common.yes') : t('platform.research.common.no')) },
              { key: 'config', title: t('platform.research.experiments.armConfig'), render: (_, a) => a.model_version ?? (a.rule_params ? JSON.stringify(a.rule_params) : null) },
            ]}
          />
          <Results experimentId={e.experiment_id} />
        </>
      ) : null}
    </section>
  );
}

function ExperimentActions({ experiment }: { experiment: LearningExperiment }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const start = useStartExperiment(experiment.experiment_id);
  const [stopping, setStopping] = useState(false);
  const running = experiment.status === 'shadow' || experiment.status === 'live';
  const startAs = (exposure: 'shadow' | 'live') => start.mutate(exposure, { onSuccess: () => message.success(t(`platform.research.experiments.started.${exposure}`)) });
  return (
    <Space wrap>
      {start.error ? <ProblemMessage error={start.error} /> : null}
      <ActionGate actionKey="action.platform_admin.learning_experiments.operate">
        <Space wrap>
          {experiment.status === 'draft' ? (
            <Button onClick={() => startAs('shadow')} loading={start.isPending}>
              {t('platform.research.experiments.startShadow')}
            </Button>
          ) : null}
          {experiment.status === 'draft' || experiment.status === 'shadow' ? (
            <Popconfirm title={t('platform.research.experiments.liveConfirm')} okText={t('platform.research.experiments.startLive')} cancelText={t('platform.research.common.cancel')} onConfirm={() => startAs('live')}>
              <Button type="primary" loading={start.isPending}>
                {t('platform.research.experiments.startLive')}
              </Button>
            </Popconfirm>
          ) : null}
          {running ? (
            <Button danger onClick={() => setStopping(true)}>
              {t('platform.research.experiments.stop')}
            </Button>
          ) : null}
        </Space>
      </ActionGate>
      {stopping ? <StopDialog experiment={experiment} onClose={() => setStopping(false)} /> : null}
    </Space>
  );
}

function StopDialog({ experiment, onClose }: { experiment: LearningExperiment; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<{ reason: string }>();
  const stop = useStopExperiment(experiment.experiment_id);
  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    stop.mutate(v.reason.trim(), {
      onSuccess: () => {
        message.success(t('platform.research.experiments.stopped'));
        onClose();
      },
      onError: (e) => applyFieldErrors(form, e, ['reason'], t, (k) => i18n.exists(k)),
    });
  };
  return (
    <Modal open title={t('platform.research.experiments.stopTitle', { code: experiment.code })} okText={t('platform.research.experiments.stop')} okButtonProps={{ danger: true }} cancelText={t('platform.research.common.cancel')} confirmLoading={stop.isPending} onOk={submit} onCancel={onClose} destroyOnHidden>
      <Typography.Paragraph type="secondary">{t('platform.research.experiments.stopHelp')}</Typography.Paragraph>
      {stop.error ? <ProblemMessage error={stop.error} /> : null}
      <Form form={form} name="stop_experiment" layout="vertical">
        <LimitedField name="reason" label={t('platform.research.common.reason')} limit={L.stopReason} required textarea />
      </Form>
    </Modal>
  );
}

function Results({ experimentId }: { experimentId: string }) {
  const { t } = useTranslation();
  const q = useExperimentResults(experimentId);
  return (
    <section aria-label={t('platform.research.experiments.results')}>
      <Typography.Title level={4}>{t('platform.research.experiments.results')}</Typography.Title>
      {q.error ? <Typography.Paragraph type="secondary">{t('platform.research.experiments.noResults')}</Typography.Paragraph> : null}
      {q.data ? (
        <>
          <Typography.Paragraph type="secondary">{t('platform.research.experiments.computedAt', { at: formatTime(q.data.computed_at) })}</Typography.Paragraph>
          {q.data.guardrail_breached ? <Alert type="error" showIcon message={t('platform.research.experiments.guardrailBreached')} /> : null}
          <Table
            rowKey="arm_code"
            size="small"
            pagination={false}
            dataSource={q.data.arms}
            columns={[
              { key: 'arm', title: t('platform.research.experiments.armCode'), render: (_, a) => a.arm_code },
              { key: 'suppressed', title: t('platform.research.experiments.cell'), render: (_, a) => (a.suppressed ? t('platform.research.report.hidden') : t('platform.research.experiments.shown')) },
              { key: 'units', title: t('platform.research.experiments.units'), render: (_, a) => a.unit_count ?? null },
              { key: 'students', title: t('platform.research.experiments.students'), render: (_, a) => a.student_count ?? null },
              { key: 'primary', title: t('platform.research.experiments.primary'), render: (_, a) => a.primary_metric ?? null },
              { key: 'logloss', title: t('platform.research.experiments.logLoss'), render: (_, a) => a.next_item_log_loss ?? null },
              { key: 'minutes', title: t('platform.research.experiments.minutes'), render: (_, a) => a.active_minutes_avg ?? null },
            ]}
          />
        </>
      ) : null}
    </section>
  );
}
