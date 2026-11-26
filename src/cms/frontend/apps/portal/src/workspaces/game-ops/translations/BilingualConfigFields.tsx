// Translation state of a game config version in the editor (F082 T022, F082-AC-09, AC-10, AC-11; ADMIN_PORTAL §1.5): the
// server `translation_status` badge (Đủ / Thiếu / Lỗi thời), the outdated texts with "Xem thay đổi bản gốc" (diff of the
// original since `translated_in_version`) and the helpers turning `missing_fields` / `outdated_fields` into the blockers
// and warnings of the status bar. The server computes everything when the draft is saved (AGENTS R1).
import { textKey, TranslationStatusBadge, type EditorBlocker, type TranslationStatusValue } from '@eduworld/ops-ui/game-console';
import { Button, Card, Flex, List, Typography } from 'antd';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import type { OutdatedField } from './OutdatedDiffDrawer';

export interface TranslationInfo {
  translation_status?: TranslationStatusValue;
  missing_fields?: string[];
  outdated_fields?: OutdatedField[];
}

/** `missing_fields` → blockers of "Gửi duyệt" / "Xuất bản" (jump to the text). */
export function missingBlockers(info: TranslationInfo, t: TFunction): EditorBlocker[] {
  return (info.missing_fields ?? []).map((p) => ({ field: textKey(p), message: t('gameOps.translations.missing', { field: textKey(p) }) }));
}

export function TranslationState({ info, onShowDiff }: { info: TranslationInfo; onShowDiff: (f: OutdatedField) => void }) {
  const { t } = useTranslation();
  if (!info.translation_status) return null;
  const outdated = info.outdated_fields ?? [];
  return (
    <Flex vertical gap="small">
      <Flex gap="small" align="center" data-testid="editor-translation-status">
        <Typography.Text type="secondary">{t('gameOps.translations.status')}</Typography.Text>
        <TranslationStatusBadge status={info.translation_status} />
      </Flex>
      {outdated.length > 0 && (
        <Card size="small" title={t('gameOps.translations.outdatedTitle')} data-testid="outdated-fields">
          <List
            size="small"
            dataSource={outdated}
            renderItem={(f) => (
              <List.Item key={f.pointer}>
                <Flex gap="small" align="center" wrap>
                  <Typography.Text code>{textKey(f.pointer)}</Typography.Text>
                  <Typography.Text type="secondary">{t('gameOps.translations.translatedIn', { version: f.translated_in_version })}</Typography.Text>
                  <Button size="small" type="link" aria-label={t('gameOps.translations.showDiffOf', { field: textKey(f.pointer) })} onClick={() => onShowDiff(f)}>
                    {t('gameOps.translations.showDiff')}
                  </Button>
                </Flex>
              </List.Item>
            )}
          />
        </Card>
      )}
    </Flex>
  );
}
