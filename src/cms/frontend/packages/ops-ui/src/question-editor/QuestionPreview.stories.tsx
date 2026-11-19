// QuestionPreview stories (F053-AC-03): what the student sees — no feedback, no answer.
import type { Meta, StoryObj } from '@storybook/react';
import { QuestionPreview } from './QuestionPreview';

const p = (s: string) => ({ blocks: [{ type: 'paragraph' as const, text: s }] });

const meta: Meta<typeof QuestionPreview> = {
  title: 'ops-ui/QuestionPreview',
  component: QuestionPreview,
  render: () => (
    <QuestionPreview
      type="single_choice"
      stem={p('Tính \\(\\frac{3}{4} + \\frac{1}{4}\\)')}
      renderPayload={{ options: [{ id: 'o_a1', content: p('1') }, { id: 'o_b2', content: p('\\(\\frac{4}{8}\\)'), feedback: 'ẩn' }] }}
      language="vi"
      contentLanguage="vi"
    />
  ),
};
export default meta;

type Story = StoryObj<typeof QuestionPreview>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
