// RichContentEditor stories (F053 T016): paragraph with an inline formula, math block, image with alt, table.
import type { Meta, StoryObj } from '@storybook/react';
import { useState } from 'react';
import { RichContentEditor } from './RichContentEditor';
import type { RichContent } from './types';

function Demo() {
  const [v, setV] = useState<RichContent>({
    blocks: [
      { type: 'paragraph', text: { vi: 'Tính \\(\\frac{3}{4} + \\frac{1}{4}\\)', en: 'Compute \\(\\frac{3}{4} + \\frac{1}{4}\\)' } },
      { type: 'math', latex: '\\ce{2H2 + O2 -> 2H2O}' },
      { type: 'image', media_ref: '0193a4d2-0000-7000-8000-000000000001', alt: 'Tam giác ABC' },
      { type: 'table', rows: [['x', 'y'], ['1', '2']] },
    ],
  });
  return <RichContentEditor id="stem" label="Đề bài" value={v} onChange={setV} contentLanguage="vi" />;
}

const meta: Meta<typeof RichContentEditor> = {
  title: 'ops-ui/RichContentEditor',
  component: RichContentEditor,
  render: () => <Demo />,
};
export default meta;

type Story = StoryObj<typeof RichContentEditor>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
