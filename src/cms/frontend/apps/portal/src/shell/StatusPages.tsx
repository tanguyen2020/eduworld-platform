// Shell status pages (F051 T021, T032, T034, T036, T061): 403 of the portal (AC-11), "Tính năng đang tắt" (AC-35,
// distinct from 403), no admin access / student account (AC-03, AC-04), VPN required (AC-14), large screen required
// (ADMIN_PORTAL §1.5), and the placeholder of a menu whose screen arrives with a later feature. Strings from i18n.
import { Alert, Button, Empty, Result, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';

export function ForbiddenPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  return (
    <Result
      status="403"
      title={t('shell.forbidden.title')}
      subTitle={t('shell.forbidden.description')}
      extra={
        <Button type="primary" onClick={() => navigate('/')}>
          {t('shell.forbidden.home')}
        </Button>
      }
    />
  );
}

export function FeatureDisabledPage({ featureKey }: { featureKey?: string }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  return (
    <div data-testid="feature-disabled">
      <Result
      status="warning"
      title={t('shell.featureDisabled.title')}
      subTitle={[t('shell.featureDisabled.description'), featureKey ? t('shell.featureDisabled.featureKey', { key: featureKey }) : ''].filter(Boolean).join(' ')}
      extra={<Button onClick={() => navigate('/')}>{t('shell.forbidden.home')}</Button>}
      />
    </div>
  );
}

export interface NoAccessPageProps {
  reason: 'no_role' | 'student';
  onLogout: () => void;
}

export function NoAccessPage({ reason, onLogout }: NoAccessPageProps) {
  const { t } = useTranslation();
  const student = reason === 'student';
  return (
    <Result
      status={student ? 'info' : 'warning'}
      title={t(student ? 'shell.noAccess.studentTitle' : 'shell.noAccess.title')}
      subTitle={t(student ? 'shell.noAccess.studentDescription' : 'shell.noAccess.description')}
      extra={<Button onClick={onLogout}>{t('shell.session.logout')}</Button>}
    />
  );
}

export function VpnRequiredNotice({ onClose }: { onClose?: () => void }) {
  const { t } = useTranslation();
  return <Alert type="warning" showIcon message={t('shell.vpn.title')} description={t('shell.vpn.description')} closable={!!onClose} onClose={onClose} role="alert" />;
}

export function LargeScreenRequired({ children }: { children?: ReactNode }) {
  const { t } = useTranslation();
  return (
    <Result status="info" title={t('shell.largeScreen.title')} subTitle={t('shell.largeScreen.description')}>
      {children}
    </Result>
  );
}

export function MenuPlaceholder({ title }: { title: string }) {
  const { t } = useTranslation();
  return (
    <section aria-labelledby="ew-menu-title">
      <Typography.Title level={1} id="ew-menu-title">
        {title}
      </Typography.Title>
      <Empty description={t('shell.menuPlaceholder')} />
    </section>
  );
}
