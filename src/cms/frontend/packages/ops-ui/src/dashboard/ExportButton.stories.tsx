// ExportButton stories (F056, F050-AC-18): light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { ExportButton } from './ExportButton';

const meta: Meta<typeof ExportButton> = { title: 'ops-ui/dashboard/ExportButton', component: ExportButton, render: () => <ExportButton onExport={() => undefined} job={{ status: 'succeeded', download_url: 'https://files.example/report.xlsx', download_expires_at: '2099-01-01T00:15:00Z' }} /> };
export default meta;

type Story = StoryObj<typeof ExportButton>;

export const Light: Story = { parameters: { opsTheme: { mode: "light", density: "default" } } };
export const Dark: Story = { parameters: { opsTheme: { mode: "dark", density: "default" } } };
export const Compact: Story = { parameters: { opsTheme: { mode: "light", density: "compact" } } };
