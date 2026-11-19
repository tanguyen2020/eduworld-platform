// ConflictBanner stories (F053-AC-07).
import type { Meta, StoryObj } from '@storybook/react';
import { ConflictBanner } from './ConflictBanner';

const meta: Meta<typeof ConflictBanner> = {
  title: 'ops-ui/ConflictBanner',
  component: ConflictBanner,
  render: () => <ConflictBanner onReload={() => {}} onCopyMine={() => {}} />,
};
export default meta;

type Story = StoryObj<typeof ConflictBanner>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
