// ReleaseTimelineView (F054-AC-01, F079-AC-04): the release history of one item (ContentRelease: publish / rollback /
// unpublish, versions, who, when in ICT, reason, schedule state, failure code) as a timeline with "Tải thêm"; the
// caller adds the actions of a row (cancel a scheduled release). Presentational, shared by Content Studio and the Game
// Operations Console.
import { Button, Empty, Tag, Timeline, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import type { components } from '../api/schema.gen';
import { formatIct } from './ict';

export type ContentRelease = components['schemas']['ContentRelease'];

const COLOR: Record<ContentRelease['action'], string> = { publish: 'green', rollback: 'orange', unpublish: 'red' };

export interface ReleaseTimelineViewProps {
  rows: readonly ContentRelease[];
  hasMore?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
  actions?: (r: ContentRelease) => ReactNode;
}

export function ReleaseTimelineView({ rows, hasMore, loadingMore, onLoadMore, actions }: ReleaseTimelineViewProps) {
  const { t } = useTranslation();
  if (rows.length === 0) return <Empty description={t('release.timeline.none')} />;
  return (
    <>
      <Timeline
        data-testid="release-timeline"
        items={rows.map((r) => ({
          key: r.release_id,
          color: r.status === 'cancelled' || r.status === 'failed' ? 'gray' : COLOR[r.action],
          children: (
            <div data-release={r.release_id}>
              <Typography.Text strong>{t(`release.timeline.action.${r.action}`, { from: r.from_version ?? '—', to: r.to_version ?? '—' })}</Typography.Text>{' '}
              {r.status && r.status !== 'applied' ? <Tag>{t(`release.timeline.state.${r.status}`)}</Tag> : null}
              <div>
                <Typography.Text type="secondary">
                  {t('release.timeline.by', { user: r.released_by, at: formatIct(r.released_at) })}
                  {r.scheduled_for ? ` · ${t('release.timeline.scheduledFor', { at: formatIct(r.scheduled_for) })}` : ''}
                </Typography.Text>
              </div>
              {r.reason ? <Typography.Paragraph>{t('release.timeline.reason', { reason: r.reason })}</Typography.Paragraph> : null}
              {r.failure_code ? <Typography.Text type="danger">{t('release.timeline.failed', { code: r.failure_code })}</Typography.Text> : null}
              {actions?.(r)}
            </div>
          ),
        }))}
      />
      {hasMore && (
        <Button onClick={onLoadMore} loading={loadingMore}>
          {t('release.loadMore')}
        </Button>
      )}
    </>
  );
}
