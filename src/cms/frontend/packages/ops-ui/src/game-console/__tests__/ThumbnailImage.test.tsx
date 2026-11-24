// F079 T032 — ThumbnailImage / TurntableViewer (F079-AC-16, DESIGN_SYSTEM §7, TC-UI-009): the 512×512 render of the
// version when `render_status = ready`; `pending` / `failed` / no render → placeholder with a status label instead of
// an image; the turntable is the 24-frame sprite sheet (6×4) of the same version, turned by a slider.
import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderOps } from '../../test/render';
import { ThumbnailImage } from '../ThumbnailImage';
import { TurntableViewer } from '../TurntableViewer';

const SRC = 'https://cdn.test/media/abc/render_thumbnail.png';

describe('ThumbnailImage (F079-AC-16)', () => {
  it('ready → the image of the version', async () => {
    await renderOps(<ThumbnailImage src={SRC} status="ready" alt="Mũ phi công v3" />);
    expect(screen.getByRole('img', { name: 'Mũ phi công v3' })).toHaveAttribute('src', SRC);
  });

  it.each([
    ['pending', 'Đang render'],
    ['failed', 'Render lỗi'],
    ['not_applicable', 'Chưa có ảnh'],
  ] as const)('%s → placeholder with the label "%s", no image', async (status, label) => {
    await renderOps(<ThumbnailImage src={status === 'failed' ? SRC : undefined} status={status} alt="Mũ" />);
    expect(screen.queryByRole('img', { name: 'Mũ' })).toBeNull();
    expect(screen.getByText(label)).toBeInTheDocument();
    expect(screen.getByTestId('thumbnail-placeholder')).toHaveAttribute('data-status', status);
  });
});

describe('TurntableViewer (F079-AC-16)', () => {
  it('shows frame 1 of 24 and turns with the slider (sprite sheet 6×4)', async () => {
    await renderOps(<TurntableViewer src="https://cdn.test/media/abc/render_turntable.png" alt="Mũ" />);
    const view = screen.getByRole('img', { name: 'Mũ' });
    expect(view.style.backgroundImage).toContain('render_turntable.png');
    expect(view.style.backgroundPosition).toBe('0% 0%');
    const slider = screen.getByRole('slider');
    fireEvent.keyDown(slider, { key: 'ArrowRight', keyCode: 39 });
    expect(view.style.backgroundPosition).toBe('20% 0%');
    fireEvent.keyDown(slider, { key: 'End', keyCode: 35 });
    expect(view.style.backgroundPosition).toBe('100% 100%');
  });
});
