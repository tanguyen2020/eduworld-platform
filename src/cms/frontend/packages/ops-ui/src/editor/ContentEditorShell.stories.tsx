// ContentEditorShell stories (ADMIN_PORTAL §1.4): status bar, conflict banner, body and side panel.
import type { Meta, StoryObj } from '@storybook/react';
import { Typography } from 'antd';
import { ContentEditorShell } from './ContentEditorShell';

const meta: Meta<typeof ContentEditorShell> = {
  title: 'ops-ui/ContentEditorShell',
  component: ContentEditorShell,
  render: () => (
    <ContentEditorShell
      title="Soạn câu hỏi"
      dirty={false}
      statusBar={{ status: 'in_review', version: 1, can: { write: true, review: true, publish: true }, isAuthor: false, onAction: () => {} }}
      conflict={{ onReload: () => {} }}
      aside={<Typography.Text>Bảng thông tin</Typography.Text>}
    >
      <Typography.Paragraph>Nội dung soạn thảo</Typography.Paragraph>
    </ContentEditorShell>
  ),
};
export default meta;

type Story = StoryObj<typeof ContentEditorShell>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
