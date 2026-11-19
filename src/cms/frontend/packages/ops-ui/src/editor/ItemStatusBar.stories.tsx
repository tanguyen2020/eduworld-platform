// ItemStatusBar stories (F053-AC-06, AC-08…AC-10, AC-23): draft with a blocker and a saved time.
import type { Meta, StoryObj } from '@storybook/react';
import { ItemStatusBar } from './ItemStatusBar';

const meta: Meta<typeof ItemStatusBar> = {
  title: 'ops-ui/ItemStatusBar',
  component: ItemStatusBar,
  render: () => (
    <ItemStatusBar
      status="draft"
      version={2}
      saveState="saved"
      lastSavedAt={new Date(2026, 10, 19, 9, 30)}
      can={{ write: true, review: true, publish: true }}
      isAuthor
      blockers={[{ field: 'stem.audio_ref', message: 'Câu hỏi lớp 1–3 thiếu âm thanh đọc đề.' }]}
      warnings={['Một số trường chưa có bản dịch.']}
      onAction={() => {}}
    />
  ),
};
export default meta;

type Story = StoryObj<typeof ItemStatusBar>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
