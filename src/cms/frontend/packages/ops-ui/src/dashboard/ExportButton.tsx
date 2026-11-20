// ExportButton (F056 T036, F056-AC-14, F056-AC-15, DASHBOARD_DATA §10): "Xuất" starts an asynchronous job (the caller
// creates it with an Idempotency-Key and polls it); while it runs: "Đang chuẩn bị báo cáo"; when it succeeded: the signed
// link with its expiry time (`download_expires_at`, `dashboard_export_link_ttl_minutes`); after expiry or on `expired`
// the link disappears and the button offers to create the report again. The file is never downloaded from the export
// request itself. Presentational: no network call here.
import { Alert, Button, Flex, Spin, Typography } from 'antd';
import dayjs from 'dayjs';
import { useEffect, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';

export type ExportJobStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'expired';

/** The fields of `ReportJob` the button needs. */
export interface ExportJobView {
  status: ExportJobStatus;
  download_url?: string | null;
  download_expires_at?: string;
  /** Failure code without PII (export_failed, too_many_rows, storage_unavailable, replica_unavailable, query_timeout). */
  error?: string;
}

export const EXPORT_FAILURE_CODES = ['export_failed', 'too_many_rows', 'storage_unavailable', 'replica_unavailable', 'query_timeout'] as const;

export interface ExportButtonProps {
  onExport: () => void;
  label?: ReactNode;
  /** The create request is in flight. */
  pending?: boolean;
  job?: ExportJobView | null;
  /** Translated message of a refused request (e.g. EXPORT_LIMIT_REACHED, FEATURE_DISABLED). */
  errorMessage?: ReactNode;
  disabled?: boolean;
  /** Clock (tests). */
  now?: () => number;
}

/** True when a succeeded job's link can still be used. */
export function linkUsable(job: ExportJobView | null | undefined, nowMs: number): boolean {
  if (!job || job.status !== 'succeeded' || !job.download_url) return false;
  return !job.download_expires_at || dayjs(job.download_expires_at).valueOf() > nowMs;
}

export function ExportButton({ onExport, label, pending, job, errorMessage, disabled, now = Date.now }: ExportButtonProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const [, setTick] = useState(0);
  const running = job?.status === 'queued' || job?.status === 'running';
  const usable = linkUsable(job, now());
  const expired = !!job && (job.status === 'expired' || (job.status === 'succeeded' && !usable));
  // re-render when the link expires so it disappears without user action
  useEffect(() => {
    if (!usable || !job?.download_expires_at) return;
    const ms = dayjs(job.download_expires_at).valueOf() - now();
    const id = setTimeout(() => setTick((n) => n + 1), Math.max(0, ms) + 50);
    return () => clearTimeout(id);
  }, [usable, job?.download_expires_at, now]);
  const failure = job?.status === 'failed' ? ((EXPORT_FAILURE_CODES as readonly string[]).includes(job.error ?? '') ? `dashboard.export.failed.${job.error}` : 'dashboard.export.failed.export_failed') : null;
  return (
    <Flex vertical gap="small" data-testid="export-button">
      <Flex wrap gap="small" align="center">
        <Button onClick={onExport} loading={pending} disabled={disabled || running}>
          {expired ? t('dashboard.export.recreate') : (label ?? t('dashboard.export.button'))}
        </Button>
        <span role="status" aria-live="polite">
          {running ? (
            <Flex gap="small" align="center">
              <Spin size="small" />
              <Typography.Text>{t('dashboard.export.preparing')}</Typography.Text>
            </Flex>
          ) : null}
          {usable && job?.download_url ? (
            <Flex gap="small" align="center" wrap>
              <Typography.Link href={job.download_url} rel="noopener noreferrer" data-testid="export-link">
                {t('dashboard.export.download')}
              </Typography.Link>
              {job.download_expires_at ? <Typography.Text type="secondary">{t('dashboard.export.expiresAt', { time: dayjs(job.download_expires_at).format('HH:mm') })}</Typography.Text> : null}
            </Flex>
          ) : null}
          {expired ? <Typography.Text type="secondary">{t('dashboard.export.expired')}</Typography.Text> : null}
        </span>
      </Flex>
      {failure ? <Alert type="error" showIcon message={t(failure)} /> : null}
      {errorMessage ? <Alert type="warning" showIcon role="alert" message={errorMessage} /> : null}
    </Flex>
  );
}
