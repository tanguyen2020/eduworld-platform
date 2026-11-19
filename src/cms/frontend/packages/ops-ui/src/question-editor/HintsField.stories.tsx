// HintsField stories (F053-AC-03): two hint levels.
import type { Meta, StoryObj } from '@storybook/react';
import { useState } from 'react';
import type { RichContent } from '../rich-content/types';
import { HintsField } from './HintsField';

const p = (s: string) => ({ blocks: [{ type: 'paragraph' as const, text: s }] });
function Demo() {
  const [h, setH] = useState<RichContent[]>([p('Em đã thuộc 7 × 7 chưa?'), p('7 × 8 = 7 × 7 + 7')]);
  return <HintsField value={h} onChange={setH} contentLanguage="vi" />;
}

const meta: Meta<typeof HintsField> = {
  title: 'ops-ui/HintsField',
  component: HintsField,
  render: () => <Demo />,
};
export default meta;

type Story = StoryObj<typeof HintsField>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
