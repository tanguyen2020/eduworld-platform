// WorkedExampleBlock stories (F053-AC-31): three steps, audio required (grade 3).
import type { Meta, StoryObj } from '@storybook/react';
import { useState } from 'react';
import type { WorkedExample } from './types';
import { WorkedExampleBlock } from './WorkedExampleBlock';

const p = (s: string) => ({ blocks: [{ type: 'paragraph' as const, text: s }] });
function Demo() {
  const [v, setV] = useState<WorkedExample>({
    type: 'worked_example',
    id: 'we_mul7',
    problem: p('7 × 6 = ?'),
    steps: [{ content: p('Em biết 7 × 5 = 35.') }, { content: p('7 × 6 = 7 × 5 + 7.') }, { content: p('35 + 7 = 42.') }],
    self_explain_prompt: p('Vì sao lại cộng thêm 7?'),
  });
  return <WorkedExampleBlock value={v} onChange={setV} contentLanguage="vi" requireAudio />;
}

const meta: Meta<typeof WorkedExampleBlock> = {
  title: 'ops-ui/WorkedExampleBlock',
  component: WorkedExampleBlock,
  render: () => <Demo />,
};
export default meta;

type Story = StoryObj<typeof WorkedExampleBlock>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
