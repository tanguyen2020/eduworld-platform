// TenantSwitcher stories (F051 T047, TC-UI-005): working tenant switcher of the topbar (F051-AC-20) — light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { TenantSwitcherView } from './TenantControls';
import { withPortal } from './storyDecorator';

// MOCK story data (not real tenants)
const TENANTS = [
  { tenant_id: '0193a4d2-0000-7000-8000-00000000000a', code: 'th-demo-a', name: 'TH Demo A' },
  { tenant_id: '0193a4d2-0000-7000-8000-00000000000b', code: 'pgd-demo', name: 'Phòng GD Demo' },
] as const;

const meta: Meta<typeof TenantSwitcherView> = {
  title: 'portal/TenantSwitcher',
  component: TenantSwitcherView,
  decorators: [withPortal],
  render: () => <TenantSwitcherView tenants={TENANTS} value={TENANTS[0].tenant_id} onChange={() => undefined} />,
};
export default meta;

type Story = StoryObj<typeof TenantSwitcherView>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
