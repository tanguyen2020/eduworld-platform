// TranslationImportDialog (F082-AC-13, AC-15, ADMIN_PORTAL §1.5): import a translated XLIFF 2.1 / CSV file — pick the file
// (the caller uploads it and follows the server check), run a dry run (report only, nothing written), read the report,
// then confirm the real import (a new intent with its own Idempotency-Key, kept on retry). Presentational: the caller
// owns the operations and the state; no machine translation.
import { Alert, Button, Flex, Form, Modal, Radio, Spin, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';
import { TranslationJobProgress, type TranslationJobView } from './TranslationJobProgress';

export type ImportUploadState = 'idle' | 'uploading' | 'ready' | 'error';

export interface TranslationImportDialogProps {
  open: boolean;
  target: 'vi' | 'en';
  onTargetChange: (lang: 'vi' | 'en') => void;
  onFile: (file: File) => void;
  fileName?: string;
  uploadState: ImportUploadState;
  /** Why the upload / server check failed (rendered by the caller: code + trace id). */
  uploadError?: ReactNode;
  dryRunJob?: TranslationJobView;
  importJob?: TranslationJobView;
  busy?: boolean;
  /** Problem of the last import request. */
  error?: ReactNode;
  onDryRun: () => void;
  onConfirm: () => void;
  onClose: () => void;
  formatTime: (iso: string) => string;
}

export function TranslationImportDialog(p: TranslationImportDialogProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const dryDone = p.dryRunJob?.status === 'succeeded';
  const dryPending = p.dryRunJob && (p.dryRunJob.status === 'queued' || p.dryRunJob.status === 'running');
  const importStarted = !!p.importJob;
  return (
    <Modal
      open={p.open}
      title={t('translations.import.title')}
      onCancel={p.onClose}
      destroyOnHidden
      footer={
        <Flex gap="small" justify="flex-end">
          <Button onClick={p.onClose}>{importStarted ? t('translations.close') : t('translations.cancel')}</Button>
          <Button disabled={p.uploadState !== 'ready' || !!dryPending || importStarted || p.busy} onClick={p.onDryRun}>
            {t('translations.import.dryRun')}
          </Button>
          <Button type="primary" disabled={!dryDone || importStarted || p.busy} loading={p.busy && dryDone} onClick={p.onConfirm}>
            {t('translations.import.confirm')}
          </Button>
        </Flex>
      }
    >
      <Flex vertical gap="middle">
        <Alert type="info" showIcon message={t('translations.import.hint')} />
        <Form layout="vertical" component="div">
          <Form.Item label={t('translations.import.file')} htmlFor="tr-import-file">
            <input id="tr-import-file" type="file" accept=".xlf,.xliff,.csv" disabled={importStarted} onChange={(e) => e.target.files?.[0] && p.onFile(e.target.files[0])} />
          </Form.Item>
          <Form.Item label={t('translations.target')}>
            <Radio.Group
              optionType="button"
              value={p.target}
              disabled={importStarted}
              onChange={(e) => p.onTargetChange(e.target.value as 'vi' | 'en')}
              options={[
                { value: 'en', label: t('translations.lang.en') },
                { value: 'vi', label: t('translations.lang.vi') },
              ]}
            />
          </Form.Item>
        </Form>
        {p.fileName && (
          <Flex gap="small" align="center">
            <Typography.Text>{p.fileName}</Typography.Text>
            {p.uploadState === 'uploading' && <Spin size="small" aria-label={t('translations.import.uploading')} />}
            <Typography.Text type={p.uploadState === 'error' ? 'danger' : 'secondary'}>{t(`translations.import.upload.${p.uploadState}`)}</Typography.Text>
          </Flex>
        )}
        {p.uploadError}
        {p.dryRunJob && (
          <section aria-label={t('translations.import.dryRunReport')}>
            <TranslationJobProgress job={p.dryRunJob} formatTime={p.formatTime} />
          </section>
        )}
        {p.importJob && (
          <section aria-label={t('translations.import.importReport')}>
            <TranslationJobProgress job={p.importJob} formatTime={p.formatTime} />
          </section>
        )}
        {p.error}
      </Flex>
    </Modal>
  );
}
