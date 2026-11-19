// TranslationExportDialog / TranslationImportDialog (F053-AC-25, ADMIN_PORTAL §1.5, ADR-0056): export the fields of a
// content item to translate (XLIFF / CSV, only missing by default) and download the file when the job succeeds;
// import a translated file (uploaded as a `document` media, license = the translator's) → a new Draft is created and
// the keys that do not match are listed. Buttons are gated by `action.content_studio.translations.translate`.
import { Alert, Button, Checkbox, Flex, Form, Input, List, Modal, Select, Typography } from 'antd';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { sha256OfFile, useMediaUpload } from '../media/useMediaUpload';
import { LIMITS } from '../shared/config';
import { useTranslationActions, useTranslationJob, type TranslationJob } from './useTranslationJobs';

type Lang = 'vi' | 'en';
const other = (l: Lang): Lang => (l === 'vi' ? 'en' : 'vi');

export function JobStatus({ job }: { job?: TranslationJob }) {
  const { t } = useTranslation();
  if (!job) return null;
  return (
    <Flex vertical gap="small" data-testid="translation-job" data-status={job.status}>
      <Typography.Text>{t(`content_studio.translations.status.${job.status}`)}</Typography.Text>
      {job.status === 'succeeded' && job.download_url && (
        <Button type="link" href={job.download_url} target="_blank" rel="noopener noreferrer">
          {t('content_studio.translations.download')}
        </Button>
      )}
      {job.report && (
        <Typography.Text type="secondary">{t('content_studio.translations.report', { total: job.report.total ?? 0, updated: job.report.updated ?? 0, skipped: job.report.skipped ?? 0 })}</Typography.Text>
      )}
      {(job.report?.errors?.length ?? 0) > 0 && (
        <List
          size="small"
          header={t('content_studio.translations.mismatch')}
          dataSource={job.report!.errors!}
          renderItem={(e) => (
            <List.Item>
              <Typography.Text code>{e.ref}</Typography.Text> {e.message}
            </List.Item>
          )}
        />
      )}
    </Flex>
  );
}

export function TranslationExportDialog({ open, onClose, contentType, contentIds, sourceLanguage }: { open: boolean; onClose: () => void; contentType: string; contentIds: string[]; sourceLanguage: Lang }) {
  const { t } = useTranslation();
  const { exportJob } = useTranslationActions();
  const [format, setFormat] = useState<'xliff' | 'csv'>('xliff');
  const [onlyMissing, setOnlyMissing] = useState(true);
  const [jobId, setJobId] = useState<string>();
  const job = useTranslationJob(jobId);
  return (
    <Modal open={open} title={t('content_studio.translations.exportTitle')} footer={null} onCancel={onClose} destroyOnHidden>
      <Flex vertical gap="small">
        <Select aria-label={t('content_studio.translations.format')} value={format} onChange={setFormat} options={[{ value: 'xliff', label: 'XLIFF 2.1' }, { value: 'csv', label: 'CSV' }]} />
        <Checkbox checked={onlyMissing} onChange={(e) => setOnlyMissing(e.target.checked)}>
          {t('content_studio.translations.onlyMissing')}
        </Checkbox>
        <ActionGate actionKey="action.content_studio.translations.translate" mode="disable">
          <Button
            type="primary"
            loading={exportJob.isPending}
            onClick={() =>
              exportJob.mutate(
                { content_types: [contentType], content_ids: contentIds, source_language: sourceLanguage, target_language: other(sourceLanguage), format, only_missing: onlyMissing },
                { onSuccess: (j) => setJobId(j.job_id) },
              )
            }
          >
            {t('content_studio.translations.exportOk')}
          </Button>
        </ActionGate>
        {exportJob.error ? <ProblemMessage error={exportJob.error} /> : null}
        {job.error ? <ProblemMessage error={job.error} /> : null}
        <JobStatus job={job.data ?? exportJob.data} />
      </Flex>
    </Modal>
  );
}

export function TranslationImportDialog({ open, onClose, onImported, sourceLanguage }: { open: boolean; onClose: () => void; onImported: () => void; sourceLanguage: Lang }) {
  const { t } = useTranslation();
  const { importJob } = useTranslationActions();
  const upload = useMediaUpload();
  const [file, setFile] = useState<File | null>(null);
  const [license, setLicense] = useState('');
  const [format, setFormat] = useState<'xliff' | 'csv'>('xliff');
  const [jobId, setJobId] = useState<string>();
  const job = useTranslationJob(jobId);
  const done = job.data?.status === 'succeeded';
  useEffect(() => {
    if (done) onImported();
    // once per finished job
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [done]);
  const start = async () => {
    if (!file || !license.trim()) return;
    const assetId = await upload.mutateAsync({ file, kind: 'document', license: license.trim(), sha256: await sha256OfFile(file) });
    const j = await importJob.mutateAsync({ asset_id: assetId, format, target_language: other(sourceLanguage), dry_run: false });
    setJobId(j.job_id);
  };
  return (
    <Modal open={open} title={t('content_studio.translations.importTitle')} footer={null} onCancel={onClose} destroyOnHidden>
      <Flex vertical gap="small">
        <Form layout="vertical" component="div">
          <Form.Item label={t('content_studio.translations.file')} required>
            <input type="file" aria-label={t('content_studio.translations.file')} accept=".xlf,.xliff,.csv" onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
          </Form.Item>
          <Form.Item label={t('content_studio.media.license')} required>
            <Input aria-label={t('content_studio.media.license')} value={license} maxLength={LIMITS.license} onChange={(e) => setLicense(e.target.value)} />
          </Form.Item>
          <Select aria-label={t('content_studio.translations.format')} value={format} onChange={setFormat} options={[{ value: 'xliff', label: 'XLIFF 2.1' }, { value: 'csv', label: 'CSV' }]} />
        </Form>
        <ActionGate actionKey="action.content_studio.translations.translate" mode="disable">
          <Button type="primary" disabled={!file || !license.trim()} loading={upload.isPending || importJob.isPending} onClick={() => void start().catch(() => undefined)}>
            {t('content_studio.translations.importOk')}
          </Button>
        </ActionGate>
        {upload.error ? <ProblemMessage error={upload.error} /> : null}
        {importJob.error ? <ProblemMessage error={importJob.error} /> : null}
        {job.data?.status === 'succeeded' && <Alert type="success" showIcon message={t('content_studio.translations.newDraft')} />}
        <JobStatus job={job.data ?? importJob.data} />
      </Flex>
    </Modal>
  );
}
