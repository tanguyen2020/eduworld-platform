// DashboardMetric stories (F056, F050-AC-18): light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { DashboardMetric } from './DashboardWidget';

const meta: Meta<typeof DashboardMetric> = { title: 'ops-ui/dashboard/DashboardMetric', component: DashboardMetric, render: () => (<><DashboardMetric label="Phút học trung bình" value={42} format="minutes" /><DashboardMetric label="Hôm nay" value={null} partial /><DashboardMetric label="Học sinh" value={3} suppressed /></>) };
export default meta;

type Story = StoryObj<typeof DashboardMetric>;

export const Light: Story = { parameters: { opsTheme: { mode: "light", density: "default" } } };
export const Dark: Story = { parameters: { opsTheme: { mode: "dark", density: "default" } } };
export const Compact: Story = { parameters: { opsTheme: { mode: "light", density: "compact" } } };
