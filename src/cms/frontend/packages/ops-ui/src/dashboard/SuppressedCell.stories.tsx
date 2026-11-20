// SuppressedCell stories (F056, F050-AC-18): light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { SuppressedCell } from './SuppressedCell';

const meta: Meta<typeof SuppressedCell> = { title: 'ops-ui/dashboard/SuppressedCell', component: SuppressedCell, render: () => <SuppressedCell /> };
export default meta;

type Story = StoryObj<typeof SuppressedCell>;

export const Light: Story = { parameters: { opsTheme: { mode: "light", density: "default" } } };
export const Dark: Story = { parameters: { opsTheme: { mode: "dark", density: "default" } } };
export const Compact: Story = { parameters: { opsTheme: { mode: "light", density: "compact" } } };
