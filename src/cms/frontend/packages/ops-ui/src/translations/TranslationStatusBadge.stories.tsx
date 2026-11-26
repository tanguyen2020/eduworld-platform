// TranslationStatusBadge stories (F082, TC-UI-004/005): light, dark and compact.
import type { Meta, StoryObj } from '@storybook/react';
import { Flex } from 'antd';
import { TranslationStatusBadge } from './TranslationStatusBadge';

const meta: Meta<typeof TranslationStatusBadge> = {
  title: 'ops-ui/translations/TranslationStatusBadge',
  component: TranslationStatusBadge,
  render: () => (
    <Flex gap="small">
      <TranslationStatusBadge status="complete" />
      <TranslationStatusBadge status="partial" />
      <TranslationStatusBadge status="outdated" />
    </Flex>
  ),
};
export default meta;
type Story = StoryObj<typeof TranslationStatusBadge>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
