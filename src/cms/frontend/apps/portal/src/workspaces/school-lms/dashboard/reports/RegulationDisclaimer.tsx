// Reference label of TT27/TT22 reports (F056 T037, F056-AC-16, FR-ANL-015, ASSESSMENT_REGULATION_PROFILE §5): every
// screen showing reference levels or exporting `regulation_reference` carries "Tham khảo — không thay đánh giá chính
// thức", plus the disclaimer text returned by the server when there is one.
import { Alert } from 'antd';
import { useTranslation } from 'react-i18next';

export function RegulationDisclaimer({ serverText }: { serverText?: string }) {
  const { t } = useTranslation();
  return <Alert type="info" showIcon data-testid="regulation-disclaimer" message={t('dashboard.reference.label')} description={serverText || undefined} />;
}
