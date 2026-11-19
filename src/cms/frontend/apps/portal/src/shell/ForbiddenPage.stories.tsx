// ForbiddenPage stories (F051 T047, TC-UI-005): portal 403 page (F051-AC-11) — light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { ForbiddenPage } from './StatusPages';
import { withPortal } from './storyDecorator';

const meta: Meta<typeof ForbiddenPage> = {
  title: 'portal/ForbiddenPage',
  component: ForbiddenPage,
  decorators: [withPortal],
  render: () => <ForbiddenPage />,
};
export default meta;

type Story = StoryObj<typeof ForbiddenPage>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
