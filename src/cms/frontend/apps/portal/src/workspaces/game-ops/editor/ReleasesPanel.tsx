// ReleasesPanel (F079-AC-04, spec edge case "publish hẹn giờ"): listGameConfigReleases as the shared release timeline
// ("Tải thêm"); a scheduled release not run yet shows "Đã hẹn" and can be cancelled (cancelScheduledGameConfigPublish,
// Idempotency-Key, `game.content.publish`).
import { App as AntApp, Button, Popconfirm, Skeleton } from 'antd';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ReleaseTimelineView } from '../../../shared/release/ReleaseTimelineView';
import { useConfigWrites, useReleasePages, type ItemRef } from '../api/gameConfigs';
import { actionOf, CONFIG_TYPES } from '../registry/configTypes';
import { useGameAccess } from '../shared/access';
import { GameOpsError } from '../shared/errors';

export function ReleasesPanel({ item }: { item: ItemRef }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const def = CONFIG_TYPES[item.configType];
  const access = useGameAccess(def);
  const q = useReleasePages(item);
  const { cancelSchedule } = useConfigWrites();
  if (q.isPending) return <Skeleton active />;
  if (q.error) return <GameOpsError error={q.error} />;
  return (
    <>
      {cancelSchedule.error ? <GameOpsError error={cancelSchedule.error} /> : null}
      <ReleaseTimelineView
        rows={q.rows}
        hasMore={q.hasNextPage}
        loadingMore={q.isFetchingNextPage}
        onLoadMore={() => void q.fetchNextPage()}
        actions={(r) =>
          r.status === 'scheduled' && access.publish ? (
            <ActionGate actionKey={actionOf(def, 'publish')} mode="disable">
              <Popconfirm
                title={t('gameOps.releases.cancelConfirm')}
                okText={t('gameOps.releases.cancel')}
                cancelText={t('gameOps.common.cancel')}
                onConfirm={() => cancelSchedule.mutate({ ...item, releaseId: r.release_id }, { onSuccess: () => message.success(t('gameOps.releases.cancelled')) })}
              >
                <Button size="small" loading={cancelSchedule.isPending}>
                  {t('gameOps.releases.cancel')}
                </Button>
              </Popconfirm>
            </ActionGate>
          ) : null
        }
      />
    </>
  );
}
