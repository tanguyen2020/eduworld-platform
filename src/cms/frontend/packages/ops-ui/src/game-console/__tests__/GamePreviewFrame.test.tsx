// F079 T033 — GamePreviewFrame (F079-AC-17, ADR-0047, AGENTS R32, TC-UI-010): Godot Web in a separate, sandboxed iframe;
// the snapshot read with the preview token is posted to the frame origin only (no token in the iframe URL); the game
// look stays inside the frame.
import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { renderOps } from '../../test/render';
import { GamePreviewFrame, PREVIEW_MESSAGE_TYPE } from '../GamePreviewFrame';

describe('GamePreviewFrame (F079-AC-17)', () => {
  it('renders a sandboxed iframe and posts the snapshot to the frame origin on load', async () => {
    const snapshot = { config_type: 'quests', config_id: 'c1', version: 2, payload: { code: 'q' } };
    await renderOps(<GamePreviewFrame src="https://preview.game.test/web/index.html" snapshot={snapshot} title="Preview quest" />);
    const frame = screen.getByTitle('Preview quest') as HTMLIFrameElement;
    expect(frame.tagName).toBe('IFRAME');
    expect(frame.getAttribute('sandbox')).toBe('allow-scripts allow-same-origin');
    expect(frame.src).toBe('https://preview.game.test/web/index.html');
    const post = vi.fn();
    Object.defineProperty(frame, 'contentWindow', { value: { postMessage: post } });
    fireEvent.load(frame);
    expect(post).toHaveBeenCalledWith({ type: PREVIEW_MESSAGE_TYPE, snapshot }, 'https://preview.game.test');
    expect(screen.getByText(/chỉ hiển thị trong khung/)).toBeInTheDocument();
  });
});
