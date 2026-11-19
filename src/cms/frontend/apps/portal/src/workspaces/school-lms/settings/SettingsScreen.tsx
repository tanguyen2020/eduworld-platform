// Screen of `menu.school_lms.settings` (F052 US4): tabs "Cài đặt tenant" (tenant.settings.read), "Cài đặt trường"
// (school.settings.read), "Học sinh sắp bị dọn" (action cleanup_view) — each tab only with its permission/action.
import { Tabs } from 'antd';
import { useTranslation } from 'react-i18next';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { CleanupPreviewPage } from '../student-cleanup/CleanupPreviewPage';
import { SchoolSettingsPage } from './SchoolSettingsPage';
import { TenantSettingsPage } from './TenantSettingsPage';

function SettingsHome() {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const has = (p: string) => !!view?.permissions.includes(p);
  const viewOn = !!view && actionStatus('action.school_lms.settings.view', view) === 'enabled';
  const items = [
    viewOn && has('tenant.settings.read') ? { key: 'tenant', label: t('portal.school_lms.settings.tenantTab'), children: <TenantSettingsPage /> } : null,
    viewOn && has('school.settings.read') ? { key: 'school', label: t('portal.school_lms.settings.schoolTab'), children: <SchoolSettingsPage /> } : null,
    view && actionStatus('action.school_lms.settings.cleanup_view', view) === 'enabled' ? { key: 'cleanup', label: t('portal.school_lms.cleanup.tab'), children: <CleanupPreviewPage /> } : null,
  ].filter((x): x is NonNullable<typeof x> => x !== null);
  return <Tabs destroyOnHidden items={items} />;
}

export function SettingsScreen({ title }: { title: string }) {
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>{() => <SettingsHome />}</SingleTenantGate>
    </section>
  );
}
