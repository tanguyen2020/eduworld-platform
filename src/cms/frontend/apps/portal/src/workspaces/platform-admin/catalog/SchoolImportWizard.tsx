// School CSV import (F057 T033, F057-AC-13, FR-ORG-016): one target tenant (mandatory, no "Platform", no "all"), a CSV
// file, dry-run first (importSchools `dry_run=true`, Idempotency-Key, X-Tenant-Id = target), job followed through
// getSchoolImport, errors per row (`row`, `field`, `code`); applying (`dry_run=false`) only after the user confirms.
import { App as AntApp, Alert, Button, Form, Space, Table, Tag, Typography, Upload } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { TenantTargetField } from '../../../shell/TenantControls';
import { isFinalJob } from '../api/common';
import { fileKeyOf, useImportSchools, useSchoolImport, type SchoolImportJob } from '../api/schoolImports';

type ImportError = NonNullable<SchoolImportJob['errors']>[number];

function ImportResult({ job, title }: { job: SchoolImportJob; title: string }) {
  const { t } = useTranslation();
  return (
    <section aria-label={title}>
      <Typography.Title level={3}>{title}</Typography.Title>
      <Space>
        <Tag data-testid="import-status">{t(`platform.import.status.${job.status}`)}</Tag>
        <Typography.Text>
          {t('platform.import.totals', { total: job.total_rows ?? 0, created: job.created_rows ?? 0, updated: job.updated_rows ?? 0, errors: job.error_rows ?? 0 })}
        </Typography.Text>
      </Space>
      {job.errors?.length ? (
        <Table<ImportError>
          rowKey={(r) => `${r.row}:${r.field ?? ''}:${r.code}`}
          size="small"
          pagination={false}
          dataSource={job.errors}
          columns={[
            { key: 'row', title: t('platform.import.row'), dataIndex: 'row' },
            { key: 'field', title: t('platform.import.field'), dataIndex: 'field' },
            { key: 'code', title: t('platform.import.code'), dataIndex: 'code' },
            { key: 'message', title: t('platform.import.message'), dataIndex: 'message' },
          ]}
        />
      ) : null}
    </section>
  );
}

export function SchoolImportWizard() {
  const { t } = useTranslation();
  const { modal } = AntApp.useApp();
  const [form] = Form.useForm<{ tenant_target: string }>();
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState(false);
  const [dryRunId, setDryRunId] = useState<string>();
  const [applyId, setApplyId] = useState<string>();
  const start = useImportSchools();
  const dryRun = useSchoolImport(dryRunId);
  const applied = useSchoolImport(applyId);

  const reset = () => {
    setDryRunId(undefined);
    setApplyId(undefined);
    start.reset();
  };

  const runDry = async () => {
    const v = await form.validateFields().catch(() => null);
    setFileError(!file);
    if (!v || !file) return;
    reset();
    start.mutate({ tenantId: v.tenant_target, dryRun: true, file, fileKey: fileKeyOf(file) }, { onSuccess: (job) => setDryRunId(job.import_id) });
  };

  const apply = () => {
    const tenantId = form.getFieldValue('tenant_target') as string;
    const job = dryRun.data;
    if (!file || !job || !tenantId) return;
    modal.confirm({
      title: t('platform.import.applyTitle'),
      content: t('platform.import.applyContent', { total: job.total_rows ?? 0, errors: job.error_rows ?? 0 }),
      okText: t('platform.common.confirm'),
      cancelText: t('platform.common.cancel'),
      onOk: () => start.mutate({ tenantId, dryRun: false, file, fileKey: fileKeyOf(file) }, { onSuccess: (j) => setApplyId(j.import_id) }),
    });
  };

  const dryDone = dryRun.data?.status === 'succeeded';
  return (
    <section aria-label={t('platform.catalog.import')}>
      <Form form={form} name="school_import" layout="vertical" onValuesChange={reset}>
        <TenantTargetField />
        <Form.Item label={t('platform.import.file')} required validateStatus={fileError ? 'error' : undefined} help={fileError ? t('platform.import.fileRequired') : undefined}>
          <Upload
            accept=".csv,text/csv"
            maxCount={1}
            beforeUpload={(f) => {
              setFile(f);
              setFileError(false);
              reset();
              return false;
            }}
            onRemove={() => {
              setFile(null);
              reset();
            }}
          >
            <Button>{t('platform.import.pickFile')}</Button>
          </Upload>
        </Form.Item>
        <Space>
          <Button onClick={runDry} loading={start.isPending && !dryRunId}>
            {t('platform.import.dryRun')}
          </Button>
          <Button type="primary" disabled={!dryDone || !!applyId} onClick={apply}>
            {t('platform.import.apply')}
          </Button>
        </Space>
      </Form>
      {start.error ? <ProblemMessage error={start.error} /> : null}
      {dryRun.error ? <ProblemMessage error={dryRun.error} /> : null}
      {dryRun.data ? <ImportResult job={dryRun.data} title={t('platform.import.dryRunResult')} /> : null}
      {applied.error ? <ProblemMessage error={applied.error} /> : null}
      {applied.data ? <ImportResult job={applied.data} title={t('platform.import.applyResult')} /> : null}
      {dryRun.data && !isFinalJob(dryRun.data.status) ? <Alert type="info" message={t(`platform.import.status.${dryRun.data.status}`)} /> : null}
    </section>
  );
}
