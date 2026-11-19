// "Nhật ký" (F052 T046, F052-AC-23): listTenantAuditLogs filtered by school / actor / action / date range (default:
// the last 7 days), `changes` and `trace_id` shown; no moderation entry.
import { DatePicker, Flex, Form, Input, Space } from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { QueryError } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { SchoolSelect } from '../common/SchoolSelect';
import { AuditTable } from './AuditTable';
import { useAuditLogs, type AuditFilters } from './useAuditLogs';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function defaultRange(now: Dayjs = dayjs()): [Dayjs, Dayjs] {
  return [now.subtract(7, 'day').startOf('day'), now.endOf('day')];
}

export function AuditLogPage() {
  const { t } = useTranslation();
  const [range, setRange] = useState<[Dayjs, Dayjs]>(defaultRange);
  const [filters, setFilters] = useState<Omit<AuditFilters, 'from' | 'to'>>({});
  const [actor, setActor] = useState('');
  const list = useAuditLogs({ ...filters, from: range[0].toISOString(), to: range[1].toISOString() });
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Flex wrap="wrap" gap="middle" align="end">
        <Form.Item label={t('portal.school_lms.common.school')}>
          <SchoolSelect value={filters.schoolId} onChange={(schoolId) => setFilters((f) => ({ ...f, schoolId }))} allowClear autoSelect={false} />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.audit.action')}>
          <Input.Search allowClear maxLength={FIELDS.auditAction.maxLength} placeholder={t('portal.school_lms.audit.actionHint')} onSearch={(action) => setFilters((f) => ({ ...f, action: action.trim() || undefined }))} />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.audit.actorId')} validateStatus={actor && !UUID.test(actor) ? 'error' : undefined}>
          <Input.Search allowClear maxLength={36} value={actor} onChange={(e) => setActor(e.target.value)} onSearch={(v) => setFilters((f) => ({ ...f, actorId: UUID.test(v.trim()) ? v.trim() : undefined }))} />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.audit.range')}>
          <DatePicker.RangePicker value={range} allowClear={false} onChange={(v) => v?.[0] && v[1] && setRange([v[0].startOf('day'), v[1].endOf('day')])} />
        </Form.Item>
      </Flex>
      {list.error ? <QueryError error={list.error} /> : null}
      <AuditTable list={list} />
    </Space>
  );
}
