// TranslationBadge stories (F053-AC-22): Đủ / Thiếu bản dịch / Thiếu bản gốc / Lỗi thời.
import type { Meta, StoryObj } from '@storybook/react';
import { Space } from 'antd';
import { TranslationBadge } from './BilingualField';

const meta: Meta<typeof TranslationBadge> = {
  title: 'ops-ui/TranslationBadge',
  component: TranslationBadge,
  render: () => (
    <Space>
      <TranslationBadge status="complete" />
      <TranslationBadge status="partial" />
      <TranslationBadge status="missing_original" />
      <TranslationBadge status="outdated" />
    </Space>
  ),
};
export default meta;

type Story = StoryObj<typeof TranslationBadge>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
