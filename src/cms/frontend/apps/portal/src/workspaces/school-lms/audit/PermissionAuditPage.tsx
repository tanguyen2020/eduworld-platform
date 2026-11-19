// "Nhật ký phân quyền" (F052 T036, F052-AC-18): listTenantAuditLogs limited to the role/UserRole actions of module
// identity (`role.*`, `user_role.*`). The API filters `action` by exact value, so one action is chosen at a time
// [Proposed: prefix filter on the server — Câu hỏi mở F052].
import { Form, Segmented, Space } from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { QueryError } from '../common/errors';
import { AuditTable } from './AuditTable';
import { PERMISSION_AUDIT_ACTIONS, useAuditLogs } from './useAuditLogs';

/** Last 90 days [Proposed]: the API requires a range. */
export function permissionAuditRange(now: Dayjs = dayjs()): [string, string] {
  return [now.subtract(90, 'day').startOf('day').toISOString(), now.endOf('day').toISOString()];
}

export function PermissionAuditPage() {
  const { t } = useTranslation();
  const [action, setAction] = useState<(typeof PERMISSION_AUDIT_ACTIONS)[number]>('user_role.granted');
  const [range] = useState(() => permissionAuditRange());
  const list = useAuditLogs({ action, from: range[0], to: range[1] });
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Form.Item label={t('portal.school_lms.audit.action')}>
        <Segmented value={action} onChange={(v) => setAction(v as typeof action)} options={PERMISSION_AUDIT_ACTIONS.map((a) => ({ value: a, label: a }))} />
      </Form.Item>
      {list.error ? <QueryError error={list.error} /> : null}
      <AuditTable list={list} />
    </Space>
  );
}
