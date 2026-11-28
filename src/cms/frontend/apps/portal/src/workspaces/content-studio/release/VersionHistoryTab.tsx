// VersionHistoryTab (F054-AC-01, AC-02, AC-05, AC-06, AC-07, AC-08, AC-09, AC-28): every version of one item
// (listContentVersions: number, status as a business-color Tag, author, reviewer, publisher, change note, time,
// pinned sessions; status filter) with read-only view, compare two versions, publish now / scheduled (approved),
// rollback (published ≠ current, `content.release.rollback`), unpublish and archive (≠ current). There is no action
// that edits or deletes a published version. Buttons follow the permissions and the catalog actions (ActionGate);
// assigned platform content is read only for tenant users. The server decides every rule again (AGENTS R1, R20).
import { DomainTag, SearchSelect } from '@eduworld/ops-ui';
import { Button, Flex, Skeleton, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { CONTENT_STATUSES, CONTENT_TYPES } from '../shared/contentTypes';
import { useContentAccess } from '../shared/access';
import { ArchiveDialog } from './ArchiveDialog';
import { PublishDialog } from './PublishDialog';
import { canArchive, formatIct, rollbackTargets, type VersionSummary } from './releaseOptions';
import { RollbackDialog } from './RollbackDialog';
import { UnpublishDialog } from './UnpublishDialog';
import { useVersionPages, useVersions, type ItemRef } from './useRelease';
import type { components } from '../../../shared/api/schema.gen';

type ContentStatus = components['schemas']['ContentStatus'];
import { VersionDiffView } from './VersionDiffView';
import { VersionViewDrawer } from './VersionViewDrawer';

export interface VersionHistoryTabProps {
  item: ItemRef;
  /** Assigned platform content seen by a tenant user: read only (F053-AC-11). */
  assigned?: boolean;
  onDraftCreated?: (version: number) => void;
}

export function VersionHistoryTab({ item, assigned = false, onDraftCreated }: VersionHistoryTabProps) {
  const { t } = useTranslation();
  const access = useContentAccess();
  const [status, setStatus] = useState<ContentStatus | undefined>();
  const q = useVersionPages(item, status);
  const all = useVersions(item);
  const [selected, setSelected] = useState<number[]>([]);
  const [compare, setCompare] = useState<[number, number] | null>(null);
  const [viewing, setViewing] = useState<number | null>(null);
  const [publishing, setPublishing] = useState<number | null>(null);
  const [archiving, setArchiving] = useState<number | null>(null);
  const [rollback, setRollback] = useState<{ target?: number } | null>(null);
  const [unpublishing, setUnpublishing] = useState(false);
  const actions = CONTENT_TYPES[item.contentType].actions;
  const { write, review, publish } = access.question;
  const canRelease = !assigned;

  if (q.isPending || all.isPending) return <Skeleton active />;
  if (q.error || all.error) return <ProblemMessage error={q.error ?? all.error} />;
  const current = all.data.current_version;
  const rows = q.rows;
  const targets = rollbackTargets(all.data.data, current);

  const columns: ColumnsType<VersionSummary> = [
    {
      key: 'version',
      title: t('content_studio.release.col.version'),
      render: (_, v) => (
        <span>
          {t('content_studio.release.v', { version: v.version })} {v.version === current || v.is_current ? <Tag color="green">{t('content_studio.release.current')}</Tag> : null}
        </span>
      ),
    },
    { key: 'status', title: t('content_studio.release.col.status'), render: (_, v) => <DomainTag kind="content_status" value={v.status} /> },
    { key: 'note', title: t('content_studio.release.col.changeNote'), render: (_, v) => v.change_note ?? '' },
    { key: 'author', title: t('content_studio.release.col.author'), render: (_, v) => <Typography.Text code>{v.author_id?.slice(-8) ?? '—'}</Typography.Text> },
    { key: 'reviewer', title: t('content_studio.release.col.reviewer'), render: (_, v) => (v.reviewer_id ? <Typography.Text code>{v.reviewer_id.slice(-8)}</Typography.Text> : '—') },
    { key: 'publisher', title: t('content_studio.release.col.publisher'), render: (_, v) => (v.published_by ? <Typography.Text code>{v.published_by.slice(-8)}</Typography.Text> : '—') },
    {
      key: 'time',
      title: t('content_studio.release.col.publishedAt'),
      render: (_, v) => (v.published_at ? formatIct(v.published_at) : v.publish_at ? t('content_studio.release.scheduledFor', { at: formatIct(v.publish_at) }) : '—'),
    },
    { key: 'pinned', title: t('content_studio.release.col.pinned'), render: (_, v) => v.active_pinned_sessions ?? 0 },
    {
      key: 'actions',
      title: t('content_studio.release.col.actions'),
      render: (_, v) => (
        <Space wrap size="small">
          <Button size="small" onClick={() => setViewing(v.version)}>
            {t('content_studio.release.view')}
          </Button>
          {v.status === 'approved' && publish && canRelease && (
            <ActionGate actionKey={actions.publish} mode="disable">
              <Button size="small" type="primary" onClick={() => setPublishing(v.version)}>
                {t('content_studio.release.publish')}
              </Button>
            </ActionGate>
          )}
          {v.status === 'published' && v.version !== current && canRelease && (
            <ActionGate actionKey={actions.rollback} mode="disable">
              <Button size="small" danger onClick={() => setRollback({ target: v.version })}>
                {t('content_studio.release.rollbackTo', { version: v.version })}
              </Button>
            </ActionGate>
          )}
          {canArchive(v, current) && write && canRelease && (
            <ActionGate actionKey={actions.workflow} mode="disable">
              <Button size="small" onClick={() => setArchiving(v.version)}>
                {t('content_studio.release.archive')}
              </Button>
            </ActionGate>
          )}
        </Space>
      ),
    },
  ];

  return (
    <Flex vertical gap="middle" data-testid="version-history">
      <Flex gap="small" wrap align="center">
        <SearchSelect
          allowClear
          aria-label={t('content_studio.release.statusFilter')}
          placeholder={t('content_studio.release.statusFilter')}
          value={status}
          onChange={setStatus}
          options={CONTENT_STATUSES.map((s) => ({ value: s, label: t(`content_studio.status.${s}`) }))}
          style={{ minWidth: 180 }}
        />
        <Button disabled={selected.length !== 2} onClick={() => setCompare([Math.max(...selected), Math.min(...selected)])}>
          {t('content_studio.release.compare')}
        </Button>
        {canRelease && targets.length > 0 && (
          <ActionGate actionKey={actions.rollback} mode="disable">
            <Button danger onClick={() => setRollback({})}>
              {t('content_studio.release.rollback')}
            </Button>
          </ActionGate>
        )}
        {canRelease && current !== undefined && publish && (
          <ActionGate actionKey={actions.rollback} mode="disable">
            <Button danger onClick={() => setUnpublishing(true)}>
              {t('content_studio.release.unpublish')}
            </Button>
          </ActionGate>
        )}
      </Flex>
      <Typography.Text type="secondary">{current !== undefined ? t('content_studio.release.currentIs', { version: current }) : t('content_studio.release.notPublished')}</Typography.Text>
      <Table<VersionSummary>
        rowKey="version"
        size="middle"
        columns={columns}
        dataSource={rows}
        pagination={false}
        scroll={{ x: true }}
        footer={
          q.hasNextPage
            ? () => (
                <Button onClick={() => void q.fetchNextPage()} loading={q.isFetchingNextPage}>
                  {t('content_studio.loadMore')}
                </Button>
              )
            : undefined
        }
        rowSelection={{
          // two versions are compared: no "select all" (its header checkbox was also copied, focusable, into the
          // aria-hidden measure row of the scrolling table — axe aria-hidden-focus, F058 T057)
          hideSelectAll: true,
          selectedRowKeys: selected,
          onChange: (keys) => setSelected((keys as number[]).slice(-2)),
          getCheckboxProps: (v) => ({ 'aria-label': t('content_studio.release.select', { version: v.version }) }) as never,
        }}
      />
      {compare && <VersionDiffView item={item} version={compare[0]} against={compare[1]} canReview={review} />}
      <VersionViewDrawer item={item} version={viewing} canWrite={write} assigned={assigned} onClose={() => setViewing(null)} onDraftCreated={(v) => {
        setViewing(null);
        onDraftCreated?.(v);
      }} />
      <PublishDialog item={item} version={publishing} current={current} onClose={() => setPublishing(null)} />
      <ArchiveDialog item={item} version={archiving} onClose={() => setArchiving(null)} />
      <RollbackDialog item={item} open={rollback !== null} versions={all.data.data} current={current} initialTarget={rollback?.target} onClose={() => setRollback(null)} />
      <UnpublishDialog item={item} open={unpublishing} onClose={() => setUnpublishing(false)} />
    </Flex>
  );
}
