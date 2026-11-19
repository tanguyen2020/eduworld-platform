// Clone job view (F057 T025, F057-AC-07, edge case `partially_failed`): getTenantClone polled until
// succeeded/partially_failed/failed; shows the count, `id_map` (source → copy) and the saga `errors`. A failed or
// partially failed job is never re-run automatically.
import { Alert, Descriptions, List, Spin, Table, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useCloneJob, type TenantCloneJob } from '../api/clones';
import { isFinalJob } from '../api/common';

type IdMapRow = NonNullable<TenantCloneJob['id_map']>[number];

const STATUS_COLOR: Record<TenantCloneJob['status'], string> = {
  queued: 'default',
  running: 'processing',
  succeeded: 'success',
  partially_failed: 'warning',
  failed: 'error',
};

export function CloneJobView({ jobId }: { jobId: string }) {
  const { t } = useTranslation();
  const job = useCloneJob(jobId);
  if (job.error) return <ProblemMessage error={job.error} />;
  if (!job.data) return <Spin aria-label={t('app.loading')} />;
  const j = job.data;
  const final = isFinalJob(j.status);
  return (
    <section aria-label={t('platform.clone.jobTitle')}>
      <Typography.Title level={2}>{t('platform.clone.jobTitle')}</Typography.Title>
      <Descriptions column={1} size="small">
        <Descriptions.Item label={t('platform.common.status')}>
          <Tag color={STATUS_COLOR[j.status]} data-testid="clone-status">
            {t(`platform.clone.jobStatus.${j.status}`)}
          </Tag>
          {final ? null : <Spin size="small" />}
        </Descriptions.Item>
        <Descriptions.Item label={t('platform.tenantData.resourceType')}>{t(`platform.resourceType.${j.resource_type}`)}</Descriptions.Item>
      </Descriptions>
      {typeof j.cloned_count === 'number' ? <Typography.Paragraph>{t('platform.clone.clonedCount', { count: j.cloned_count })}</Typography.Paragraph> : null}
      {j.id_map?.length ? (
        <Table<IdMapRow>
          rowKey="source_id"
          size="small"
          pagination={false}
          title={() => t('platform.clone.idMap')}
          dataSource={j.id_map}
          columns={[
            { key: 'type', title: t('platform.tenantData.resourceType'), render: (_, r) => (r.resource_type ? t(`platform.resourceType.${r.resource_type}`) : '') },
            { key: 'source', title: t('platform.clone.sourceId'), dataIndex: 'source_id' },
            { key: 'clone', title: t('platform.clone.cloneId'), dataIndex: 'clone_id' },
          ]}
        />
      ) : null}
      {j.errors?.length ? <List header={t('platform.clone.errors')} bordered dataSource={j.errors} renderItem={(e) => <List.Item>{e}</List.Item>} /> : null}
      {j.status === 'partially_failed' || j.status === 'failed' ? <Alert type="warning" showIcon message={t('platform.clone.noAutoRetry')} /> : null}
    </section>
  );
}
