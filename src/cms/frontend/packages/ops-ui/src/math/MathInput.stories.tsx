// MathInput stories (F053-AC-16): MathLive field with the Toán / Lý / Hóa keyboard, LaTeX source and preview.
import type { Meta, StoryObj } from '@storybook/react';
import { useState } from 'react';
import { MathInput } from './MathInput';

function Demo() {
  const [v, setV] = useState('\\frac{3}{4}');
  const [d, setD] = useState(false);
  return <MathInput label="Công thức" value={v} onChange={setV} display={d} onDisplayChange={setD} />;
}

const meta: Meta<typeof MathInput> = {
  title: 'ops-ui/MathInput',
  component: MathInput,
  render: () => <Demo />,
};
export default meta;

type Story = StoryObj<typeof MathInput>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
