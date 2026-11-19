// F053 T013 — basic blocks (paragraph, heading, list, math, image with alt, audio, table, callout) are structured JSON
// (question.schema.json richBlock) — no free HTML is ever stored; blocks move up/down; image requires alt; the
// media slot of the host is used for image/audio.
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderOps } from '../test/render';
import { RichContentEditor } from './RichContentEditor';
import type { RichContent } from './types';

function Harness({ initial, onValue }: { initial: RichContent; onValue: (v: RichContent) => void }) {
  const [v, setV] = useState(initial);
  return (
    <RichContentEditor
      id="stem"
      label="Đề bài"
      value={v}
      contentLanguage="vi"
      onChange={(next) => {
        setV(next);
        onValue(next);
      }}
      mediaSlot={({ kind, onChange, id }) => (
        <button type="button" data-testid={`pick-${id}`} onClick={() => onChange(`0193a4d2-0000-7000-8000-0000000000${kind === 'image' ? '01' : '02'}`)}>
          pick {kind}
        </button>
      )}
    />
  );
}

async function addBlock(label: string) {
  fireEvent.mouseDown(screen.getAllByLabelText('Thêm khối nội dung').find((e) => e.getAttribute('role') === 'combobox')!);
  fireEvent.click(await screen.findByTitle(label));
}

describe('RichContentEditor', () => {
  it('stores structured blocks only (no HTML), every basic block type can be added', async () => {
    const onValue = vi.fn();
    await renderOps(<Harness initial={{ blocks: [{ type: 'paragraph' }] }} onValue={onValue} />);
    fireEvent.change(screen.getByLabelText('Đoạn văn (Tiếng Việt)'), { target: { value: '<b>7 × 8</b> = ?' } });
    for (const label of ['Tiêu đề', 'Danh sách', 'Công thức', 'Hình ảnh', 'Âm thanh', 'Bảng', 'Ghi chú nổi bật']) await addBlock(label);
    const last = onValue.mock.calls.at(-1)![0] as RichContent;
    expect(last.blocks.map((b) => b.type)).toEqual(['paragraph', 'heading', 'list', 'math', 'image', 'audio', 'table', 'callout']);
    // the text is kept as a plain string — it is never parsed or stored as HTML
    expect(last.blocks[0]).toEqual({ type: 'paragraph', text: '<b>7 × 8</b> = ?' });
    expect(Object.keys(last.blocks[5]!).sort()).toEqual(['type']);
  });

  it('image: media from the host slot, alt text required (help shown until filled)', async () => {
    const onValue = vi.fn();
    await renderOps(<Harness initial={{ blocks: [{ type: 'image' }] }} onValue={onValue} />);
    expect(screen.getByText('Ảnh phải có mô tả.')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('pick-stem.blocks.0.media_ref'));
    fireEvent.change(document.getElementById('stem.blocks.0.alt')!, { target: { value: 'Hình tam giác ABC' } });
    await waitFor(() => expect(screen.queryByText('Ảnh phải có mô tả.')).toBeNull());
    expect(onValue.mock.calls.at(-1)![0]).toEqual({ blocks: [{ type: 'image', media_ref: '0193a4d2-0000-7000-8000-000000000001', alt: 'Hình tam giác ABC' }] });
  });

  it('blocks move with Lên / Xuống and can be removed', async () => {
    const onValue = vi.fn();
    await renderOps(<Harness initial={{ blocks: [{ type: 'paragraph', text: 'A' }, { type: 'callout', text: 'B' }] }} onValue={onValue} />);
    const cards = screen.getAllByLabelText('Chuyển xuống dưới');
    fireEvent.click(cards[0]!);
    expect((onValue.mock.calls.at(-1)![0] as RichContent).blocks.map((b) => b.text)).toEqual(['B', 'A']);
    fireEvent.click(screen.getAllByText('Xóa')[0]!.closest('button')!);
    expect((onValue.mock.calls.at(-1)![0] as RichContent).blocks.map((b) => b.text)).toEqual(['A']);
  });
});
