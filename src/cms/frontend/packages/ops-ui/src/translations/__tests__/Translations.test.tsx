// F082 T008 — presentational pieces of "Dịch nội dung game" (F082-AC-09, AC-15): status badge, export dialog (types,
// items ≤ maxItems, target language, format, only missing by default), job progress (status, link + expiry, report with
// error codes) and the import dialog (file → dry run → report → confirm). No API call: the portal wires the operations.
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { renderOps } from '../../test/render';
import { TranslationExportDialog } from '../TranslationExportDialog';
import { TranslationImportDialog } from '../TranslationImportDialog';
import { TranslationJobProgress, type TranslationJobView } from '../TranslationJobProgress';
import { TranslationStatusBadge } from '../TranslationStatusBadge';

const fmt = (iso: string) => `ICT(${iso})`;

describe('TranslationStatusBadge (F082-AC-09)', () => {
  it.each([
    ['complete', 'Đủ'],
    ['partial', 'Thiếu'],
    ['outdated', 'Lỗi thời'],
  ] as const)('F082-AC-09 %s → %s', async (status, label) => {
    await renderOps(<TranslationStatusBadge status={status} />);
    expect(screen.getByText(label)).toHaveAttribute('data-translation-status', status);
  });

  it('F082-AC-09 no status (chat/competition types) → nothing', async () => {
    const { container } = await renderOps(<TranslationStatusBadge />);
    expect(container.querySelector('[data-translation-status]')).toBeNull();
  });
});

describe('TranslationExportDialog (F082-AC-15)', () => {
  it('F082-AC-15 sends the chosen types, items, target language, format, only missing (default on)', async () => {
    const onSubmit = vi.fn();
    const onTypesChange = vi.fn();
    const user = userEvent.setup();
    await renderOps(
      <TranslationExportDialog
        open
        typeOptions={[
          { value: 'quests', label: 'Quest' },
          { value: 'npc-dialogues', label: 'Hội thoại NPC' },
        ]}
        itemOptions={[{ value: 'id-1', label: 'q_intro' }]}
        maxItems={500}
        onTypesChange={onTypesChange}
        onCancel={vi.fn()}
        onSubmit={onSubmit}
      />,
    );
    const dialog = await screen.findByRole('dialog', { name: 'Xuất file dịch' });
    await user.click(within(dialog).getByRole('combobox', { name: 'Loại nội dung' }));
    await user.click((await screen.findAllByTitle('Quest')).at(-1)!);
    expect(onTypesChange).toHaveBeenLastCalledWith(['quests']);
    await user.click(within(dialog).getByRole('combobox', { name: 'Nội dung (tối đa 500)' }));
    await user.click((await screen.findAllByTitle('q_intro')).at(-1)!);
    expect(within(dialog).getByRole('switch', { name: 'Chỉ trường thiếu hoặc lỗi thời' })).toBeChecked();
    await user.click(within(dialog).getByRole('button', { name: 'Xuất file' }));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit.mock.calls[0]![0]).toEqual({ content_types: ['quests'], content_ids: ['id-1'], target_language: 'en', source_language: 'vi', format: 'xliff', only_missing: true });
  });
});

describe('TranslationJobProgress (F082-AC-12, AC-15)', () => {
  it('F082-AC-15 a succeeded export shows the download link with its expiry', async () => {
    const job: TranslationJobView = { job_id: 'j1', kind: 'export', status: 'succeeded', download_url: 'https://exports.test/j1.xlf?sig=1', expires_at: '2026-11-26T03:15:00Z' };
    await renderOps(<TranslationJobProgress job={job} formatTime={fmt} />);
    expect(screen.getByRole('link', { name: 'Tải file' })).toHaveAttribute('href', 'https://exports.test/j1.xlf?sig=1');
    expect(screen.getByText('Link hết hạn lúc ICT(2026-11-26T03:15:00Z)')).toBeInTheDocument();
  });

  it('F082-AC-14 an import report lists errors with their code; a failed job shows its message', async () => {
    const job: TranslationJobView = {
      job_id: 'j2',
      kind: 'import',
      status: 'succeeded',
      dry_run: true,
      report: { total: 3, updated: 1, skipped: 2, errors: [{ ref: 'quests/q1/2/name', code: 'stale_version', message: 'version 2 < 3' }] },
    };
    const { rerender } = await renderOps(<TranslationJobProgress job={job} formatTime={fmt} />);
    expect(screen.getByText('Chạy thử — chưa ghi gì')).toBeInTheDocument();
    expect(screen.getByText('quests/q1/2/name')).toBeInTheDocument();
    expect(screen.getByText('Version cũ hơn version mới nhất')).toBeInTheDocument();
    expect(screen.getByText('Tổng 3 · cập nhật 1 · bỏ qua 2')).toBeInTheDocument();
    rerender(<TranslationJobProgress job={{ job_id: 'j3', kind: 'import', status: 'failed', error_message: 'file is not XLIFF 2.1' }} formatTime={fmt} />);
    expect(screen.getByText('file is not XLIFF 2.1')).toBeInTheDocument();
  });
});

describe('TranslationImportDialog (F082-AC-15)', () => {
  it('F082-AC-15 file → dry run → report → confirm; confirm only after a succeeded dry run', async () => {
    const onFile = vi.fn();
    const onDryRun = vi.fn();
    const onConfirm = vi.fn();
    const base = { open: true, target: 'en' as const, onTargetChange: vi.fn(), onFile, onDryRun, onConfirm, onClose: vi.fn(), formatTime: fmt };
    const { rerender } = await renderOps(<TranslationImportDialog {...base} uploadState="idle" />);
    const dialog = await screen.findByRole('dialog', { name: 'Nhập file dịch' });
    const input = within(dialog).getByLabelText('File dịch (XLIFF 2.1 hoặc CSV)');
    expect(input).toHaveAttribute('accept', '.xlf,.xliff,.csv');
    fireEvent.change(input, { target: { files: [new File(['x'], 'quests.xlf', { type: 'application/xliff+xml' })] } });
    expect(onFile).toHaveBeenCalledTimes(1);
    expect(within(dialog).getByRole('button', { name: 'Chạy thử' })).toBeDisabled();
    rerender(<TranslationImportDialog {...base} uploadState="ready" fileName="quests.xlf" />);
    expect(screen.getByRole('button', { name: 'Xác nhận nhập' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Chạy thử' }));
    expect(onDryRun).toHaveBeenCalledTimes(1);
    rerender(
      <TranslationImportDialog {...base} uploadState="ready" fileName="quests.xlf" dryRunJob={{ job_id: 'd', kind: 'import', status: 'succeeded', dry_run: true, report: { total: 2, updated: 2, skipped: 0, errors: [] } }} />,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Xác nhận nhập' }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });
});
