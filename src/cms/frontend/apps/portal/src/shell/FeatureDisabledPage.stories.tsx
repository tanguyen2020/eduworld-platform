// FeatureDisabledPage stories (F051 T047, TC-UI-005): "Tính năng đang tắt" page (F051-AC-35) — light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { FeatureDisabledPage } from './StatusPages';
import { withPortal } from './storyDecorator';

const meta: Meta<typeof FeatureDisabledPage> = {
  title: 'portal/FeatureDisabledPage',
  component: FeatureDisabledPage,
  decorators: [withPortal],
  render: () => <FeatureDisabledPage featureKey="menu.school_lms.assignments" />,
};
export default meta;

type Story = StoryObj<typeof FeatureDisabledPage>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
