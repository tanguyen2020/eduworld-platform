// VersionsPanel (F079-AC-04, ADR-0043, AGENTS R11): every version of one config (listGameConfigVersions, cursor pages
// "Tải thêm") — status Tag (shared-tokens colors), current pointer, author, publish time (ICT); a published version is
// read only: no edit action, "Tạo bản nháp từ phiên bản này" (createGameConfigVersion) instead; compare two versions
// (diffGameConfigVersions, field by field); publish an approved version; rollback to another published version and
// unpublish with a mandatory reason (rollbackGameConfig / unpublishGameConfig, Idempotency-Key) when the type offers them
// (F078: no rollback for reward rules and events, no unpublish for events; F082: a sticker pack / GIF of module chat rolls
// back to a version published before — now archived — and its unpublish warns that old messages keep the asset). Buttons
// inside ActionGate; the server decides.
import { DomainTag } from '@eduworld/ops-ui';
import { App as AntApp, Button, Flex, Skeleton, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { DiffTable } from '../../../shared/release/DiffTable';
import { formatIct } from '../../../shared/release/ict';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { useConfigWrites, useDiff, useVersionPages, type ItemRef, type VersionSummary } from '../api/gameConfigs';
import { actionOf, CONFIG_TYPES } from '../registry/configTypes';
import { useGameAccess } from '../shared/access';
import { GameOpsError } from '../shared/errors';
import { PublishDialog } from './PublishDialog';

/** RollbackRequest.reason / ReasonRequest.reason maxLength (OpenAPI). */
export const RELEASE_REASON_MAX = 500;

export function VersionsPanel({ item, editorPath }: { item: ItemRef; editorPath: (version: number) => string }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const def = CONFIG_TYPES[item.configType];
  const access = useGameAccess(def);
  const q = useVersionPages(item);
  const writes = useConfigWrites();
  const [selected, setSelected] = useState<number[]>([]);
  const [compare, setCompare] = useState<[number, number] | null>(null);
  const [publishing, setPublishing] = useState<number | null>(null);
  const [rollbackTo, setRollbackTo] = useState<number | null>(null);
  const [unpublishing, setUnpublishing] = useState(false);
  const diff = useDiff(item, compare?.[0], compare?.[1]);
  if (q.isPending) return <Skeleton active />;
  if (q.error) return <GameOpsError error={q.error} />;
  const current = q.data?.pages[0]?.current_version;
  const newDraft = (basedOn: number) =>
    writes.newVersion.mutate(
      { ...item, basedOn },
      {
        onSuccess: (v) => {
          message.success(t('gameOps.versions.draftCreated', { version: v.version }));
          navigate(editorPath(v.version));
        },
      },
    );

  const columns: ColumnsType<VersionSummary> = [
    {
      key: 'version',
      title: t('gameOps.versions.version'),
      render: (_, v) => (
        <span>
          {t('gameOps.common.v', { version: v.version })} {v.version === current || v.is_current ? <Tag color="green">{t('gameOps.versions.current')}</Tag> : null}
        </span>
      ),
    },
    { key: 'status', title: t('gameOps.list.status'), render: (_, v) => <DomainTag kind="content_status" value={v.status} /> },
    { key: 'note', title: t('gameOps.versions.changeNote'), render: (_, v) => v.change_note ?? '' },
    { key: 'author', title: t('gameOps.list.author'), render: (_, v) => <Typography.Text code>{v.author_id ? v.author_id.slice(-8) : '—'}</Typography.Text> },
    {
      key: 'time',
      title: t('gameOps.versions.publishedAt'),
      render: (_, v) => (v.published_at ? formatIct(v.published_at) : v.publish_at ? t('gameOps.versions.scheduledFor', { at: formatIct(v.publish_at) }) : '—'),
    },
    {
      key: 'actions',
      title: t('gameOps.common.actions'),
      render: (_, v) => (
        <Space wrap size="small">
          <Button size="small" onClick={() => navigate(editorPath(v.version))}>
            {t('gameOps.versions.open')}
          </Button>
          {(v.status === 'published' || v.status === 'archived') && access.write && (
            <ActionGate actionKey={actionOf(def, 'edit')} mode="disable">
              <Button size="small" loading={writes.newVersion.isPending} onClick={() => newDraft(v.version)}>
                {t('gameOps.versions.newDraftFrom', { version: v.version })}
              </Button>
            </ActionGate>
          )}
          {v.status === 'approved' && access.publish && (
            <ActionGate actionKey={actionOf(def, 'publish')} mode="disable">
              <Button size="small" type="primary" onClick={() => setPublishing(v.version)}>
                {t('gameOps.versions.publish')}
              </Button>
            </ActionGate>
          )}
          {def.rollback && (v.status === 'published' || (def.rollbackArchived && v.status === 'archived' && !!v.published_at)) && current !== undefined && v.version !== current && access.publish && (
            <ActionGate actionKey={actionOf(def, 'rollback')} mode="disable">
              <Button size="small" danger onClick={() => setRollbackTo(v.version)}>
                {t('gameOps.versions.rollbackTo', { version: v.version })}
              </Button>
            </ActionGate>
          )}
        </Space>
      ),
    },
  ];

  return (
    <Flex vertical gap="middle" data-testid="versions-panel">
      <Flex gap="small" wrap align="center">
        <Typography.Text type="secondary">{current !== undefined ? t('gameOps.versions.currentIs', { version: current }) : t('gameOps.versions.notPublished')}</Typography.Text>
        <Button disabled={selected.length !== 2} onClick={() => setCompare([Math.max(...selected), Math.min(...selected)])}>
          {t('gameOps.versions.compare')}
        </Button>
        {def.unpublish && current !== undefined && access.publish && (
          <ActionGate actionKey={actionOf(def, 'rollback')} mode="disable">
            <Button danger onClick={() => setUnpublishing(true)}>
              {t('gameOps.versions.unpublish')}
            </Button>
          </ActionGate>
        )}
      </Flex>
      {writes.newVersion.error ? <GameOpsError error={writes.newVersion.error} /> : null}
      <Table<VersionSummary>
        rowKey="version"
        size="middle"
        columns={columns}
        dataSource={q.rows}
        pagination={false}
        scroll={{ x: true }}
        rowSelection={{
          selectedRowKeys: selected,
          onChange: (keys) => setSelected((keys as number[]).slice(-2)),
          getCheckboxProps: (v) => ({ 'aria-label': t('gameOps.versions.select', { version: v.version }) }) as never,
        }}
        footer={
          q.hasNextPage
            ? () => (
                <Button onClick={() => void q.fetchNextPage()} loading={q.isFetchingNextPage}>
                  {t('gameOps.common.loadMore')}
                </Button>
              )
            : undefined
        }
      />
      {compare && (
        <section aria-label={t('gameOps.versions.diffTitle', { a: compare[1], b: compare[0] })} data-testid="version-diff">
          <Typography.Title level={3}>{t('gameOps.versions.diffTitle', { a: compare[1], b: compare[0] })}</Typography.Title>
          {diff.isPending ? <Skeleton active /> : diff.error ? <GameOpsError error={diff.error} /> : diff.data ? <DiffTable changes={diff.data.changes} fromVersion={diff.data.from_version} toVersion={diff.data.to_version} /> : null}
        </section>
      )}
      <PublishDialog item={item} version={publishing} onClose={() => setPublishing(null)} />
      <ReasonDialog
        open={rollbackTo !== null}
        title={t('gameOps.versions.rollbackTitle', { from: current ?? '—', to: rollbackTo ?? '' })}
        description={t('gameOps.versions.rollbackImpact', { to: rollbackTo ?? '' })}
        maxLength={RELEASE_REASON_MAX}
        confirmLoading={writes.rollback.isPending}
        error={writes.rollback.error ? <GameOpsError error={writes.rollback.error} /> : null}
        onCancel={() => {
          setRollbackTo(null);
          writes.rollback.reset();
        }}
        onSubmit={(reason) =>
          writes.rollback.mutate(
            { ...item, toVersion: rollbackTo!, reason },
            {
              onSuccess: () => {
                message.success(t('gameOps.versions.rolledBack', { version: rollbackTo }));
                setRollbackTo(null);
              },
            },
          )
        }
      />
      <ReasonDialog
        open={unpublishing}
        title={t('gameOps.versions.unpublishTitle')}
        description={t(def.menu === 'chat_assets' ? 'gameOps.chatAssets.unpublishImpact' : 'gameOps.versions.unpublishImpact')}
        maxLength={RELEASE_REASON_MAX}
        confirmLoading={writes.unpublish.isPending}
        error={writes.unpublish.error ? <GameOpsError error={writes.unpublish.error} /> : null}
        onCancel={() => {
          setUnpublishing(false);
          writes.unpublish.reset();
        }}
        onSubmit={(reason) =>
          writes.unpublish.mutate(
            { ...item, reason },
            {
              onSuccess: () => {
                message.success(t('gameOps.versions.unpublished'));
                setUnpublishing(false);
              },
            },
          )
        }
      />
    </Flex>
  );
}
