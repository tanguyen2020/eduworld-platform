// NoAccessPage stories (F051 T047, TC-UI-005): no admin access / student account (F051-AC-03, F051-AC-04) — light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { Flex } from 'antd';
import { NoAccessPage } from './StatusPages';
import { withPortal } from './storyDecorator';

const meta: Meta<typeof NoAccessPage> = {
  title: 'portal/NoAccessPage',
  component: NoAccessPage,
  decorators: [withPortal],
  render: () => (
    <Flex vertical>
      <NoAccessPage reason="no_role" onLogout={() => undefined} />
      <NoAccessPage reason="student" onLogout={() => undefined} />
    </Flex>
  ),
};
export default meta;

type Story = StoryObj<typeof NoAccessPage>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
