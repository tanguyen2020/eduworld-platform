// "Nhật ký phân quyền" (F052 T036, T060, F052-AC-18): listTenantAuditLogs limited to the role/UserRole actions of
// module identity with one query `action_prefix=role&action_prefix=user_role` (OQ F052-2); the optional "Hành động"
// field narrows to one exact action (AND on the server).
import { Form, Space } from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { QueryError } from '../common/errors';
import { AuditTable } from './AuditTable';
import { PERMISSION_AUDIT_ACTIONS, PERMISSION_AUDIT_PREFIXES, useAuditLogs } from './useAuditLogs';
import { SearchSelect } from '@eduworld/ops-ui';

/** Last 90 days [Proposed]: the API requires a range. */
export function permissionAuditRange(now: Dayjs = dayjs()): [string, string] {
  return [now.subtract(90, 'day').startOf('day').toISOString(), now.endOf('day').toISOString()];
}

export function PermissionAuditPage() {
  const { t } = useTranslation();
  const [action, setAction] = useState<(typeof PERMISSION_AUDIT_ACTIONS)[number] | undefined>();
  const [range] = useState(() => permissionAuditRange());
  const list = useAuditLogs({ actionPrefixes: PERMISSION_AUDIT_PREFIXES, action, from: range[0], to: range[1] });
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Form.Item label={t('portal.school_lms.audit.action')}>
        <SearchSelect
          allowClear
          aria-label={t('portal.school_lms.audit.action')}
          placeholder={t('portal.school_lms.audit.allPermissionActions')}
          value={action}
          onChange={(v?: (typeof PERMISSION_AUDIT_ACTIONS)[number]) => setAction(v)}
          options={PERMISSION_AUDIT_ACTIONS.map((a) => ({ value: a, label: a }))}
          style={{ minWidth: 220 }}
        />
      </Form.Item>
      {list.error ? <QueryError error={list.error} /> : null}
      <AuditTable list={list} />
    </Space>
  );
}
