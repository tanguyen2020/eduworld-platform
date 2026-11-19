// Audit rows (F052-AC-18, F052-AC-23): who, when, action, target, result, reason, before/after `changes` (already
// PII-filtered by the server) and the copyable `trace_id`.
import { Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { formatDateTime } from '../common/format';
import { ResponsiveTable } from '../common/ResponsiveTable';
import type { AuditEntry } from './useAuditLogs';

export function ChangesView({ changes }: { changes?: Record<string, unknown> }) {
  if (!changes || Object.keys(changes).length === 0) return null;
  return (
    <Typography.Text code data-testid="audit-changes">
      {JSON.stringify(changes)}
    </Typography.Text>
  );
}

export function AuditTable({ list }: { list: { items: AuditEntry[]; isFetching: boolean; hasMore: boolean; loadMore: () => void } }) {
  const { t } = useTranslation();
  const columns: ColumnType<AuditEntry>[] = [
    { key: 'title', title: t('portal.school_lms.audit.occurredAt'), render: (_, e) => formatDateTime(e.occurred_at) },
    { key: 'actor', title: t('portal.school_lms.audit.actor'), render: (_, e) => `${t(`portal.school_lms.audit.actorTypes.${e.actor_type}`)}${e.actor_id ? ` · ${e.actor_id}` : ''}` },
    { key: 'action', title: t('portal.school_lms.audit.action'), dataIndex: 'action' },
    { key: 'target', title: t('portal.school_lms.audit.target'), render: (_, e) => `${e.target_type} · ${e.target_id}` },
    { key: 'result', title: t('portal.school_lms.audit.result'), render: (_, e) => t(`portal.school_lms.audit.results.${e.result}`) },
    { key: 'reason', title: t('portal.school_lms.common.reason'), render: (_, e) => e.reason ?? '' },
    { key: 'changes', title: t('portal.school_lms.audit.changes'), render: (_, e) => <ChangesView changes={e.changes} /> },
    { key: 'trace', title: t('portal.school_lms.audit.traceId'), render: (_, e) => (e.trace_id ? <Typography.Text copyable>{e.trace_id}</Typography.Text> : '') },
  ];
  return <ResponsiveTable<AuditEntry> rowKey="audit_id" columns={columns} data={list.items} loading={list.isFetching} cardTitle={(e) => `${formatDateTime(e.occurred_at)} · ${e.action}`} hasMore={list.hasMore} onLoadMore={list.loadMore} />;
}
