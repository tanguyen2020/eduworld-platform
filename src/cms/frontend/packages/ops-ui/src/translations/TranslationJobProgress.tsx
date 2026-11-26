// TranslationJobProgress (F082-AC-12, AC-14, AC-15): one export/import job of the game translations — status, the signed
// download link of an export with its expiry (report_link_ttl, shown in ICT by the caller's `formatTime`), the report of
// an import (total / updated / skipped, every refused row with its `code`), the message of a failed job. Presentational.
import { Alert, Descriptions, Flex, Spin, Table, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';

export interface TranslationReportError {
  ref?: string;
  code?: string;
  message?: string;
}

export interface TranslationJobView {
  job_id: string;
  kind: 'export' | 'import';
  status: 'queued' | 'running' | 'succeeded' | 'failed';
  dry_run?: boolean;
  download_url?: string;
  expires_at?: string;
  error_message?: string;
  report?: { total?: number; updated?: number; skipped?: number; errors?: TranslationReportError[] };
}

export interface TranslationJobProgressProps {
  job: TranslationJobView;
  /** ISO instant → display time (ICT). */
  formatTime: (iso: string) => string;
}

const STATUS_COLOR = { queued: 'default', running: 'processing', succeeded: 'success', failed: 'error' } as const;

export function TranslationJobProgress({ job, formatTime }: TranslationJobProgressProps) {
  const { t, i18n } = useTranslation(OPS_UI_NS);
  const pending = job.status === 'queued' || job.status === 'running';
  const errors = job.report?.errors ?? [];
  const codeLabel = (code?: string) => (code && i18n.exists(`${OPS_UI_NS}:translations.errorCode.${code}`) ? t(`translations.errorCode.${code}`) : (code ?? ''));
  return (
    <Flex vertical gap="small" data-testid="translation-job" data-status={job.status}>
      <Flex gap="small" align="center" wrap>
        <Typography.Text strong>{t(`translations.job.kind.${job.kind}`)}</Typography.Text>
        <Tag color={STATUS_COLOR[job.status]}>{t(`translations.job.status.${job.status}`)}</Tag>
        {pending && <Spin size="small" aria-label={t('translations.job.waiting')} />}
        {job.dry_run && <Tag>{t('translations.job.dryRun')}</Tag>}
      </Flex>
      {job.status === 'failed' && <Alert type="error" showIcon message={job.error_message ?? t('translations.job.failed')} />}
      {job.kind === 'export' && job.status === 'succeeded' && job.download_url && (
        <Flex vertical gap="small">
          <Typography.Link href={job.download_url} target="_blank" rel="noopener noreferrer">
            {t('translations.job.download')}
          </Typography.Link>
          {job.expires_at && <Typography.Text type="secondary">{t('translations.job.expiresAt', { at: formatTime(job.expires_at) })}</Typography.Text>}
        </Flex>
      )}
      {job.report && (
        <Descriptions size="small" column={1}>
          <Descriptions.Item label={t('translations.job.report')}>
            {t('translations.job.counts', { total: job.report.total ?? 0, updated: job.report.updated ?? 0, skipped: job.report.skipped ?? 0 })}
          </Descriptions.Item>
        </Descriptions>
      )}
      {errors.length > 0 && (
        <Table<TranslationReportError>
          size="small"
          rowKey={(r) => `${r.ref ?? ''}:${r.code ?? ''}:${r.message ?? ''}`}
          pagination={false}
          dataSource={errors}
          scroll={{ x: true }}
          columns={[
            { key: 'ref', title: t('translations.job.ref'), render: (_, r) => <Typography.Text code>{r.ref}</Typography.Text> },
            { key: 'code', title: t('translations.job.code'), render: (_, r) => codeLabel(r.code) },
            { key: 'message', title: t('translations.job.message'), render: (_, r) => r.message },
          ]}
        />
      )}
    </Flex>
  );
}
