// Menu "Nghiên cứu hiệu quả học tập" (F035 US8, F035-AC-35, AC-37; LEARNING_EFFICACY_PLAN): research agreements of the
// tenants (tenant_ids[] mandatory, X-Tenant-Id on writes), the efficacy studies (list, create) and the study detail
// (route `:studyId`). Every button sits in an ActionGate of `menu.platform_admin.efficacy_studies`; the API decides.
import { App as AntApp, Button, Checkbox, DatePicker, Form, Modal, Select, Space, Table, Tabs, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import type { Dayjs } from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Route, Routes } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useTenantDirectory } from '../api/tenants';
import { applyFieldErrors, formatTime, LimitedField, PageTitle, TenantCell, TenantSelect } from '../ui';
import {
  STUDY_STATUSES,
  useAgreements,
  useCreateAgreement,
  useCreateStudy,
  useRevokeAgreement,
  useStudies,
  type EfficacyStudy,
  type EfficacyStudyStatus,
  type ResearchAgreement,
} from './api';
import { RESEARCH_LIMITS as L } from './limits';
import { BoundedNumberField, StatusTag, UuidField, UuidListField } from './ResearchForms';
import { StudyDetail } from './StudyDetail';

export function EfficacyStudiesScreen({ title }: { title: string }) {
  return (
    <Routes>
      <Route index element={<StudiesHome title={title} />} />
      <Route path=":studyId" element={<StudyDetail />} />
    </Routes>
  );
}

function StudiesHome({ title }: { title: string }) {
  const { t } = useTranslation();
  return (
    <section>
      <PageTitle title={title} />
      <Typography.Paragraph type="secondary">{t('platform.research.studies.intro')}</Typography.Paragraph>
      <Tabs
        items={[
          { key: 'studies', label: t('platform.research.studies.tab'), children: <StudyList /> },
          { key: 'agreements', label: t('platform.research.agreements.tab'), children: <AgreementsPanel /> },
        ]}
      />
    </section>
  );
}

function StudyList() {
  const { t } = useTranslation();
  const [status, setStatus] = useState<EfficacyStudyStatus | undefined>();
  const [creating, setCreating] = useState(false);
  const list = useStudies(status);
  const columns: ColumnsType<EfficacyStudy> = [
    { key: 'code', title: t('platform.research.studies.code'), render: (_, s) => <Link to={s.study_id}>{s.code}</Link> },
    { key: 'title', title: t('platform.research.studies.title'), render: (_, s) => s.title },
    { key: 'grade', title: t('platform.research.studies.grade'), render: (_, s) => s.grade_level },
    { key: 'status', title: t('platform.research.common.status'), render: (_, s) => <StatusTag status={s.status} /> },
    { key: 'cohorts', title: t('platform.research.studies.cohorts'), render: (_, s) => t('platform.research.studies.cohortCount', { cohorts: s.cohort_count ?? 0, pairs: s.pair_count ?? 0 }) },
    { key: 'tenants', title: t('platform.research.studies.tenants'), render: (_, s) => s.tenant_ids?.length ?? 0 },
    { key: 'created', title: t('platform.research.common.createdAt'), render: (_, s) => formatTime(s.created_at) },
  ];
  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      <Space wrap>
        <Select
          aria-label={t('platform.research.common.status')}
          allowClear
          placeholder={t('platform.research.common.allStatuses')}
          value={status}
          onChange={setStatus}
          options={STUDY_STATUSES.map((x) => ({ value: x, label: t(`platform.research.status.${x}`) }))}
        />
        <ActionGate actionKey="action.platform_admin.efficacy_studies.edit">
          <Button type="primary" onClick={() => setCreating(true)}>
            {t('platform.research.studies.create')}
          </Button>
        </ActionGate>
      </Space>
      {list.error ? <ProblemMessage error={list.error} /> : null}
      <Table rowKey="study_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} locale={{ emptyText: t('platform.research.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('platform.research.common.loadMore')}
        </Button>
      ) : null}
      {creating ? <CreateStudyDialog onClose={() => setCreating(false)} /> : null}
    </Space>
  );
}

const STUDY_FIELDS = ['code', 'title', 'subject_id', 'grade_level', 'objective_ids', 'form_a_blueprint_version_id', 'form_b_blueprint_version_id', 'min_intervention_weeks'];

function CreateStudyDialog({ onClose }: { onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm();
  const create = useCreateStudy();
  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    create.mutate(
      { ...v, code: v.code.trim(), title: v.title.trim() },
      {
        onSuccess: () => {
          message.success(t('platform.research.studies.created'));
          onClose();
        },
        onError: (e) => applyFieldErrors(form, e, STUDY_FIELDS, t, (k) => i18n.exists(k)),
      },
    );
  };
  return (
    <Modal open title={t('platform.research.studies.create')} okText={t('platform.research.common.save')} cancelText={t('platform.research.common.cancel')} confirmLoading={create.isPending} onOk={submit} onCancel={onClose} destroyOnHidden>
      <Typography.Paragraph type="secondary">{t('platform.research.studies.createHelp')}</Typography.Paragraph>
      {create.error ? <ProblemMessage error={create.error} /> : null}
      <Form form={form} name="efficacy_study" layout="vertical" initialValues={{ min_intervention_weeks: 8 }}>
        <LimitedField name="code" label={t('platform.research.studies.code')} limit={L.studyCode} required />
        <LimitedField name="title" label={t('platform.research.studies.title')} limit={L.studyTitle} required />
        <UuidField name="subject_id" label={t('platform.research.studies.subject')} />
        <BoundedNumberField name="grade_level" label={t('platform.research.studies.grade')} limit={{ source: 'GradeLevel', minimum: 1, maximum: 9 }} />
        <UuidListField name="objective_ids" label={t('platform.research.studies.objectives')} limit={L.studyObjectives} required />
        <UuidField name="form_a_blueprint_version_id" label={t('platform.research.studies.formA')} />
        <UuidField name="form_b_blueprint_version_id" label={t('platform.research.studies.formB')} />
        <BoundedNumberField name="min_intervention_weeks" label={t('platform.research.studies.minWeeks')} limit={L.minWeeks} />
      </Form>
    </Modal>
  );
}

// ---------- agreements ----------

function AgreementsPanel() {
  const { t } = useTranslation();
  const directory = useTenantDirectory();
  const [tenantIds, setTenantIds] = useState<string[]>([]);
  const [status, setStatus] = useState<ResearchAgreement['status'] | undefined>();
  const [creating, setCreating] = useState(false);
  const [revoking, setRevoking] = useState<ResearchAgreement | null>(null);
  const list = useAgreements(tenantIds, status);
  const columns: ColumnsType<ResearchAgreement> = [
    { key: 'tenant', title: t('platform.research.common.tenant'), render: (_, a) => <TenantCell tenantId={a.tenant_id} directory={directory} /> },
    { key: 'ref', title: t('platform.research.agreements.ref'), render: (_, a) => a.agreement_ref },
    { key: 'covers', title: t('platform.research.agreements.covers'), render: (_, a) => a.covers.map((c) => <Tag key={c}>{t(`platform.research.agreements.cover.${c}`)}</Tag>) },
    { key: 'agreed', title: t('platform.research.agreements.agreedOn'), render: (_, a) => a.agreed_on },
    { key: 'status', title: t('platform.research.common.status'), render: (_, a) => <StatusTag status={a.status} /> },
    { key: 'revoked', title: t('platform.research.agreements.revokedAt'), render: (_, a) => formatTime(a.revoked_at) },
    {
      key: 'actions',
      title: t('platform.research.common.actions'),
      render: (_, a) =>
        a.status === 'active' ? (
          <ActionGate actionKey="action.platform_admin.efficacy_studies.manage_agreements">
            <Button size="small" danger onClick={() => setRevoking(a)}>
              {t('platform.research.agreements.revoke')}
            </Button>
          </ActionGate>
        ) : null,
    },
  ];
  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      <Space wrap>
        <TenantSelect multiple maxCount={100} ariaLabel={t('platform.research.agreements.tenants')} value={tenantIds} onChange={(v) => setTenantIds(v as string[])} />
        <Select
          aria-label={t('platform.research.common.status')}
          allowClear
          placeholder={t('platform.research.common.allStatuses')}
          value={status}
          onChange={setStatus}
          options={(['active', 'revoked'] as const).map((x) => ({ value: x, label: t(`platform.research.status.${x}`) }))}
        />
        <ActionGate actionKey="action.platform_admin.efficacy_studies.manage_agreements">
          <Button type="primary" onClick={() => setCreating(true)}>
            {t('platform.research.agreements.create')}
          </Button>
        </ActionGate>
      </Space>
      {tenantIds.length === 0 ? <Typography.Paragraph type="secondary">{t('platform.research.agreements.pickTenants')}</Typography.Paragraph> : null}
      {list.error ? <ProblemMessage error={list.error} /> : null}
      <Table rowKey="agreement_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} locale={{ emptyText: t('platform.research.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('platform.research.common.loadMore')}
        </Button>
      ) : null}
      {creating ? <CreateAgreementDialog onClose={() => setCreating(false)} /> : null}
      {revoking ? <RevokeAgreementDialog agreement={revoking} onClose={() => setRevoking(null)} /> : null}
    </Space>
  );
}

function CreateAgreementDialog({ onClose }: { onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<{ tenant_id: string; agreement_ref: string; covers: ('efficacy_study' | 'experiment')[]; agreed_on: Dayjs }>();
  const create = useCreateAgreement();
  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    create.mutate(
      { tenantId: v.tenant_id, body: { agreement_ref: v.agreement_ref.trim(), covers: v.covers, agreed_on: v.agreed_on.format('YYYY-MM-DD') } },
      {
        onSuccess: () => {
          message.success(t('platform.research.agreements.created'));
          onClose();
        },
        onError: (e) => applyFieldErrors(form, e, ['agreement_ref', 'covers', 'agreed_on'], t, (k) => i18n.exists(k)),
      },
    );
  };
  return (
    <Modal open title={t('platform.research.agreements.create')} okText={t('platform.research.common.save')} cancelText={t('platform.research.common.cancel')} confirmLoading={create.isPending} onOk={submit} onCancel={onClose} destroyOnHidden>
      {create.error ? <ProblemMessage error={create.error} /> : null}
      <Form form={form} name="research_agreement" layout="vertical" initialValues={{ covers: ['efficacy_study'] }}>
        <Form.Item name="tenant_id" label={t('platform.research.common.targetTenant')} rules={[{ required: true, message: t('platform.research.form.required') }]}>
          <TenantSelect ariaLabel={t('platform.research.common.targetTenant')} />
        </Form.Item>
        <LimitedField name="agreement_ref" label={t('platform.research.agreements.ref')} limit={L.agreementRef} required />
        <Form.Item name="covers" label={t('platform.research.agreements.covers')} rules={[{ required: true, message: t('platform.research.form.required') }]}>
          <Checkbox.Group options={(['efficacy_study', 'experiment'] as const).map((c) => ({ value: c, label: t(`platform.research.agreements.cover.${c}`) }))} />
        </Form.Item>
        <Form.Item name="agreed_on" label={t('platform.research.agreements.agreedOn')} rules={[{ required: true, message: t('platform.research.form.required') }]}>
          <DatePicker aria-label={t('platform.research.agreements.agreedOn')} format="DD/MM/YYYY" />
        </Form.Item>
      </Form>
    </Modal>
  );
}

function RevokeAgreementDialog({ agreement, onClose }: { agreement: ResearchAgreement; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<{ reason: string }>();
  const revoke = useRevokeAgreement();
  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    revoke.mutate(
      { agreement, reason: v.reason.trim() },
      {
        onSuccess: () => {
          message.success(t('platform.research.agreements.revoked'));
          onClose();
        },
        onError: (e) => applyFieldErrors(form, e, ['reason'], t, (k) => i18n.exists(k)),
      },
    );
  };
  return (
    <Modal open title={t('platform.research.agreements.revokeTitle', { ref: agreement.agreement_ref })} okText={t('platform.research.agreements.revoke')} okButtonProps={{ danger: true }} cancelText={t('platform.research.common.cancel')} confirmLoading={revoke.isPending} onOk={submit} onCancel={onClose} destroyOnHidden>
      <Typography.Paragraph type="secondary">{t('platform.research.agreements.revokeHelp')}</Typography.Paragraph>
      {revoke.error ? <ProblemMessage error={revoke.error} /> : null}
      <Form form={form} name="revoke_agreement" layout="vertical">
        <LimitedField name="reason" label={t('platform.research.common.reason')} limit={L.revokeReason} required textarea />
      </Form>
    </Modal>
  );
}
