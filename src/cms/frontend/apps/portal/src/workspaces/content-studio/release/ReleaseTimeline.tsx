// ReleaseTimeline (F054-AC-01, AC-04, SC-002): listContentReleases as a timeline — publish / rollback / unpublish,
// versions, who, when (ICT), reason, schedule state; a scheduled release not run yet can be cancelled
// (cancelScheduledPublication, `learning_content.publish`, Idempotency-Key). Audit itself is server-side.
import { App as AntApp, Button, Empty, Popconfirm, Skeleton, Tag, Timeline, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { CONTENT_TYPES } from '../shared/contentTypes';
import { formatIct } from './releaseOptions';
import { useRelease, useReleases, type ContentRelease, type ItemRef } from './useRelease';

const COLOR: Record<ContentRelease['action'], string> = { publish: 'green', rollback: 'orange', unpublish: 'red' };

export function ReleaseTimeline({ item, canPublish }: { item: ItemRef; canPublish: boolean }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const q = useReleases(item);
  const { cancelSchedule } = useRelease();
  if (q.isPending) return <Skeleton active />;
  if (q.error) return <ProblemMessage error={q.error} />;
  if (q.data.data.length === 0) return <Empty description={t('content_studio.release.noReleases')} />;
  return (
    <>
      {cancelSchedule.error ? <ProblemMessage error={cancelSchedule.error} /> : null}
      <Timeline
        data-testid="release-timeline"
        items={q.data.data.map((r) => ({
          key: r.release_id,
          color: r.status === 'cancelled' || r.status === 'failed' ? 'gray' : COLOR[r.action],
          children: (
            <div data-release={r.release_id}>
              <Typography.Text strong>
                {t(`content_studio.release.action.${r.action}`, { from: r.from_version ?? '—', to: r.to_version ?? '—' })}
              </Typography.Text>{' '}
              {r.status && r.status !== 'applied' ? <Tag>{t(`content_studio.release.state.${r.status}`)}</Tag> : null}
              <div>
                <Typography.Text type="secondary">
                  {t('content_studio.release.by', { user: r.released_by, at: formatIct(r.released_at) })}
                  {r.scheduled_for ? ` · ${t('content_studio.release.scheduledFor', { at: formatIct(r.scheduled_for) })}` : ''}
                </Typography.Text>
              </div>
              {r.reason ? <Typography.Paragraph>{t('content_studio.release.reasonShown', { reason: r.reason })}</Typography.Paragraph> : null}
              {r.failure_code ? <Typography.Text type="danger">{t('content_studio.release.failed', { code: r.failure_code })}</Typography.Text> : null}
              {r.status === 'scheduled' && canPublish && (
                <ActionGate actionKey={CONTENT_TYPES[item.contentType].actions.publish} mode="disable">
                  <Popconfirm
                    title={t('content_studio.release.cancelScheduleConfirm')}
                    okText={t('content_studio.release.cancelSchedule')}
                    cancelText={t('content_studio.cancel')}
                    onConfirm={() => cancelSchedule.mutate({ ...item, releaseId: r.release_id }, { onSuccess: () => message.success(t('content_studio.release.scheduleCancelled')) })}
                  >
                    <Button size="small" loading={cancelSchedule.isPending}>
                      {t('content_studio.release.cancelSchedule')}
                    </Button>
                  </Popconfirm>
                </ActionGate>
              )}
            </div>
          ),
        }))}
      />
    </>
  );
}
