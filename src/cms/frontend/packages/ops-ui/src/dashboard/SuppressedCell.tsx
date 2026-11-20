// SuppressedCell (F056 T019, F056-AC-06, FR-ANL-016, DESIGN_SYSTEM §4.4): the ONE shared rendering of a value hidden
// because the group has fewer than 5 students (k-anonymity, LEARNING_ANALYTICS). Shows "< 5" with an explaining tooltip
// and never the real number (it takes no value at all). Reused by F080/F081 (Game Analytics, "Hoạt động game").
import { Tooltip, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';

export function SuppressedCell() {
  const { t } = useTranslation(OPS_UI_NS);
  return (
    <Tooltip title={t('dashboard.suppressed.tooltip')}>
      <Typography.Text type="secondary" data-testid="suppressed-cell" aria-label={t('dashboard.suppressed.aria')} tabIndex={0}>
        {t('dashboard.suppressed.text')}
      </Typography.Text>
    </Tooltip>
  );
}
