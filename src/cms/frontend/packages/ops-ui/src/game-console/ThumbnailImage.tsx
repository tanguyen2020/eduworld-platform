// ThumbnailImage (F079-AC-16, FR-CMS-017, DESIGN_SYSTEM §7, ADR-0047): the 512×512 3D render of ONE version of a game
// config (`thumbnail_url` of GameConfigVersion) in lists, pickers and details. When `render_status` is not `ready`
// (pending, failed, no render) a neutral placeholder with a status label is shown instead of the image — never an
// older version's picture. Size from the Admin tokens.
import { Flex, Image, Tag, theme } from 'antd';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';

export type RenderStatus = 'pending' | 'ready' | 'failed' | 'not_applicable';

export interface ThumbnailImageProps {
  src?: string | null;
  /** `render_status` of the version; absent = ready when `src` is present. */
  status?: RenderStatus | null;
  alt: string;
  size?: 'small' | 'large';
}

export function ThumbnailImage({ src, status, alt, size = 'small' }: ThumbnailImageProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const { token } = theme.useToken();
  const px = size === 'small' ? token.controlHeightLG * 1.5 : token.controlHeightLG * 6;
  const effective: RenderStatus = status ?? (src ? 'ready' : 'not_applicable');
  if (effective === 'ready' && src) {
    return <Image src={src} alt={alt} width={px} height={px} preview={size === 'large'} />;
  }
  return (
    <Flex
      vertical
      align="center"
      justify="center"
      gap="small"
      data-testid="thumbnail-placeholder"
      data-status={effective}
      aria-label={`${alt} — ${t(`game.render.${effective}`)}`}
      style={{ width: px, height: px, border: `${token.lineWidth}px dashed ${token.colorBorder}`, borderRadius: token.borderRadius, background: token.colorFillTertiary }}
    >
      <Tag color={effective === 'failed' ? 'error' : effective === 'pending' ? 'processing' : 'default'}>{t(`game.render.${effective}`)}</Tag>
    </Flex>
  );
}
