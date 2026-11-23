// Banner "Đang tắt kiểm duyệt: <bước> — đến <thời hạn>" in the BannerSlot of every screen (F122 T028; F122-AC-13, AC-14;
// ADR-0030 requires it while moderation is off): error colour of the theme, the steps and the time they come back on —
// never the reason. Tenant users only ever receive their own tenant's state (getTenantModerationStatus); platform staff
// see the platform row and the tenants of the topbar filter; without a filter, the number of tenants off with a link to
// the switches (disabled_tenant_count, F122 T065 — never a list of tenants). Display only.
import { Alert } from 'antd';
import dayjs from 'dayjs';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { menuByKey } from '../../permissions/menuRegistry';
import { useModerationBannerState, type DisabledLevel } from '../../shared/chatSafety/useModerationStatus';

function describe(l: DisabledLevel, t: (k: string, o?: Record<string, unknown>) => string): string {
  const steps = l.allOff ? t('chatSafety.switches.all') : l.steps.map((s) => t(`chatSafety.step.${s}`)).join(', ');
  const until = l.expiresAt ? t('chatSafety.banner.until', { at: dayjs(l.expiresAt).format('DD/MM/YYYY HH:mm') }) : '';
  const who = l.tenantId === null ? t('chatSafety.switches.platform') : l.tenantName;
  return [who ? `${who}: ` : '', steps, until].join('');
}

/** The banner element, or null when every step runs (so the slot stays empty). */
export function useModerationDisabledBanner(): ReactNode | null {
  const { t } = useTranslation();
  const { levels, disabledTenantCount } = useModerationBannerState();
  // path from the generated catalog (ADR-0090: no menu path by hand)
  const switchesPath = menuByKey('menu.platform_admin.moderation_switches')?.path;
  if (!levels.length && !disabledTenantCount) return null;
  const parts = levels.map((l) => describe(l, t));
  if (disabledTenantCount) parts.push(t('chatSafety.banner.tenantCount', { count: disabledTenantCount }));
  return (
    <Alert
      type="error"
      banner
      showIcon
      role="alert"
      data-testid="moderation-disabled-banner"
      message={t('chatSafety.banner.title', { detail: parts.join(' · ') })}
      action={disabledTenantCount && switchesPath ? <Link to={switchesPath}>{t('chatSafety.banner.openSwitches')}</Link> : undefined}
    />
  );
}
