// TurntableViewer stories (F079-AC-16): 24-frame sprite sheet turned with the slider.
import type { Meta, StoryObj } from '@storybook/react';
import { TurntableViewer } from './TurntableViewer';

const SHEET = 'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="3072" height="2048"/>';

const meta: Meta<typeof TurntableViewer> = { title: 'ops-ui/game-console/TurntableViewer', component: TurntableViewer, render: () => <TurntableViewer src={SHEET} alt="turntable" /> };
export default meta;
type Story = StoryObj<typeof TurntableViewer>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
