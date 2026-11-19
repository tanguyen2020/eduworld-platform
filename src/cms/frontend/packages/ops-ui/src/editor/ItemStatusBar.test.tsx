// F053 T012 — F053-AC-06, AC-08, AC-09, AC-10, AC-13, AC-23: buttons by status and permission, Duyệt / Từ chối never
// for the author, published → only "Tạo bản nháp từ v{n}", blockers disable Gửi duyệt / Xuất bản and jump to fields.
import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { renderOps } from '../test/render';
import { actionsForStatus, ItemStatusBar, visibleActions, type ItemStatusBarProps } from './ItemStatusBar';

const ALL = { write: true, review: true, publish: true };
const base: ItemStatusBarProps = { status: 'draft', version: 1, can: ALL, isAuthor: true, onAction: () => {} };
const buttons = () => screen.queryAllByRole('button').map((b) => b.getAttribute('data-action')).filter(Boolean);

describe('ItemStatusBar', () => {
  it('actions by status (data-model §2)', () => {
    expect(actionsForStatus('draft')).toEqual(['save', 'submit_review']);
    expect(actionsForStatus('rejected')).toEqual(['save', 'submit_review']);
    expect(actionsForStatus('in_review')).toEqual(['approve', 'reject']);
    expect(actionsForStatus('approved')).toEqual(['publish']);
    expect(actionsForStatus('published')).toEqual(['new_draft']);
    expect(actionsForStatus('archived')).toEqual([]);
  });

  it('F053-AC-13: hidden without learning_content.write / review / publish', () => {
    expect(visibleActions({ status: 'draft', can: { write: false, review: true, publish: true }, isAuthor: false })).toEqual([]);
    expect(visibleActions({ status: 'in_review', can: { write: true, review: false, publish: true }, isAuthor: false })).toEqual([]);
    expect(visibleActions({ status: 'approved', can: { write: true, review: true, publish: false }, isAuthor: false })).toEqual([]);
    expect(visibleActions({ status: 'draft', can: ALL, isAuthor: false, readOnly: true })).toEqual([]);
  });

  it('F053-AC-09: the author never sees Duyệt / Từ chối; another reviewer does', async () => {
    const { unmount } = await renderOps(<ItemStatusBar {...base} status="in_review" isAuthor />);
    expect(buttons()).toEqual([]);
    unmount();
    await renderOps(<ItemStatusBar {...base} status="in_review" isAuthor={false} />);
    expect(buttons()).toEqual(['approve', 'reject']);
    expect(screen.getByRole('button', { name: 'Duyệt' })).toBeEnabled();
  });

  it('F053-AC-08: published → read only label "Đã xuất bản v3" and only "Tạo bản nháp từ v3"', async () => {
    const onAction = vi.fn();
    await renderOps(<ItemStatusBar {...base} status="published" version={3} onAction={onAction} />);
    expect(screen.getByText('Đã xuất bản v3')).toBeInTheDocument();
    expect(buttons()).toEqual(['new_draft']);
    fireEvent.click(screen.getByRole('button', { name: 'Tạo bản nháp từ v3' }));
    expect(onAction).toHaveBeenCalledWith('new_draft');
  });

  it('F053-AC-10: approved → "Xuất bản ngay" for learning_content.publish; busy disables other clicks', async () => {
    const onAction = vi.fn();
    await renderOps(<ItemStatusBar {...base} status="approved" isAuthor={false} busy="publish" onAction={onAction} />);
    expect(buttons()).toEqual(['publish']);
  });

  it('F053-AC-06: save state texts', async () => {
    const { rerender } = await renderOps(<ItemStatusBar {...base} saveState="saving" />);
    expect(screen.getByText('Đang lưu…')).toBeInTheDocument();
    rerender(<ItemStatusBar {...base} saveState="saved" lastSavedAt={new Date(2026, 10, 19, 9, 5)} />);
    expect(screen.getByText('Đã lưu lúc 09:05')).toBeInTheDocument();
    rerender(<ItemStatusBar {...base} saveState="error" />);
    expect(screen.getByText('Lỗi lưu, thử lại')).toBeInTheDocument();
  });

  it('F053-AC-23: blockers disable Gửi duyệt and each jumps to its field', async () => {
    const onJumpTo = vi.fn();
    await renderOps(<ItemStatusBar {...base} blockers={[{ field: 'stem.audio_ref', message: 'Thiếu âm thanh đọc đề' }]} onJumpTo={onJumpTo} />);
    expect(screen.getByRole('button', { name: 'Gửi duyệt' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Lưu' })).toBeEnabled();
    fireEvent.click(screen.getByRole('button', { name: 'Thiếu âm thanh đọc đề' }));
    expect(onJumpTo).toHaveBeenCalledWith('stem.audio_ref');
  });

  it('wrapAction lets the host gate each button (ActionGate)', async () => {
    await renderOps(<ItemStatusBar {...base} wrapAction={(a, b) => (a === 'submit_review' ? null : b)} />);
    expect(buttons()).toEqual(['save']);
  });
});
