// OptionFeedbackFields stories (F053-AC-03).
import type { Meta, StoryObj } from '@storybook/react';
import { useState } from 'react';
import type { OptionItem } from './mappers';
import { OptionFeedbackFields } from './OptionFeedbackFields';

function Demo() {
  const [o, setO] = useState<OptionItem>({ id: 'o_c7d0', content: { blocks: [] }, misconception_tag: 'off_by_one_factor', feedback: '63 là 7 × 9.' });
  return <OptionFeedbackFields id="render_payload.options.2" option={o} onChange={setO} contentLanguage="vi" />;
}

const meta: Meta<typeof OptionFeedbackFields> = {
  title: 'ops-ui/OptionFeedbackFields',
  component: OptionFeedbackFields,
  render: () => <Demo />,
};
export default meta;

type Story = StoryObj<typeof OptionFeedbackFields>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
