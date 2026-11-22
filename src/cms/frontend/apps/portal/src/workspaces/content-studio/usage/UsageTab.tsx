// UsageTab / UsageList (F054-AC-19, FR-CNT-008, F054 T042): where a content item is referenced — question sets,
// lessons, blueprints, activities, assignments, exam sessions — with the version used and the number of sessions
// pinned to it (getContentUsages, projection of module learning, F026). The same list is shown inside the confirm
// dialogs of archive / unpublish / delete. Read only.
import { useCursorPages } from '../../../shared/api/useCursorPages';
import { Alert, Button, Empty, List, Skeleton, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { components } from '../../../shared/api/schema.gen';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import type { ItemRef } from '../release/useRelease';

export type ContentUsage = components['schemas']['ContentUsage'];

export const usagesKey = (scope: string, r: ItemRef) => ['content-studio', 'usages', scope, r.contentType, r.contentId] as const;

export function useContentUsages(r: ItemRef, enabled = true) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  // Rows: cursor pages, "Tải thêm" (owner rule 2026-11-22).
  return useCursorPages<ContentUsage>(
    usagesKey(scopeKey, r),
    (cursor) =>
      api.call(api.client.GET('/cms/learning/content/{content_type}/{content_id}/usages', { params: { path: { content_type: r.contentType, content_id: r.contentId }, query: cursor ? { cursor } : {} } })),
    { enabled },
  );
}

export function UsageList({ item, compact = false }: { item: ItemRef; compact?: boolean }) {
  const { t } = useTranslation();
  const q = useContentUsages(item);
  if (q.isPending) return <Skeleton active paragraph={{ rows: 2 }} />;
  if (q.error) return <ProblemMessage error={q.error} />;
  const rows = q.rows;
  if (rows.length === 0) return compact ? <Typography.Text type="secondary">{t('content_studio.usage.none')}</Typography.Text> : <Empty description={t('content_studio.usage.none')} />;
  const pinned = rows.reduce((n, u) => n + u.pinned_sessions, 0);
  return (
    <>
      {compact && <Alert type="warning" showIcon message={t('content_studio.usage.warning', { count: rows.length, pinned })} />}
      <List
        size="small"
        data-testid="usage-list"
        dataSource={rows}
        loadMore={
          q.hasNextPage ? (
            <Button size="small" onClick={() => void q.fetchNextPage()} loading={q.isFetchingNextPage}>
              {t('content_studio.loadMore')}
            </Button>
          ) : null
        }
        renderItem={(u) => (
          <List.Item key={`${u.ref_type}:${u.ref_id}:${u.ref_version ?? ''}`}>
            <List.Item.Meta
              title={
                <span>
                  <Tag>{t(`content_studio.usage.refType.${u.ref_type}`)}</Tag>
                  {u.title ?? u.ref_id}
                  {u.ref_version ? ` · v${u.ref_version}` : ''}
                </span>
              }
              description={t('content_studio.usage.detail', { version: u.used_version ?? '—', pinned: u.pinned_sessions })}
            />
          </List.Item>
        )}
      />
    </>
  );
}

export function UsageTab({ item }: { item: ItemRef }) {
  return <UsageList item={item} />;
}
