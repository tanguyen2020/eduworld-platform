// RegradeScreen (F058 T049, F058-AC-23; menu `menu.content_studio.regrade` of the catalog): the requests of system
// re-evaluation of one question or activity version (listRegradeRequests) and the status of a request, opened from the
// version page or by version id. Read only; requests are created from the version page (RegradeEntryButton).
import { Button, Descriptions, Flex, Input, Segmented, Table, Tag, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useContentAccess } from '../shared/access';
import { useRegradeHistory, useRegradeStatus, type RegradeScope } from './useRegrade';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function RegradeScreen({ title }: { title: string }) {
  const { t } = useTranslation();
  const access = useContentAccess();
  const [scope, setScope] = useState<RegradeScope>('question_version');
  const [versionId, setVersionId] = useState('');
  const [open, setOpen] = useState<string | undefined>();
  const valid = UUID.test(versionId.trim());
  const history = useRegradeHistory(scope, valid && access.activity.reEvaluate ? versionId.trim() : undefined);
  const status = useRegradeStatus(open);
  return (
    <Flex vertical gap="middle">
      <Typography.Title level={1}>{title}</Typography.Title>
      <Typography.Paragraph type="secondary">{t('content_studio.regrade.screenHelp')}</Typography.Paragraph>
      <Flex gap="small">
        <Segmented value={scope} onChange={(v) => setScope(v as RegradeScope)} options={(['question_version', 'activity_version'] as const).map((s) => ({ value: s, label: t(`content_studio.regrade.scopes.${s}`) }))} />
        <Input aria-label={t('content_studio.regrade.versionId')} placeholder={t('content_studio.regrade.versionId')} maxLength={36} value={versionId} onChange={(e) => setVersionId(e.target.value)} style={{ maxWidth: 360 }} />
      </Flex>
      {history.error ? <ProblemMessage error={history.error} /> : null}
      <Table
        rowKey="regrade_request_id"
        loading={history.isFetching}
        pagination={false}
        data-testid="regrade-screen-history"
        dataSource={history.rows}
        footer={
          history.hasNextPage
            ? () => (
                <Button onClick={() => void history.fetchNextPage()} loading={history.isFetchingNextPage}>
                  {t('content_studio.loadMore')}
                </Button>
              )
            : undefined
        }
        onRow={(r) => ({ onClick: () => setOpen(r.regrade_request_id) })}
        columns={[
          { title: t('content_studio.regrade.createdAt'), render: (_, r) => new Date(r.created_at).toLocaleString() },
          { title: t('content_studio.regrade.action'), render: (_, r) => t(`content_studio.regrade.actions.${r.action}`) },
          { title: t('content_studio.regrade.status'), render: (_, r) => <Tag>{t(`content_studio.regrade.statuses.${r.status}`)}</Tag> },
          { title: t('content_studio.regrade.doneLabel'), render: (_, r) => t('content_studio.regrade.counts', { done: r.re_evaluated_attempts ?? 0, total: r.affected_attempts ?? 0 }) },
        ]}
      />
      {status.data && (
        <Descriptions size="small" bordered column={1} data-testid="regrade-screen-status">
          <Descriptions.Item label={t('content_studio.regrade.status')}>{t(`content_studio.regrade.statuses.${status.data.status}`)}</Descriptions.Item>
          <Descriptions.Item label={t('content_studio.regrade.reason')}>{status.data.reason}</Descriptions.Item>
          <Descriptions.Item label={t('content_studio.regrade.doneLabel')}>{t('content_studio.regrade.counts', { done: status.data.re_evaluated_attempts ?? 0, total: status.data.affected_attempts ?? 0 })}</Descriptions.Item>
        </Descriptions>
      )}
    </Flex>
  );
}
