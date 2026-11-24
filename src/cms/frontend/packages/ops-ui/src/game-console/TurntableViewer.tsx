// TurntableViewer (F079-AC-16, DESIGN_SYSTEM §7, F078 Clarification 2026-11-24): the `render_turntable` of one version —
// a sprite sheet of 24 frames (6 columns × 4 rows, 512×512 each) — shown one frame at a time and turned with a slider
// (keyboard accessible). No 3D engine in the Admin UI: the frames are pre-rendered by module game.
import { Flex, Slider, theme, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';

export const TURNTABLE_COLUMNS = 6;
export const TURNTABLE_ROWS = 4;
export const TURNTABLE_FRAMES = TURNTABLE_COLUMNS * TURNTABLE_ROWS;

/** CSS background-position of a frame of the sheet (percentages of the free space). */
export function framePosition(frame: number): string {
  const f = ((frame % TURNTABLE_FRAMES) + TURNTABLE_FRAMES) % TURNTABLE_FRAMES;
  const col = f % TURNTABLE_COLUMNS;
  const row = Math.floor(f / TURNTABLE_COLUMNS);
  return `${(col / (TURNTABLE_COLUMNS - 1)) * 100}% ${(row / (TURNTABLE_ROWS - 1)) * 100}%`;
}

export interface TurntableViewerProps {
  src: string;
  alt: string;
}

export function TurntableViewer({ src, alt }: TurntableViewerProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const { token } = theme.useToken();
  const [frame, setFrame] = useState(0);
  const px = token.controlHeightLG * 6;
  return (
    <Flex vertical gap="small" align="center">
      <div
        role="img"
        aria-label={alt}
        style={{
          width: px,
          height: px,
          backgroundImage: `url("${src}")`,
          backgroundSize: `${TURNTABLE_COLUMNS * 100}% ${TURNTABLE_ROWS * 100}%`,
          backgroundPosition: framePosition(frame),
          backgroundRepeat: 'no-repeat',
          borderRadius: token.borderRadius,
        }}
      />
      <Slider
        min={0}
        max={TURNTABLE_FRAMES - 1}
        value={frame}
        onChange={(v) => setFrame(v)}
        tooltip={{ formatter: (v) => t('game.turntable.frame', { n: (v ?? 0) + 1, total: TURNTABLE_FRAMES }) }}
        aria-label={t('game.turntable.rotate')}
        style={{ width: px }}
      />
      <Typography.Text type="secondary">{t('game.turntable.frame', { n: frame + 1, total: TURNTABLE_FRAMES })}</Typography.Text>
    </Flex>
  );
}
