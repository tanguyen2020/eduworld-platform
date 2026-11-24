// GamePreviewFrame (F079-AC-17, ADR-0047, AGENTS R32, ADMIN_PORTAL §5): the ONLY place the game look appears in the
// Admin Portal — the Godot Web export (URL from the runtime config, never hardcoded) in a separate sandboxed iframe. The
// portal reads the draft with the short-lived preview token (getGamePreviewSnapshot) and posts the snapshot to the
// frame's own origin once it has loaded; the token never enters the iframe URL nor the frame. No game component or
// game theme is imported here (lint ew/no-game-imports).
import { Alert, Flex, theme } from 'antd';
import { useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';

/** `type` of the message the Godot Web preview listens for. */
export const PREVIEW_MESSAGE_TYPE = 'eduworld:preview-snapshot';

export interface GamePreviewFrameProps {
  /** URL of the Godot Web preview build (runtime config `gamePreviewUrl`). */
  src: string;
  /** Snapshot returned by getGamePreviewSnapshot (config type, id, version, payload). */
  snapshot: unknown;
  title: string;
}

export function GamePreviewFrame({ src, snapshot, title }: GamePreviewFrameProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const { token } = theme.useToken();
  const ref = useRef<HTMLIFrameElement>(null);
  const origin = new URL(src).origin;
  return (
    <Flex vertical gap="small">
      <Alert type="info" showIcon message={t('game.preview.frameNotice')} />
      <iframe
        ref={ref}
        title={title}
        src={src}
        sandbox="allow-scripts allow-same-origin"
        referrerPolicy="no-referrer"
        onLoad={(e) => e.currentTarget.contentWindow?.postMessage({ type: PREVIEW_MESSAGE_TYPE, snapshot }, origin)}
        style={{ width: '100%', height: token.controlHeightLG * 14, border: `${token.lineWidth}px solid ${token.colorBorder}`, borderRadius: token.borderRadius }}
      />
    </Flex>
  );
}
