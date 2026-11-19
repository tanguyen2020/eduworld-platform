// TenantFilter stories (F051 T047, TC-UI-005): System Admin tenant filter, empty = all tenants (F051-AC-22) — light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { Flex } from 'antd';
import { TenantFilterView } from './TenantControls';
import { withPortal } from './storyDecorator';

// MOCK story data (not real tenants)
const TENANTS = [
  { tenant_id: '0193a4d2-0000-7000-8000-00000000000a', code: 'th-demo-a', name: 'TH Demo A' },
  { tenant_id: '0193a4d2-0000-7000-8000-00000000000b', code: 'pgd-demo', name: 'Phòng GD Demo' },
] as const;

const meta: Meta<typeof TenantFilterView> = {
  title: 'portal/TenantFilter',
  component: TenantFilterView,
  decorators: [withPortal],
  render: () => (
    <Flex vertical gap="middle">
      <TenantFilterView options={TENANTS} value={[]} onChange={() => undefined} search="" onSearch={() => undefined} />
      <TenantFilterView options={TENANTS} value={[TENANTS[0].tenant_id, TENANTS[1].tenant_id]} onChange={() => undefined} search="" onSearch={() => undefined} />
    </Flex>
  ),
};
export default meta;

type Story = StoryObj<typeof TenantFilterView>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
