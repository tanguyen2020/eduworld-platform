// ResponsiveTable of workspace C (F052-AC-27): the shared shell table with the school_lms "Xem thêm" label.
import { useTranslation } from 'react-i18next';
import { ResponsiveTable as ShellResponsiveTable, type ResponsiveTableProps } from '../../../shell/ResponsiveTable';

export { MOBILE_QUERY } from '../../../shell/ResponsiveTable';

export function ResponsiveTable<T extends object>(props: Omit<ResponsiveTableProps<T>, 'loadMoreLabel'>) {
  const { t } = useTranslation();
  return <ShellResponsiveTable<T> {...props} loadMoreLabel={t('portal.school_lms.common.loadMore')} />;
}
