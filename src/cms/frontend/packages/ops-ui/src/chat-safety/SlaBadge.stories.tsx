// SlaBadge stories (F122 T007): pending, overdue and done deadlines — light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { Flex } from 'antd';
import { SlaBadge } from './SlaBadge';

const NOW = new Date('2026-11-23T08:00:00Z');

const meta: Meta<typeof SlaBadge> = {
  title: 'ops-ui/chat-safety/SlaBadge',
  component: SlaBadge,
  render: () => (
    <Flex gap="small" wrap>
      <SlaBadge dueAt="2026-11-23T20:00:00Z" now={NOW} />
      <SlaBadge dueAt="2026-11-23T06:00:00Z" now={NOW} />
      <SlaBadge dueAt="2026-11-23T06:00:00Z" now={NOW} done />
    </Flex>
  ),
};
export default meta;

type Story = StoryObj<typeof SlaBadge>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
