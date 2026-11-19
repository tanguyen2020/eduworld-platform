// Audit entry detail (F059 T038, F059-AC-15, spec edge case): every field of the entry and its `changes` rendered as
// read-only JSON text (React escapes it — unknown keys or markup inside values are never rendered as HTML).
import { Descriptions, Drawer, Typography, theme } from 'antd';
import { useTranslation } from 'react-i18next';
import type { AuditLogEntry } from '../api/audit';
import type { TenantSummary } from '../api/tenants';
import { formatTime, TenantCell } from '../ui';

export const changesText = (changes: AuditLogEntry['changes']): string => JSON.stringify(changes ?? {}, null, 2);

export function AuditDetailDrawer({ entry, directory, onClose }: { entry: AuditLogEntry; directory: ReadonlyMap<string, TenantSummary>; onClose: () => void }) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  return (
    <Drawer open title={t('platform.audit.detailTitle')} width={640} onClose={onClose} destroyOnHidden>
      <Descriptions column={1} size="small" bordered>
        <Descriptions.Item label={t('platform.audit.occurredAt')}>{formatTime(entry.occurred_at)}</Descriptions.Item>
        <Descriptions.Item label={t('platform.audit.actor')}>{entry.actor_id ?? t('platform.audit.systemActor')}</Descriptions.Item>
        <Descriptions.Item label={t('platform.audit.actorType')}>{t(`platform.audit.actorKind.${entry.actor_type}`)}</Descriptions.Item>
        <Descriptions.Item label={t('platform.common.tenant')}>
          <TenantCell tenantId={entry.tenant_id} directory={directory} />
        </Descriptions.Item>
        <Descriptions.Item label={t('platform.audit.action')}>{entry.action}</Descriptions.Item>
        <Descriptions.Item label={t('platform.audit.resource')}>{`${entry.target_type} · ${entry.target_id}`}</Descriptions.Item>
        <Descriptions.Item label={t('platform.audit.result')}>{t(`platform.audit.resultKind.${entry.result}`)}</Descriptions.Item>
        <Descriptions.Item label={t('platform.common.reason')}>{entry.reason}</Descriptions.Item>
        <Descriptions.Item label={t('platform.audit.traceId')}>{entry.trace_id ? <Typography.Text code copyable>{entry.trace_id}</Typography.Text> : null}</Descriptions.Item>
      </Descriptions>
      <Typography.Title level={2}>{t('platform.audit.changes')}</Typography.Title>
      <pre data-testid="audit-changes" style={{ whiteSpace: 'pre-wrap', background: token.colorFillTertiary, padding: token.paddingSM, borderRadius: token.borderRadius }}>
        {changesText(entry.changes)}
      </pre>
    </Drawer>
  );
}
