// TranslationStatusBadge (F082-AC-09, ADMIN_PORTAL §1.5): the `translation_status` of a game config version computed by
// the server — Đủ (complete) / Thiếu (partial) / Lỗi thời (outdated). Nothing when the type has no status (chat assets).
import { Tag } from 'antd';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';

export type TranslationStatusValue = 'complete' | 'partial' | 'outdated';

const COLOR: Record<TranslationStatusValue, 'success' | 'warning' | 'error'> = { complete: 'success', partial: 'error', outdated: 'warning' };

export interface TranslationStatusBadgeProps {
  status?: TranslationStatusValue | null;
}

export function TranslationStatusBadge({ status }: TranslationStatusBadgeProps) {
  const { t } = useTranslation(OPS_UI_NS);
  if (!status) return null;
  return (
    <Tag color={COLOR[status]} data-translation-status={status}>
      {t(`translations.status.${status}`)}
    </Tag>
  );
}
