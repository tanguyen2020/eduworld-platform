// MathInsertButton stories (F053-AC-16).
import type { Meta, StoryObj } from '@storybook/react';
import { MathInsertButton } from './MathInsertButton';

const meta: Meta<typeof MathInsertButton> = {
  title: 'ops-ui/MathInsertButton',
  component: MathInsertButton,
  render: () => <MathInsertButton onInsert={() => {}} />,
};
export default meta;

type Story = StoryObj<typeof MathInsertButton>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
