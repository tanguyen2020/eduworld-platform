import { Result } from 'antd';
import { useTranslation } from 'react-i18next';

export function NotFound() {
  const { t } = useTranslation();
  return <Result status="404" title={t('app.notFound')} />;
}
