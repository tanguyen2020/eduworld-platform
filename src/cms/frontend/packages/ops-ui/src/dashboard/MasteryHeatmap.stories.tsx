// MasteryHeatmap stories (F056, F050-AC-18): light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { MASTERY_LEVELS, MasteryHeatmap } from './MasteryHeatmap';

const meta: Meta<typeof MasteryHeatmap> = { title: 'ops-ui/dashboard/MasteryHeatmap', component: MasteryHeatmap, render: () => <MasteryHeatmap caption="Bản đồ kỹ năng lớp 3A" rowHeader="LO" columns={MASTERY_LEVELS.map((l) => ({ key: l, label: l }))} rows={[{ key: 'lo', header: 'TOAN.3.CHIA_DU', cells: MASTERY_LEVELS.map((level, i) => ({ level, text: String(i * 3), description: level + ': ' + i * 3, muted: i === 0 })) }]} /> };
export default meta;

type Story = StoryObj<typeof MasteryHeatmap>;

export const Light: Story = { parameters: { opsTheme: { mode: "light", density: "default" } } };
export const Dark: Story = { parameters: { opsTheme: { mode: "dark", density: "default" } } };
export const Compact: Story = { parameters: { opsTheme: { mode: "light", density: "compact" } } };
