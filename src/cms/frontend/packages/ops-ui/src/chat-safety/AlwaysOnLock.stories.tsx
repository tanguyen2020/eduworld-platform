// AlwaysOnLock stories (F122 T007): the lock with and without an explanation — light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { Flex } from 'antd';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';
import { AlwaysOnLock } from './AlwaysOnLock';

function Examples() {
  const { t } = useTranslation(OPS_UI_NS);
  return (
    <Flex vertical gap="small">
      <AlwaysOnLock />
      <AlwaysOnLock reason={t('story.alwaysOnReason')} />
    </Flex>
  );
}

const meta: Meta<typeof AlwaysOnLock> = {
  title: 'ops-ui/chat-safety/AlwaysOnLock',
  component: AlwaysOnLock,
  render: () => <Examples />,
};
export default meta;

type Story = StoryObj<typeof AlwaysOnLock>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
