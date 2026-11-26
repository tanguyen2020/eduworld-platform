// ChatAssetPreview (F082 T016, F082-AC-07, ADR-0047, AGENTS R32): a sticker pack or a GIF as a student sees it in the chat
// picker — sticker grid with the alt text of the chosen language (vi / en), the locked / unlocked label, the GIF static
// preview — only from the short-lived URLs module chat signs for this version (never the public CDN before publish, R31).
// Admin/Ops UI only: no game Theme or component.
import { Card, Flex, Image, Segmented, Tag, theme, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { ChatAssetType } from '../registry/configTypes';
import { GifTile } from './GifTile';
import type { GifPayload } from './gif';
import { unlockKindOf, type StickerPack } from './stickerPack';

export function ChatAssetPreview({ type, payload }: { type: ChatAssetType; payload: Record<string, unknown> }) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const [lang, setLang] = useState<'vi' | 'en'>('vi');
  const pack = payload as StickerPack;
  const kind = unlockKindOf(pack);
  return (
    <Card size="small" title={t('gameOps.chatAssets.preview.title')} data-testid="chat-asset-preview">
      {type === 'gifs' ? (
        <GifTile gif={payload as GifPayload} />
      ) : (
        <Flex vertical gap="small">
          <Segmented<'vi' | 'en'> size="small" value={lang} onChange={setLang} options={[{ value: 'vi', label: 'VI' }, { value: 'en', label: 'EN' }]} aria-label={t('gameOps.chatAssets.preview.language')} />
          <Typography.Text strong>{(lang === 'en' && pack.name_en) || pack.name_vi}</Typography.Text>
          <Tag color={kind === 'default' ? 'success' : 'warning'}>{t(kind === 'default' ? 'gameOps.chatAssets.preview.open' : `gameOps.chatAssets.preview.locked.${kind}`)}</Tag>
          {kind !== 'default' && (pack.unlock_hint_vi || pack.unlock_hint_en) && <Typography.Text type="secondary">{(lang === 'en' && pack.unlock_hint_en) || pack.unlock_hint_vi}</Typography.Text>}
          <Flex gap="small" wrap>
            {(pack.stickers ?? []).map((s, i) =>
              s.preview_url ? (
                <Image key={s.sticker_id ?? s.asset_key} src={s.preview_url} alt={(lang === 'en' && s.alt_text_en) || s.alt_text_vi || t('gameOps.chatAssets.pack.sticker', { n: i + 1 })} width={token.controlHeightLG * 2} preview={false} />
              ) : null,
            )}
          </Flex>
        </Flex>
      )}
    </Card>
  );
}
