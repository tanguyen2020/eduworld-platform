// OpsThemeProvider stories (F050-AC-05, AC-18, TC-UI-005): Ant Design components themed from the generated
// tokens — typography, buttons, system banner, form field, table — light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { Alert, Button, DatePicker, Flex, Input, Table, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';
import { DomainTag } from '../components/DomainTag/DomainTag';
import { OpsThemeProvider } from './OpsThemeProvider';

function ThemePreview() {
  const { t } = useTranslation(OPS_UI_NS);
  const rows = [
    { key: 'math', subject: 'math', status: 'published' },
    { key: 'english', subject: 'english', status: 'in_review' },
  ];
  return (
    <Flex vertical gap="middle">
      <Typography.Title level={2}>{t('story.themeTitle')}</Typography.Title>
      <Typography.Paragraph type="secondary">{t('story.themeDescription')}</Typography.Paragraph>
      <Alert type="warning" showIcon banner message={t('story.alert')} />
      <Flex gap="small">
        <Button type="primary">{t('story.primaryAction')}</Button>
        <Button>{t('story.secondaryAction')}</Button>
      </Flex>
      <Input aria-label={t('story.fieldLabel')} placeholder={t('story.fieldPlaceholder')} />
      <DatePicker aria-label={t('story.fieldLabel')} />
      <Table
        size="small"
        pagination={false}
        dataSource={rows}
        columns={[
          { title: t('domain.subject.math'), dataIndex: 'subject', render: (v: string) => <DomainTag kind="subject" value={v} /> },
          { title: t('domain.content_status.published'), dataIndex: 'status', render: (v: string) => <DomainTag kind="content_status" value={v} /> },
        ]}
      />
    </Flex>
  );
}

const meta: Meta<typeof OpsThemeProvider> = {
  title: 'ops-ui/OpsThemeProvider',
  component: OpsThemeProvider,
  render: () => <ThemePreview />,
};
export default meta;

type Story = StoryObj<typeof OpsThemeProvider>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
