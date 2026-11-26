// TranslationExportDialog stories (F082, TC-UI-004/005): light, dark and compact.
import type { Meta, StoryObj } from '@storybook/react';
import { TranslationExportDialog } from './TranslationExportDialog';

const meta: Meta<typeof TranslationExportDialog> = {
  title: 'ops-ui/translations/TranslationExportDialog',
  component: TranslationExportDialog,
  render: () => (
    <TranslationExportDialog
      open
      typeOptions={[{ value: 'quests', label: 'Quest' }, { value: 'npc-dialogues', label: 'Hội thoại NPC' }]}
      itemOptions={[{ value: '1', label: 'q_intro' }]}
      maxItems={500}
      onCancel={() => {}}
      onSubmit={() => {}}
    />
  ),
};
export default meta;
type Story = StoryObj<typeof TranslationExportDialog>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
