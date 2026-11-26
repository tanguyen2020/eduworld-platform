// GifTile (F082-AC-05, AC-07): one GIF of the library — the static preview (`preview_url`, signed by module chat), "Phát GIF"
// swaps it for the animation (`url`) and back. Images only from the server URLs; nothing is fetched from outside.
import { Button, Flex, Image, theme, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { gifTags, type GifPayload } from './gif';

export function GifTile({ gif, size }: { gif: GifPayload; size?: number }) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const [playing, setPlaying] = useState(false);
  const src = playing && gif.url ? gif.url : gif.preview_url;
  const tags = gifTags(gif);
  return (
    <Flex vertical gap="small" align="flex-start">
      {src ? (
        <Image src={src} alt={t('gameOps.chatAssets.gif.alt', { tags: tags || t('gameOps.chatAssets.gif.untagged') })} width={size ?? token.controlHeightLG * 4} preview={false} />
      ) : (
        <Typography.Text type="secondary">{t('gameOps.chatAssets.gif.noFile')}</Typography.Text>
      )}
      {gif.url && (
        <Button size="small" aria-pressed={playing} onClick={() => setPlaying((p) => !p)}>
          {playing ? t('gameOps.chatAssets.gif.stop') : t('gameOps.chatAssets.gif.play')}
        </Button>
      )}
    </Flex>
  );
}
