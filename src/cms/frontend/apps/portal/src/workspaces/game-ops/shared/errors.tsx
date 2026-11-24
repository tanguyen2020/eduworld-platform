// Errors of workspace A (F079-AC-19, AC-20, AC-21, ADMIN_PORTAL §1.2, §10.1): `403 PERMISSION_DENIED` → the 403 page
// with the `trace_id` (no MFA step-up — ADR-0094); internal gateway not reachable (`VPN_REQUIRED`) → "Cần kết nối mạng
// quản trị (VPN)"; `403 FEATURE_DISABLED` → "tính năng tạm tắt" (the shell also reloads getMyPermissions); anything
// else → the translated Problem by `code` + `trace_id`. `422` field errors (`errors[].field`) → a map by field path.
import { Result, Typography } from 'antd';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { VpnRequiredNotice } from '../../../shell/StatusPages';

export function GameOpsError({ error }: { error: unknown }) {
  const { t } = useTranslation();
  if (error instanceof ApiError) {
    const trace = error.traceId ? (
      <Typography.Text type="secondary" copyable={{ text: error.traceId }}>
        {t('shell.problem.traceId', { traceId: error.traceId })}
      </Typography.Text>
    ) : null;
    if (error.code === 'VPN_REQUIRED') return <VpnRequiredNotice />;
    if (error.status === 403 && error.code === 'FEATURE_DISABLED') {
      return (
        <div data-testid="game-ops-feature-disabled">
          <Result status="warning" title={t('shell.featureDisabled.title')} subTitle={t('shell.featureDisabled.description')} extra={trace} />
        </div>
      );
    }
    if (error.status === 403) {
      return (
        <div data-testid="game-ops-forbidden">
          <Result status="403" title={t('shell.forbidden.title')} subTitle={t(error.i18nKey)} extra={trace} />
        </div>
      );
    }
  }
  return <ProblemMessage error={error} />;
}

/** `422` field errors (VALIDATION_FAILED, TRANSLATION_INCOMPLETE…) translated by field code, keyed by server path. */
export function fieldErrorsOf(error: unknown, t: TFunction, exists: (k: string) => boolean): Record<string, string> {
  if (!(error instanceof ApiError) || error.status !== 422) return {};
  const out: Record<string, string> = {};
  for (const [field, fe] of Object.entries(error.fieldErrors)) {
    const own = `error.field.${fe.code}`;
    out[field] = exists(own) ? t(own) : t('gameOps.editor.fieldInvalid', { code: fe.code });
  }
  return out;
}

/** True for an error that carries field errors to show on the form. */
export const hasFieldErrors = (e: unknown) => e instanceof ApiError && e.status === 422 && Object.keys(e.fieldErrors).length > 0;
