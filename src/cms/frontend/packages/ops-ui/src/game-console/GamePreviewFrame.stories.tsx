// GamePreviewFrame stories (F079-AC-17, ADR-0047): the sandboxed frame of the Godot Web preview (blank page here).
import type { Meta, StoryObj } from '@storybook/react';
import { GamePreviewFrame } from './GamePreviewFrame';

const meta: Meta<typeof GamePreviewFrame> = {
  title: 'ops-ui/game-console/GamePreviewFrame',
  component: GamePreviewFrame,
  render: () => <GamePreviewFrame src="about:blank" snapshot={{ config_type: 'quests', version: 1 }} title="preview" />,
};
export default meta;
type Story = StoryObj<typeof GamePreviewFrame>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
