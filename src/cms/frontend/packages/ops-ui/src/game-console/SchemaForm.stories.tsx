// SchemaForm stories (F079-AC-02, DESIGN_SYSTEM §4): a season-track-like payload generated from its JSON Schema —
// bilingual name, enum, number, nested list; const fields (paid_track) have no control. Light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { useState } from 'react';
import { SchemaForm } from './SchemaForm';
import { initialValue, type JsonSchema } from './schemaModel';

const schema: JsonSchema = {
  type: 'object',
  required: ['code', 'name', 'paid_track', 'tiers'],
  properties: {
    code: { type: 'string', maxLength: 64 },
    name: { type: 'object', required: ['vi', 'en'], properties: { vi: { type: 'string', maxLength: 255 }, en: { type: 'string', maxLength: 255 } } },
    theme_code: { enum: ['spring', 'summer'] },
    paid_track: { const: false },
    tiers: { type: 'array', maxItems: 50, items: { type: 'object', required: ['tier'], properties: { tier: { type: 'integer', minimum: 1, maximum: 50 }, is_milestone: { type: 'boolean' } } } },
  },
};

function Demo() {
  const [value, setValue] = useState<unknown>({ ...(initialValue(schema) as object), code: 'spring_2027', name: { vi: 'Mùa xuân', en: '' }, tiers: [{ tier: 1 }] });
  return <SchemaForm schema={schema} value={value} onChange={setValue} label={(k) => k} errors={{ 'name/en': 'Thiếu bản tiếng Anh' }} />;
}

const meta: Meta<typeof SchemaForm> = { title: 'ops-ui/game-console/SchemaForm', component: SchemaForm, render: () => <Demo /> };
export default meta;
type Story = StoryObj<typeof SchemaForm>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
