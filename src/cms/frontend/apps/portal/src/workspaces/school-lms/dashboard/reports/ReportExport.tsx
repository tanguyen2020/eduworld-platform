// Export control of workspace C (F056 T036/T037; F056-AC-14, AC-15, AC-16, AC-23): the ops-ui ExportButton driven by
// useReportExport (Idempotency-Key per intent, job polling, link + expiry). Refusals by `code`: 429
// EXPORT_LIMIT_REACHED (dashboard_export_max_concurrent_per_user) → wait for a running export; 403 FEATURE_DISABLED →
// "tính năng tạm tắt" (the shell reloads getMyPermissions). Hidden when the catalog action is not granted or turned off.
import { ExportButton } from '@eduworld/ops-ui/dashboard';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../../permissions/ActionGate';
import { ApiError } from '../../../../shared/api/middleware/problem';
import { useReportExport, type LmsReportRequest, type ReportRequest } from '../api/reportsApi';

export function useExportErrorMessage(error: unknown): ReactNode {
  const { t } = useTranslation();
  if (!error) return null;
  if (!(error instanceof ApiError)) return t('error.generic');
  if (error.code === 'EXPORT_LIMIT_REACHED' || (error.status === 429 && error.code !== 'DASHBOARD_BUSY')) return t('dashboard.export.limit');
  if (error.code === 'FEATURE_DISABLED') return t('dashboard.featureDisabled');
  if (error.status === 403) return t('dashboard.outOfScope');
  return t(error.i18nKey);
}

export function LmsReportExport({ request, label, disabled }: { request: LmsReportRequest | null; label?: ReactNode; disabled?: boolean }) {
  const exp = useReportExport<LmsReportRequest>('lms');
  const message = useExportErrorMessage(exp.error);
  return (
    <ActionGate actionKey="action.school_lms.reports.export">
      <ExportButton label={label} onExport={() => request && exp.create(request)} pending={exp.pending} job={exp.job} errorMessage={message} disabled={disabled || !request} />
    </ActionGate>
  );
}

export function TenantReportExport({ request, label }: { request: ReportRequest; label?: ReactNode }) {
  const exp = useReportExport<ReportRequest>('tenant');
  const message = useExportErrorMessage(exp.error);
  return (
    <ActionGate actionKey="action.school_lms.reports.export">
      <ExportButton label={label} onExport={() => exp.create(request)} pending={exp.pending} job={exp.job} errorMessage={message} />
    </ActionGate>
  );
}
