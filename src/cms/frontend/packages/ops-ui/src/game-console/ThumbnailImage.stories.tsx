// ThumbnailImage stories (F079-AC-16, DESIGN_SYSTEM §7): ready render and the pending / failed placeholders.
import type { Meta, StoryObj } from '@storybook/react';
import { Flex } from 'antd';
import { ThumbnailImage } from './ThumbnailImage';

const PIXEL = 'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512"/>';

const meta: Meta<typeof ThumbnailImage> = {
  title: 'ops-ui/game-console/ThumbnailImage',
  component: ThumbnailImage,
  render: () => (
    <Flex gap="middle">
      <ThumbnailImage src={PIXEL} status="ready" alt="ready" />
      <ThumbnailImage status="pending" alt="pending" />
      <ThumbnailImage status="failed" alt="failed" />
      <ThumbnailImage status="not_applicable" alt="none" size="large" />
    </Flex>
  ),
};
export default meta;
type Story = StoryObj<typeof ThumbnailImage>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
