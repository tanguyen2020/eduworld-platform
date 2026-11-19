// ProblemMessage (F051 T009, F051-AC-15, ADMIN_PORTAL §10.1): message of an API error translated from its Problem
// `code` (apiClient maps `code` → `error.<CODE>`, unknown code → `error.generic`) plus the `trace_id` for support.
// Never shows the server `title`/`detail` (may be untranslated) nor any token.
import { Alert, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { ApiError, GENERIC_KEY } from '../shared/api/middleware/problem';

/** i18n key + trace id of any thrown value (non-ApiError → generic). */
export function problemOf(error: unknown): { key: string; code?: string; traceId?: string } {
  if (error instanceof ApiError) return { key: error.i18nKey, code: error.code, traceId: error.traceId };
  return { key: GENERIC_KEY };
}

export function ProblemMessage({ error, banner = false }: { error: unknown; banner?: boolean }) {
  const { t } = useTranslation();
  const p = problemOf(error);
  const description = p.traceId ? (
    <Typography.Text type="secondary" copyable={{ text: p.traceId }}>
      {t('shell.problem.traceId', { traceId: p.traceId })}
    </Typography.Text>
  ) : undefined;
  return <Alert type="error" showIcon banner={banner} message={t(p.key)} description={description} role="alert" />;
}
