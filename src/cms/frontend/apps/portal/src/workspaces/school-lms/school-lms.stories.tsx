// Workspace C stories (F052 T049, TC-UI-004/006): CSV import result, permission picker with a refused permission, and the
// login-history table — Light / Dark / Compact. MOCK story data only (no API, not real people).
import type { Meta, StoryObj } from '@storybook/react';
import { Flex } from 'antd';
import { withPortal } from '../../shell/storyDecorator';
import { PermissionPicker } from './roles/PermissionPicker';
import { SisImportProgress } from './sis/SisImportProgress';
import { StudentLoginTable } from './students/StudentLoginActivityPanel';

// MOCK story data
const IMPORT = {
  import_id: '0193a4d2-6000-7000-8000-0000000000e1',
  school_id: '0193a4d2-2000-7000-8000-00000000000a',
  status: 'completed_with_issues' as const,
  dry_run: false,
  rows_total: 500,
  rows_applied: 498,
  rows_rejected: 2,
  reconciliation_issue_count: 1,
  created_at: '2026-11-19T08:00:00Z',
  errors: [{ row_no: 12, column_name: 'username', error_code: 'DUPLICATE_IDENTIFIER', message: 'Trùng tên đăng nhập', occurred_at: '2026-11-19T08:00:01Z' }],
};
const PERMISSIONS = [
  { code: 'tenant.class.read', scope_kind: 'tenant' as const, group_code: 'school_lms', name: { vi: 'Xem lớp', en: 'Read classes' } },
  { code: 'lms.report.read', scope_kind: 'tenant' as const, group_code: 'school_lms', name: { vi: 'Xem báo cáo', en: 'Read reports' } },
];
const EVENTS = [
  { id: 'e1', loginAt: '2026-11-17T08:00:00Z', logoutAt: '2026-11-17T09:00:00Z', platform: 'windows' as const, deviceLabel: 'Windows · PC', clientVersion: '1.0.0', endReason: 'replaced_by_other_device' as const },
  { id: 'e2', loginAt: '2026-11-16T19:00:00Z', logoutAt: '2026-11-16T21:30:00Z', platform: 'android' as const, deviceLabel: 'Android · Tablet', clientVersion: '1.0.0', endReason: 'curfew' as const },
];

function Gallery() {
  return (
    <Flex vertical gap="large">
      <SisImportProgress value={IMPORT} onRetry={() => undefined} />
      <PermissionPicker permissions={PERMISSIONS} value={['tenant.class.read']} invalid={new Set(['lms.report.read'])} />
      <StudentLoginTable events={EVENTS} />
    </Flex>
  );
}

const meta: Meta<typeof Gallery> = { title: 'portal/SchoolLms', component: Gallery, decorators: [withPortal] };
export default meta;

type Story = StoryObj<typeof Gallery>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };
