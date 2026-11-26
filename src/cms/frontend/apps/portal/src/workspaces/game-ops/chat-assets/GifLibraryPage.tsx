// GifLibraryPage (F082 T016, F082-AC-05, AC-06; CHAT_SYSTEM §7.5, ADR-0034): the internal GIF library of module chat — a grid
// of static previews (signed URLs of module chat, "Phát GIF" plays the animation), filters sent to the server (tag, status,
// uploader: everyone / me), cursor pages ("Tải thêm" — paged in the database, never sliced here), "Tải GIF lên" (the
// upload dialog) inside ActionGate, each GIF opens in the shared editor (review / publish workflow). No external URL field,
// no third-party GIF search.
import { DomainTag, SearchSelect } from '@eduworld/ops-ui';
import { Button, Card, Empty, Flex, Input, Radio, Skeleton, Tabs, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { useMe } from '../../../shared/api/hooks/useMe';
import { useConfigPages, type ContentStatus, type GameConfigVersion } from '../api/gameConfigs';
import { actionOf, CONFIG_TYPES, typesOfMenu } from '../registry/configTypes';
import { GameOpsError } from '../shared/errors';
import { GifTile } from './GifTile';
import { tagRules, type GifPayload } from './gif';
import { GifUploadDialog } from './GifUploadDialog';

const STATUSES: ContentStatus[] = ['draft', 'in_review', 'approved', 'published', 'archived', 'rejected'];

export function GifLibraryPage({ title }: { title: string }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const me = useMe();
  const def = CONFIG_TYPES.gifs;
  const [params, setParams] = useSearchParams();
  const [uploading, setUploading] = useState(false);
  const status = (params.get('status') ?? undefined) as ContentStatus | undefined;
  const tag = params.get('tag') ?? '';
  const mine = params.get('uploader') === 'me';
  const q = useConfigPages('gifs', { status, tag: tag.trim() || undefined, author_id: mine ? me.data?.user_id : undefined }, !mine || !!me.data);
  const setParam = (k: string, v: string | undefined) => {
    const next = new URLSearchParams(params);
    if (v) next.set(k, v);
    else next.delete(k);
    setParams(next, { replace: true });
  };

  return (
    <Flex vertical gap="middle" data-testid="gif-library">
      <Typography.Title level={1}>{title}</Typography.Title>
      <Tabs activeKey="gifs" onChange={(k) => navigate(`../${k}`, { relative: 'path' })} items={typesOfMenu('chat_assets').map((d) => ({ key: d.type, label: t(`gameOps.type.${d.type}`) }))} />
      <Flex gap="small" wrap align="center" justify="space-between">
        <Flex gap="small" wrap align="center">
          <Input aria-label={t('gameOps.chatAssets.gif.tagFilter')} placeholder={t('gameOps.chatAssets.gif.tagFilter')} allowClear maxLength={tagRules(def.schema).maxLength} value={tag} onChange={(e) => setParam('tag', e.target.value)} />
          <SearchSelect
            allowClear
            aria-label={t('gameOps.list.status')}
            placeholder={t('gameOps.list.statusFilter')}
            value={status}
            onChange={(v) => setParam('status', v ? String(v) : undefined)}
            options={STATUSES.map((s) => ({ value: s, label: t(`gameOps.status.${s}`) }))}
          />
          <Radio.Group
            aria-label={t('gameOps.chatAssets.gif.uploader')}
            optionType="button"
            value={mine ? 'me' : 'all'}
            onChange={(e) => setParam('uploader', e.target.value === 'me' ? 'me' : undefined)}
            options={[
              { value: 'all', label: t('gameOps.chatAssets.gif.uploaderAll') },
              { value: 'me', label: t('gameOps.chatAssets.gif.uploaderMe') },
            ]}
          />
        </Flex>
        <ActionGate actionKey={actionOf(def, 'edit')}>
          <Button type="primary" onClick={() => setUploading(true)}>
            {t('gameOps.chatAssets.gif.upload')}
          </Button>
        </ActionGate>
      </Flex>
      {q.error ? (
        <GameOpsError error={q.error} />
      ) : q.isPending ? (
        <Skeleton active />
      ) : q.rows.length === 0 ? (
        <Empty description={t('gameOps.chatAssets.gif.empty')} />
      ) : (
        <Flex gap="middle" wrap data-testid="gif-grid">
          {q.rows.map((v: GameConfigVersion) => (
            <Card key={v.config_id} size="small">
              <Flex vertical gap="small">
                <GifTile gif={v.payload as GifPayload} />
                <Flex gap="small" align="center" wrap>
                  <DomainTag kind="content_status" value={v.status} />
                  <Typography.Text>{t('gameOps.common.v', { version: v.version })}</Typography.Text>
                  <Button size="small" type="link" onClick={() => navigate(`${v.config_id}/v/${v.version}`)}>
                    {t('gameOps.versions.open')}
                  </Button>
                </Flex>
              </Flex>
            </Card>
          ))}
        </Flex>
      )}
      {q.hasNextPage && (
        <Button onClick={() => void q.fetchNextPage()} loading={q.isFetchingNextPage}>
          {t('gameOps.common.loadMore')}
        </Button>
      )}
      <GifUploadDialog open={uploading} onClose={() => setUploading(false)} />
    </Flex>
  );
}
