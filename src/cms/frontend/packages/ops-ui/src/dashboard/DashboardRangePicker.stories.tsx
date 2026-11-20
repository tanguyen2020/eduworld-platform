// DashboardRangePicker stories (F056, F050-AC-18): light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { DashboardRangePicker } from './DashboardRangePicker';

const meta: Meta<typeof DashboardRangePicker> = { title: 'ops-ui/dashboard/DashboardRangePicker', component: DashboardRangePicker, render: () => <DashboardRangePicker value={{ preset: '7d', from: '2026-11-15', to: '2026-11-21' }} today="2026-11-21" onChange={() => undefined} limits={{ maxRangeDays: 730, grainDailyMaxDays: 31, grainWeeklyMaxDays: 366 }} /> };
export default meta;

type Story = StoryObj<typeof DashboardRangePicker>;

export const Light: Story = { parameters: { opsTheme: { mode: "light", density: "default" } } };
export const Dark: Story = { parameters: { opsTheme: { mode: "dark", density: "default" } } };
export const Compact: Story = { parameters: { opsTheme: { mode: "light", density: "compact" } } };
