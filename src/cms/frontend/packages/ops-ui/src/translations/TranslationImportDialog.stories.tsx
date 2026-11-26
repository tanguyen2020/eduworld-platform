// TranslationImportDialog stories (F082, TC-UI-004/005): light, dark and compact.
import type { Meta, StoryObj } from '@storybook/react';
import { TranslationImportDialog } from './TranslationImportDialog';

const meta: Meta<typeof TranslationImportDialog> = {
  title: 'ops-ui/translations/TranslationImportDialog',
  component: TranslationImportDialog,
  render: () => (
    <TranslationImportDialog
      open
      target="en"
      onTargetChange={() => {}}
      onFile={() => {}}
      fileName="quests.xlf"
      uploadState="ready"
      dryRunJob={{ job_id: 'd', kind: 'import', status: 'succeeded', dry_run: true, report: { total: 2, updated: 2, skipped: 0, errors: [] } }}
      onDryRun={() => {}}
      onConfirm={() => {}}
      onClose={() => {}}
      formatTime={(s) => s}
    />
  ),
};
export default meta;
type Story = StoryObj<typeof TranslationImportDialog>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
