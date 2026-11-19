// BilingualField stories (F053-AC-21, AC-22): two columns, original first with "Gốc".
import type { Meta, StoryObj } from '@storybook/react';
import { useState } from 'react';
import { BilingualField } from './BilingualField';
import type { I18nTextValue } from './translationStatus';

function Demo() {
  const [v, setV] = useState<I18nTextValue | undefined>({ vi: 'Con mèo có mấy chân?' });
  return <BilingualField id="stem" label="Đề bài" value={v} onChange={setV} contentLanguage="vi" maxLength={1000} multiline required layout="columns" />;
}

const meta: Meta<typeof BilingualField> = {
  title: 'ops-ui/BilingualField',
  component: BilingualField,
  render: () => <Demo />,
};
export default meta;

type Story = StoryObj<typeof BilingualField>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
