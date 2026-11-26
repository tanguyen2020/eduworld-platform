// UploadQueue stories (F082, TC-UI-004/005): light, dark and compact.
import type { Meta, StoryObj } from '@storybook/react';
import { UploadQueue } from './UploadQueue';
import type { UploadItem } from './useChecksumUpload';

const file = new File(['x'], 'sticker.webp', { type: 'image/webp' });
const ITEMS: UploadItem[] = [
  { id: '1', name: 'sticker-vui.webp', file, status: 'done', progress: 100, intent: 'a' },
  { id: '2', name: 'sticker-buon.webp', file, status: 'uploading', progress: 70, intent: 'b' },
  { id: '3', name: 'qua-lon.webp', file, status: 'error', progress: 35, intent: 'c', error: { code: 'MEDIA_TOO_LARGE', message: 'Tệp vượt giới hạn của loại', traceId: 'trace-1' } },
];

const meta: Meta<typeof UploadQueue> = {
  title: 'ops-ui/upload/UploadQueue',
  component: UploadQueue,
  render: () => <UploadQueue items={ITEMS} onRetry={() => {}} onRemove={() => {}} />,
};
export default meta;
type Story = StoryObj<typeof UploadQueue>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
