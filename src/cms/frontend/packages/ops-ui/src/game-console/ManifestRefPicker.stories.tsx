// ManifestRefPicker stories (F079-AC-07): manifest values only; a stored value missing from the list is flagged.
import type { Meta, StoryObj } from '@storybook/react';
import { useState } from 'react';
import { ManifestRefPicker } from './ManifestRefPicker';

function Demo() {
  const [value, setValue] = useState<string | string[] | undefined>('old_area');
  return <ManifestRefPicker ariaLabel="area" options={[{ value: 'main_hub', label: 'Quảng trường' }, { value: 'math_forest', label: 'Rừng Toán' }]} value={value} onChange={setValue} />;
}

const meta: Meta<typeof ManifestRefPicker> = { title: 'ops-ui/game-console/ManifestRefPicker', component: ManifestRefPicker, render: () => <Demo /> };
export default meta;
type Story = StoryObj<typeof ManifestRefPicker>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
