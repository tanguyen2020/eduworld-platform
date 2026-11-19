// QuestionEditor stories (F053 T059, TC-UI-005): one story per QuestionType after Light / Dark / Compact.
import type { Meta, StoryObj } from '@storybook/react';
import { useState } from 'react';
import { newQuestionDraft, type QuestionDraft, type QuestionType } from './mappers';
import { QuestionEditor } from './QuestionEditor';

function Demo({ type }: { type: QuestionType }) {
  const [v, setV] = useState<QuestionDraft>(() => ({ ...newQuestionDraft(type), stem: { blocks: [{ type: 'paragraph', text: 'Đề bài mẫu' }] } }));
  return <QuestionEditor value={v} onChange={setV} contentLanguage="vi" />;
}

const meta: Meta<typeof QuestionEditor> = {
  title: 'ops-ui/QuestionEditor',
  component: QuestionEditor,
  render: () => <Demo type="single_choice" />,
};
export default meta;

type Story = StoryObj<typeof QuestionEditor>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
export const MultipleChoice: Story = { render: () => <Demo type="multiple_choice" /> };
export const TrueFalse: Story = { render: () => <Demo type="true_false" /> };
export const FillInBlank: Story = { render: () => <Demo type="fill_in_blank" /> };
export const ShortAnswer: Story = { render: () => <Demo type="short_answer" /> };
export const Numeric: Story = { render: () => <Demo type="numeric" /> };
export const Matching: Story = { render: () => <Demo type="matching" /> };
export const Ordering: Story = { render: () => <Demo type="ordering" /> };
export const DragDrop: Story = { render: () => <Demo type="drag_drop" /> };
export const ImageBased: Story = { render: () => <Demo type="image_based" /> };
export const Interactive: Story = { render: () => <Demo type="interactive" /> };
