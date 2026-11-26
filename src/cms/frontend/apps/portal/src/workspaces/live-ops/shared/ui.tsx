// Small shared pieces of the F080 screens of workspace E: page title (h1), action status from the catalog (ADR-0090),
// localized names, the API error of a whole screen (reuses GameOpsError of workspace A: 403 → the 403 page with the
// trace_id, internal gateway down → "Cần kết nối mạng quản trị (VPN)", 403 FEATURE_DISABLED → "Tính năng đang tạm tắt"
// while the shell reloads getMyPermissions — F080-AC-25, AC-33) and the error of one dashboard widget. No data logic.
import type { DashboardWidgetError } from '@eduworld/ops-ui/dashboard';
import { Flex, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import type { ActionKey, MenuKey } from '../../../generated/menu-catalog';
import { actionStatus, menuStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { ApiError } from '../../../shared/api/middleware/problem';
import { GameOpsError } from '../../game-ops/shared/errors';

export function PageTitle({ title, extra }: { title: string; extra?: ReactNode }) {
  return (
    <Flex justify="space-between" align="center" wrap="wrap" gap="small">
      <Typography.Title level={1} id="ew-menu-title">
        {title}
      </Typography.Title>
      {extra}
    </Flex>
  );
}

export function useActionEnabled(key: ActionKey): boolean {
  const { view } = usePermissions();
  return !!view && actionStatus(key, view) === 'enabled';
}

export function useMenuVisible(key: MenuKey): boolean {
  const { view } = usePermissions();
  return !!view && menuStatus(key, view) === 'visible';
}

/** vi/en name (TenantLocalizedName / LocalizedName): the UI language, else vi. */
export function nameIn(name: { vi?: string; en?: string } | undefined | null, language: string): string {
  if (!name) return '';
  return (language.startsWith('en') ? name.en : undefined) ?? name.vi ?? name.en ?? '';
}

/** Error of a whole screen or section (403 page with trace id, VPN notice, feature disabled, translated Problem). */
export function LiveOpsError({ error }: { error: unknown }) {
  return <GameOpsError error={error} />;
}

/** Error of one dashboard widget: ops-ui explains DASHBOARD_* itself; other codes get their translation + trace id. */
export function useWidgetError(error: unknown): DashboardWidgetError | null {
  const { t } = useTranslation();
  if (!error) return null;
  if (!(error instanceof ApiError)) return { code: 'UNKNOWN', message: t('error.generic') };
  return {
    code: error.code,
    message: (
      <>
        {t(error.i18nKey)}
        {error.traceId ? (
          <Typography.Text type="secondary" copyable={{ text: error.traceId }}>
            {' '}
            {t('shell.problem.traceId', { traceId: error.traceId })}
          </Typography.Text>
        ) : null}
      </>
    ),
  };
}
