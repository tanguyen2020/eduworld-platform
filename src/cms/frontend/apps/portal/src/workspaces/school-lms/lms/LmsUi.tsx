// Presentational helpers of the F055 screens: translated Problem (own LMS message by `code` + trace id, never the raw
// `detail` — F055-AC-06), permission / catalog-action checks for conditional UI (display only: the backend authorizes
// every call, ADR-0027 mục 3), status tags and date formatting. No data logic (AGENTS R1).
import { Alert, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import type { ActionKey } from '../../../generated/menu-catalog';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { ApiError } from '../../../shared/api/middleware/problem';
import { lmsErrorKey } from './model/problemMessages';

export function LmsProblem({ error, testId = 'lms-problem' }: { error: unknown; testId?: string }) {
  const { t } = useTranslation();
  if (!error) return null;
  const traceId = error instanceof ApiError ? error.traceId : undefined;
  return (
    <Alert
      type="error"
      showIcon
      role="alert"
      data-testid={testId}
      message={t(lmsErrorKey(error))}
      description={
        traceId ? (
          <Typography.Text type="secondary" copyable={{ text: traceId }}>
            {t('shell.problem.traceId', { traceId })}
          </Typography.Text>
        ) : undefined
      }
    />
  );
}

/** True when the session holds the permission (conditional rendering only). */
export function useHasPermission(code: string): boolean {
  const { view } = usePermissions();
  return !!view?.permissions.includes(code);
}

/** True when a catalog action is granted and not turned off (ADR-0090). */
export function useActionEnabled(key: ActionKey): boolean {
  const { view } = usePermissions();
  return !!view && actionStatus(key, view) === 'enabled';
}

export function LmsTag({ group, value }: { group: string; value: string | undefined }) {
  const { t } = useTranslation();
  if (!value) return null;
  return <Tag data-testid={`tag-${group}`}>{t(`lms.${group}.${value}`)}</Tag>;
}
