// ReleaseTimeline (F054-AC-01, AC-04, SC-002): listContentReleases as a timeline (shared ReleaseTimelineView —
// publish / rollback / unpublish, versions, who, when (ICT), reason, schedule state); a scheduled release not run yet
// can be cancelled (cancelScheduledPublication, `learning_content.publish`, Idempotency-Key). Audit is server-side.
import { App as AntApp, Button, Popconfirm, Skeleton } from 'antd';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ReleaseTimelineView } from '../../../shared/release/ReleaseTimelineView';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { CONTENT_TYPES } from '../shared/contentTypes';
import { useRelease, useReleases, type ItemRef } from './useRelease';

export function ReleaseTimeline({ item, canPublish }: { item: ItemRef; canPublish: boolean }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const q = useReleases(item);
  const { cancelSchedule } = useRelease();
  if (q.isPending) return <Skeleton active />;
  if (q.error) return <ProblemMessage error={q.error} />;
  return (
    <>
      {cancelSchedule.error ? <ProblemMessage error={cancelSchedule.error} /> : null}
      <ReleaseTimelineView
        rows={q.rows}
        hasMore={q.hasNextPage}
        loadingMore={q.isFetchingNextPage}
        onLoadMore={() => void q.fetchNextPage()}
        actions={(r) =>
          r.status === 'scheduled' && canPublish ? (
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
          ) : null
        }
      />
    </>
  );
}
