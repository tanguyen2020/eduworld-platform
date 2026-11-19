// Menu "Danh mục quốc gia, tỉnh/thành, trường" (F057 US5, ADMIN_PORTAL §3.16): tabs countries, provinces, school
// catalog and the school CSV import (behind `action.platform_admin.catalog.import_schools`, hidden when turned off).
import { Tabs } from 'antd';
import { useTranslation } from 'react-i18next';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { PageTitle } from '../ui';
import { CountriesPage } from './CountriesPage';
import { ProvincesPage } from './ProvincesPage';
import { SchoolCatalogPage } from './SchoolCatalogPage';
import { SchoolImportWizard } from './SchoolImportWizard';

export function CatalogPage({ title }: { title: string }) {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const canImport = !!view && actionStatus('action.platform_admin.catalog.import_schools', view) === 'enabled';
  const items = [
    { key: 'countries', label: t('platform.catalog.countries'), children: <CountriesPage /> },
    { key: 'provinces', label: t('platform.catalog.provinces'), children: <ProvincesPage /> },
    { key: 'schools', label: t('platform.catalog.schools'), children: <SchoolCatalogPage /> },
    ...(canImport ? [{ key: 'import', label: t('platform.catalog.import'), children: <SchoolImportWizard /> }] : []),
  ];
  return (
    <section>
      <PageTitle title={title} />
      <Tabs items={items} destroyOnHidden />
    </section>
  );
}
