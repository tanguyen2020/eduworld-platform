// DashboardChart stories (F056, F050-AC-18): light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { DashboardChart } from './DashboardChart';

const meta: Meta<typeof DashboardChart> = { title: 'ops-ui/dashboard/DashboardChart', component: DashboardChart, render: () => <DashboardChart kind="bar" categories={['0', '0,5', '1', '1,5']} values={[0, 2, 5, 3]} ariaLabel="Phổ điểm: 0: 0; 0,5: 2; 1: 5; 1,5: 3" /> };
export default meta;

type Story = StoryObj<typeof DashboardChart>;

export const Light: Story = { parameters: { opsTheme: { mode: "light", density: "default" } } };
export const Dark: Story = { parameters: { opsTheme: { mode: "dark", density: "default" } } };
export const Compact: Story = { parameters: { opsTheme: { mode: "light", density: "compact" } } };
