// VpnRequiredNotice stories (F051 T047, TC-UI-005): VPN notice of the internal workspaces (F051-AC-14) — light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { VpnRequiredNotice } from './StatusPages';
import { withPortal } from './storyDecorator';

const meta: Meta<typeof VpnRequiredNotice> = {
  title: 'portal/VpnRequiredNotice',
  component: VpnRequiredNotice,
  decorators: [withPortal],
  render: () => <VpnRequiredNotice onClose={() => undefined} />,
};
export default meta;

type Story = StoryObj<typeof VpnRequiredNotice>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
