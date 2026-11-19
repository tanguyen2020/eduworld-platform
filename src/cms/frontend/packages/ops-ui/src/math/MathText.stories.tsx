// MathText stories (F053-AC-19, TC-UI-011): sample Toán / Lý / Hóa formulas, currentColor in light and dark.
import type { Meta, StoryObj } from '@storybook/react';
import { Flex } from 'antd';
import { MathText } from './MathText';

const meta: Meta<typeof MathText> = {
  title: 'ops-ui/MathText',
  component: MathText,
  render: () => (
    <Flex vertical gap="middle">
      <MathText text="Phân số \\(\\frac{3}{4}\\) và căn \\(\\sqrt{2}\\)" />
      <MathText latex="x = \\dfrac{-b \\pm \\sqrt{b^2 - 4ac}}{2a}" display />
      <MathText latex="\\vec{F} = m\\vec{a}" />
      <MathText latex="\\ce{CO2 + H2O <=> H2CO3}" display />
    </Flex>
  ),
};
export default meta;

type Story = StoryObj<typeof MathText>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
