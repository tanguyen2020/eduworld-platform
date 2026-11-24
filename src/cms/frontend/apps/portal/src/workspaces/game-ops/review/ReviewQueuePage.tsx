// ReviewQueuePage (F079-AC-03, AC-21 `menu.game_ops.review`): versions waiting for review (`status = in_review`) of one
// config type at a time (tabs), cursor pages "Tải thêm"; opening a row goes to its editor where the reviewer approves or
// rejects with a note (never their own version — the server checks reviewer ≠ author).
import { DomainTag } from '@eduworld/ops-ui';
import { Button, Flex, Table, Tabs, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { formatIct } from '../../../shared/release/ict';
import { useConfigPages, type GameConfigVersion } from '../api/gameConfigs';
import { codeOf, CONFIG_TYPES, editorPathOf, nameOf, type ConfigType } from '../registry/configTypes';
import { GameOpsError } from '../shared/errors';


export function ReviewQueuePage({ title }: { title: string }) {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const lang = i18n.language.startsWith('en') ? 'en' : 'vi';
  const served = Object.values(CONFIG_TYPES).filter((d) => d.served);
  const [type, setType] = useState<ConfigType>(served[0]!.type);
  const def = CONFIG_TYPES[type];
  const q = useConfigPages(type, 'in_review');
  return (
    <Flex vertical gap="middle" data-testid="review-queue">
      <Typography.Title level={1}>{title}</Typography.Title>
      <Tabs activeKey={type} onChange={(k) => setType(k as ConfigType)} items={served.map((d) => ({ key: d.type, label: t(`gameOps.type.${d.type}`) }))} />
      {q.error ? (
        <GameOpsError error={q.error} />
      ) : (
        <Table<GameConfigVersion>
          rowKey="config_id"
          loading={q.isPending}
          pagination={false}
          dataSource={q.rows}
          scroll={{ x: true }}
          columns={[
            { key: 'code', title: t('gameOps.list.code'), render: (_, v) => <Typography.Text code>{codeOf(def, v.payload) || v.config_id.slice(-8)}</Typography.Text> },
            { key: 'title', title: t('gameOps.list.title'), render: (_, v) => v.title || nameOf(def, v.payload, lang) || '—' },
            { key: 'status', title: t('gameOps.list.status'), render: (_, v) => <DomainTag kind="content_status" value={v.status} /> },
            { key: 'version', title: t('gameOps.list.version'), render: (_, v) => t('gameOps.common.v', { version: v.version }) },
            { key: 'updated', title: t('gameOps.list.updatedAt'), render: (_, v) => formatIct(v.updated_at) },
            {
              key: 'open',
              title: t('gameOps.common.actions'),
              render: (_, v) => (
                <Button size="small" onClick={() => navigate(editorPathOf(type, v.config_id, v.version))}>
                  {t('gameOps.review.open')}
                </Button>
              ),
            },
          ]}
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
    </Flex>
  );
}
