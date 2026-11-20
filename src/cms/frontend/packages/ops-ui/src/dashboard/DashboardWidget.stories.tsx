// DashboardWidget stories (F056, F050-AC-18): light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { DashboardMetric, DashboardWidget } from './DashboardWidget';

const meta: Meta<typeof DashboardWidget> = { title: 'ops-ui/dashboard/DashboardWidget', component: DashboardWidget, render: () => (<DashboardWidget title="Tự học & gợi ý" meta={{ data_as_of: '2026-11-21T03:05:00Z', grain: 'week', is_partial: true }}><DashboardMetric label="Phút học trung bình" value={42} format="minutes" /></DashboardWidget>) };
export default meta;

type Story = StoryObj<typeof DashboardWidget>;

export const Light: Story = { parameters: { opsTheme: { mode: "light", density: "default" } } };
export const Dark: Story = { parameters: { opsTheme: { mode: "dark", density: "default" } } };
export const Compact: Story = { parameters: { opsTheme: { mode: "light", density: "compact" } } };
