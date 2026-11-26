// ConfigListPage (F079-AC-01, AC-16, ADMIN_PORTAL §1.6, §3.3, §3.12): the configs of one `config_type` of a menu
// (listGameConfigs: newest version of each config, `status` filter, cursor pages with "Tải thêm" — paged in the database,
// never loaded whole) with code, title, status Tag (shared-tokens colors), version, author, update time (ICT) and the
// thumbnail of THAT version (placeholder + status while the render is pending / failed). The menu's other types are tabs.
// "Tạo mới" is inside ActionGate. A type module game does not serve yet says so (no API call).
import { DomainTag, SearchSelect } from '@eduworld/ops-ui';
import { ThumbnailImage } from '@eduworld/ops-ui/game-console';
import { Alert, Button, Flex, Segmented, Table, Tabs, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { formatIct } from '../../../shared/release/ict';
import { useConfigPages, type ContentStatus, type GameConfigVersion } from '../api/gameConfigs';
import { actionOf, codeOf, COMPETITION_GROUPS, CONFIG_TYPES, isConfigType, nameOf, typesOfGroup, typesOfMenu, type GameOpsMenu } from '../registry/configTypes';
import { GameOpsError } from '../shared/errors';

const STATUSES: ContentStatus[] = ['draft', 'in_review', 'approved', 'published', 'archived', 'rejected'];

export function ConfigListPage({ title, menu, extraTabs = [] }: { title: string; menu: GameOpsMenu; extraTabs?: { key: string; label: string }[] }) {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { configType } = useParams();
  const [params, setParams] = useSearchParams();
  const lang = i18n.language.startsWith('en') ? 'en' : 'vi';
  const menuTypes = typesOfMenu(menu);
  const def = isConfigType(configType) ? CONFIG_TYPES[configType] : menuTypes[0]!;
  // "Thi đấu" (F108 US2): the 5 groups first, then the types of the group as tabs.
  const types = def.group ? typesOfGroup(def.group) : menuTypes;
  const status = (params.get('status') ?? undefined) as ContentStatus | undefined;
  const q = useConfigPages(def.type, status, def.served);

  const columns: ColumnsType<GameConfigVersion> = [
    ...(def.has3D
      ? [{ key: 'thumb', title: t('gameOps.list.thumbnail'), render: (_: unknown, v: GameConfigVersion) => <ThumbnailImage src={v.thumbnail_url} status={v.render_status} alt={t('gameOps.editor.thumbnailAlt', { name: v.title ?? codeOf(def, v.payload), version: v.version })} /> }]
      : []),
    { key: 'code', title: t('gameOps.list.code'), render: (_, v) => <Typography.Text code>{codeOf(def, v.payload) || v.config_id.slice(-8)}</Typography.Text> },
    { key: 'title', title: t('gameOps.list.title'), render: (_, v) => v.title || nameOf(def, v.payload, lang) || '—' },
    { key: 'status', title: t('gameOps.list.status'), render: (_, v) => <DomainTag kind="content_status" value={v.status} /> },
    { key: 'version', title: t('gameOps.list.version'), render: (_, v) => <span>{t('gameOps.common.v', { version: v.version })} {v.is_current ? <Tag color="green">{t('gameOps.versions.current')}</Tag> : null}</span> },
    { key: 'author', title: t('gameOps.list.author'), render: (_, v) => <Typography.Text code>{v.author_id ? v.author_id.slice(-8) : '—'}</Typography.Text> },
    { key: 'updated', title: t('gameOps.list.updatedAt'), render: (_, v) => formatIct(v.updated_at) },
  ];

  return (
    <Flex vertical gap="middle" data-testid="config-list">
      <Typography.Title level={1}>{title}</Typography.Title>
      {def.group && (
        <Segmented
          aria-label={t('gameOps.competition.groups')}
          value={def.group}
          onChange={(g) => navigate(`../${typesOfGroup(g as never)[0]!.type}`, { relative: 'path' })}
          options={COMPETITION_GROUPS.map((g) => ({ value: g, label: t(`gameOps.competition.group.${g}`) }))}
          data-testid="competition-groups"
        />
      )}
      {(types.length > 1 || extraTabs.length > 0) && (
        <Tabs
          activeKey={def.type}
          onChange={(k) => navigate(`../${k}`, { relative: 'path' })}
          items={[...types.map((d) => ({ key: d.type, label: t(`gameOps.type.${d.type}`) })), ...extraTabs]}
        />
      )}
      {!def.served ? (
        <Alert type="info" showIcon message={t('gameOps.list.notServed', { type: t(`gameOps.type.${def.type}`) })} data-testid="type-not-served" />
      ) : (
        <>
          <Flex gap="small" wrap align="center" justify="space-between">
            <SearchSelect
              allowClear
              aria-label={t('gameOps.list.status')}
              placeholder={t('gameOps.list.statusFilter')}
              value={status}
              onChange={(v) => {
                const next = new URLSearchParams(params);
                if (v) next.set('status', String(v));
                else next.delete('status');
                setParams(next, { replace: true });
              }}
              options={STATUSES.map((s) => ({ value: s, label: t(`gameOps.status.${s}`) }))}
            />
            <ActionGate actionKey={actionOf(def, 'edit')}>
              <Button type="primary" onClick={() => navigate('new')}>
                {t('gameOps.list.create', { type: t(`gameOps.type.${def.type}`) })}
              </Button>
            </ActionGate>
          </Flex>
          {q.error ? (
            <GameOpsError error={q.error} />
          ) : (
            <Table<GameConfigVersion>
              rowKey="config_id"
              loading={q.isPending}
              columns={columns}
              dataSource={q.rows}
              pagination={false}
              scroll={{ x: true }}
              onRow={(v) => ({ onClick: () => navigate(`${v.config_id}/v/${v.version}`), style: { cursor: 'pointer' } })}
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
          )}
        </>
      )}
    </Flex>
  );
}
