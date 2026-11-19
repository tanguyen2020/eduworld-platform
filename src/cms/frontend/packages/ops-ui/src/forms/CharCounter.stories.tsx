// CharCounter + openapiRules stories (F050-AC-09, AC-18): a field limited by an OpenAPI schema
// (maxLength 20, required) with its counter, light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { Form, Input } from 'antd';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';
import { CharCounter, openapiInputProps } from './CharCounter';
import { openapiRules, type OpenapiStringSchema } from './openapiRules';

const schema: OpenapiStringSchema = { type: 'string', maxLength: 20, pattern: '^[0-9A-Za-z]+$' };

function SchemaField() {
  const { t } = useTranslation(OPS_UI_NS);
  return (
    <Form layout="vertical" initialValues={{ name: '3A' }}>
      <Form.Item name="name" label={t('story.fieldLabel')} rules={openapiRules(schema, { required: true, t })}>
        <Input placeholder={t('story.fieldPlaceholder')} {...openapiInputProps(schema)} />
      </Form.Item>
      <CharCounter count={2} max={20} />
    </Form>
  );
}

const meta: Meta<typeof CharCounter> = {
  title: 'ops-ui/CharCounter',
  component: CharCounter,
  render: () => <SchemaField />,
};
export default meta;

type Story = StoryObj<typeof CharCounter>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
