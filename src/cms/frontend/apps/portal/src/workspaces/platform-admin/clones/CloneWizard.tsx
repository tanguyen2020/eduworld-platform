// Clone wizard (F057 T025, F057-AC-07/08, ADMIN_PORTAL §3.7, AGENTS R22): 1. source tenant + type (configuration and
// content only, no student data) + record ids (≤ maxItems of TenantCloneRequest); 2. exactly one target tenant, the
// source is not offered and there is no "all tenants"; 3. dependency preview — skipped while previewTenantClone does
// not exist (said on screen); 4. run createTenantClone (Idempotency-Key) and follow the job (CloneJobView).
import { Alert, Button, Form, Select, Space, Steps, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { CLONABLE_TYPES, CLONE_PREVIEW_AVAILABLE, useCreateClone, type TenantCloneRequest } from '../api/clones';
import type { ResourceType } from '../api/common';
import { useTenantDirectory } from '../api/tenants';
import { isUuid, LIMITS } from '../limits';
import { TenantSelect } from '../ui';
import { CloneJobView } from './CloneJobPage';

interface SourceValues {
  source_tenant_id: string;
  resource_type: ResourceType;
  resource_ids: string[];
}

export function CloneWizard() {
  const { t } = useTranslation();
  const [step, setStep] = useState(0);
  const [source, setSource] = useState<SourceValues | null>(null);
  const [target, setTarget] = useState<string | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const [sourceForm] = Form.useForm<SourceValues>();
  const [targetForm] = Form.useForm<{ target_tenant_id: string }>();
  const create = useCreateClone();
  const max = LIMITS.cloneResourceIds.maxItems;
  const directory = useTenantDirectory();
  const nameOf = (id: string) => directory.get(id)?.name ?? id;

  const steps = [
    { title: t('platform.clone.stepSource') },
    { title: t('platform.clone.stepTarget') },
    { title: t('platform.clone.stepPreview') },
    { title: t('platform.clone.stepRun') },
  ];

  if (jobId) {
    return (
      <Space direction="vertical">
        <CloneJobView jobId={jobId} />
        <Button
          onClick={() => {
            setJobId(null);
            setSource(null);
            setTarget(null);
            setStep(0);
            create.reset();
          }}
        >
          {t('platform.clone.newClone')}
        </Button>
      </Space>
    );
  }

  const run = () => {
    if (!source || !target) return;
    const body: TenantCloneRequest = { source_tenant_id: source.source_tenant_id, target_tenant_id: target, resource_type: source.resource_type, resource_ids: source.resource_ids };
    create.mutate(body, { onSuccess: (job) => setJobId(job.job_id) });
  };

  return (
    <Space direction="vertical">
      <Steps current={step} items={steps} />
      <Alert type="info" showIcon message={t('platform.clone.noStudentData')} />
      {step === 0 ? (
        <Form
          form={sourceForm}
          name="clone_source"
          layout="vertical"
          initialValues={source ?? { resource_ids: [] }}
          onFinish={(v) => {
            setSource({ ...v, resource_ids: [...new Set(v.resource_ids.map((x) => x.trim().toLowerCase()))] });
            setStep(1);
          }}
        >
          <Form.Item name="source_tenant_id" label={t('platform.clone.sourceTenant')} rules={[{ required: true, message: t('shell.tenantTarget.required') }]}>
            <TenantSelect ariaLabel={t('platform.clone.sourceTenant')} />
          </Form.Item>
          <Form.Item name="resource_type" label={t('platform.tenantData.resourceType')} rules={[{ required: true }]}>
            <Select aria-label={t('platform.tenantData.resourceType')} options={CLONABLE_TYPES.map((x) => ({ value: x, label: t(`platform.resourceType.${x}`) }))} />
          </Form.Item>
          <Form.Item
            name="resource_ids"
            label={t('platform.clone.resourceIds')}
            extra={t('platform.clone.resourceIdsHelp', { max })}
            rules={[
              {
                validator: (_, v: string[] = []) => {
                  if (v.length === 0) return Promise.reject(new Error(t('platform.clone.resourceIdsRequired')));
                  if (v.length > max) return Promise.reject(new Error(t('platform.common.tooMany', { max })));
                  if (v.some((x) => !isUuid(x.trim()))) return Promise.reject(new Error(t('platform.common.uuidInvalid')));
                  return Promise.resolve();
                },
              },
            ]}
          >
            <Select aria-label={t('platform.clone.resourceIds')} mode="tags" tokenSeparators={[',', ' ', '\n']} open={false} maxCount={max} />
          </Form.Item>
          <Button type="primary" htmlType="submit">
            {t('platform.clone.next')}
          </Button>
        </Form>
      ) : null}
      {step === 1 && source ? (
        <Form
          form={targetForm}
          name="clone_target"
          layout="vertical"
          initialValues={target ? { target_tenant_id: target } : {}}
          onFinish={(v) => {
            setTarget(v.target_tenant_id);
            setStep(2);
          }}
        >
          <Form.Item
            name="target_tenant_id"
            label={t('platform.clone.targetTenant')}
            rules={[
              { required: true, message: t('platform.clone.targetRequired') },
              { validator: (_, v?: string) => (v && v === source.source_tenant_id ? Promise.reject(new Error(t('platform.clone.targetSameAsSource'))) : Promise.resolve()) },
            ]}
          >
            <TenantSelect ariaLabel={t('platform.clone.targetTenant')} exclude={[source.source_tenant_id]} />
          </Form.Item>
          <Space>
            <Button onClick={() => setStep(0)}>{t('platform.clone.back')}</Button>
            <Button type="primary" htmlType="submit">
              {t('platform.clone.next')}
            </Button>
          </Space>
        </Form>
      ) : null}
      {step === 2 ? (
        <Space direction="vertical">
          {CLONE_PREVIEW_AVAILABLE ? null : <Alert type="warning" showIcon message={t('platform.clone.previewUnavailable')} />}
          <Space>
            <Button onClick={() => setStep(1)}>{t('platform.clone.back')}</Button>
            <Button type="primary" onClick={() => setStep(3)}>
              {t('platform.clone.next')}
            </Button>
          </Space>
        </Space>
      ) : null}
      {step === 3 && source && target ? (
        <Space direction="vertical">
          <Typography.Paragraph>
            {t('platform.clone.summary', { count: source.resource_ids.length, type: t(`platform.resourceType.${source.resource_type}`), source: nameOf(source.source_tenant_id), target: nameOf(target) })}
          </Typography.Paragraph>
          {create.error ? <ProblemMessage error={create.error} /> : null}
          <Space>
            <Button onClick={() => setStep(2)}>{t('platform.clone.back')}</Button>
            <Button type="primary" loading={create.isPending} onClick={run}>
              {t('platform.clone.run')}
            </Button>
          </Space>
        </Space>
      ) : null}
    </Space>
  );
}
