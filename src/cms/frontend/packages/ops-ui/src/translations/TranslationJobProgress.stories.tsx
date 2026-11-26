// TranslationJobProgress stories (F082, TC-UI-004/005): light, dark and compact.
import type { Meta, StoryObj } from '@storybook/react';
import { Flex } from 'antd';
import { TranslationJobProgress } from './TranslationJobProgress';

const meta: Meta<typeof TranslationJobProgress> = {
  title: 'ops-ui/translations/TranslationJobProgress',
  component: TranslationJobProgress,
  render: () => (
    <Flex vertical gap="large">
      <TranslationJobProgress job={{ job_id: '1', kind: 'export', status: 'running' }} formatTime={(s) => s} />
      <TranslationJobProgress job={{ job_id: '2', kind: 'export', status: 'succeeded', download_url: 'https://exports.example/1.xlf', expires_at: '2026-11-26T03:15:00Z' }} formatTime={(s) => s} />
      <TranslationJobProgress
        job={{ job_id: '3', kind: 'import', status: 'succeeded', dry_run: true, report: { total: 3, updated: 1, skipped: 2, errors: [{ ref: 'quests/q1/2/name', code: 'stale_version', message: 'version 2 < 3' }] } }}
        formatTime={(s) => s}
      />
      <TranslationJobProgress job={{ job_id: '4', kind: 'import', status: 'failed', error_message: 'file is not XLIFF 2.1' }} formatTime={(s) => s} />
    </Flex>
  ),
};
export default meta;
type Story = StoryObj<typeof TranslationJobProgress>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
