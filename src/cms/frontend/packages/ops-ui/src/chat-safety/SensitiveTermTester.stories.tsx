// SensitiveTermTester stories (F122 T007): a test result with placeholder text only (R15: no real sensitive word in
// code, tests or stories) — light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { SensitiveTermTester } from './SensitiveTermTester';

const TEXT = 'ban la zqplaceholder that';

const meta: Meta<typeof SensitiveTermTester> = {
  title: 'ops-ui/chat-safety/SensitiveTermTester',
  component: SensitiveTermTester,
  render: () => (
    <SensitiveTermTester
      maxLength={500}
      onTest={() => undefined}
      result={{ text: TEXT, outcome: { decision: 'mask', masked_text: 'ban la ************* that', matches: [{ category_code: 'insult', severity: 'medium', action: 'mask', start: 7, end: 20 }] } }}
    />
  ),
};
export default meta;

type Story = StoryObj<typeof SensitiveTermTester>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
