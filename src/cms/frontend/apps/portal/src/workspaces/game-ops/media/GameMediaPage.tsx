// GameMediaPage (F079 `menu.game_ops.media`, ADR-0044, F078 media): upload a game content file — sha256 in the browser →
// presignGameMediaUpload (Idempotency-Key; license required, alt text for images) → PUT to the presigned URL → the
// pipeline checks it (size, magic bytes, scan) and the asset is polled with getGameMediaAsset until `ready` / `rejected`.
// Payloads reference it by `*_asset_id` / `asset:<uuid>`. Look up an asset by id. No media list exists in the contract.
import { SearchSelect } from '@eduworld/ops-ui';
import { Alert, Button, Card, Descriptions, Flex, Form, Input, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { sha256OfFile } from '../../../shared/files/sha256';
import { useMediaAsset, useMediaPresign, type MediaPresignRequest } from '../api/console';
import { GameOpsError } from '../shared/errors';

/** MediaPresignRequest limits (OpenAPI). */
const LIMITS = { license: 255, altText: 500, filename: 255, mime: 127, assetId: 36 };
const KINDS: MediaPresignRequest['kind'][] = ['image', 'audio', 'video', 'model3d'];
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function GameMediaPage({ title }: { title: string }) {
  const { t } = useTranslation();
  const presign = useMediaPresign();
  const [file, setFile] = useState<File | null>(null);
  const [kind, setKind] = useState<MediaPresignRequest['kind']>('image');
  const [license, setLicense] = useState('');
  const [alt, setAlt] = useState('');
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [assetId, setAssetId] = useState<string | undefined>();
  const [lookup, setLookup] = useState('');
  const asset = useMediaAsset(assetId);

  const upload = async () => {
    if (!file) return;
    setUploadError(null);
    const sha256 = await sha256OfFile(file);
    presign.mutate(
      {
        kind,
        filename: file.name.slice(0, LIMITS.filename),
        mime_type: (file.type || 'application/octet-stream').slice(0, LIMITS.mime),
        size_bytes: file.size,
        sha256,
        license: license.trim(),
        ...(alt.trim() ? { alt_text: alt.trim() } : {}),
        is_exam_material: false,
        owner_scope: 'platform',
      },
      {
        onSuccess: async (r) => {
          if (r.upload_url && !r.duplicate_of) {
            const put = await fetch(r.upload_url, { method: 'PUT', body: file, headers: r.upload_headers ?? {} });
            if (!put.ok) return setUploadError(t('gameOps.media.putFailed', { status: put.status }));
          }
          setAssetId(r.asset_id);
        },
      },
    );
  };

  return (
    <Flex vertical gap="middle" data-testid="game-media">
      <Typography.Title level={1}>{title}</Typography.Title>
      <ActionGate actionKey="action.game_ops.media.upload">
        <Card size="small" title={t('gameOps.media.upload')}>
          <Form layout="vertical" component="div">
            <Form.Item label={t('gameOps.media.file')} htmlFor="game-media-file">
              <input id="game-media-file" type="file" onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
            </Form.Item>
            <Form.Item label={t('gameOps.media.kind')} htmlFor="game-media-kind">
              <SearchSelect id="game-media-kind" aria-label={t('gameOps.media.kind')} value={kind} onChange={(v) => setKind(v as MediaPresignRequest['kind'])} options={KINDS.map((k) => ({ value: k, label: t(`gameOps.media.kinds.${k}`) }))} />
            </Form.Item>
            <Form.Item label={t('gameOps.media.license')} htmlFor="game-media-license" required>
              <Input id="game-media-license" maxLength={LIMITS.license} showCount value={license} onChange={(e) => setLicense(e.target.value)} />
            </Form.Item>
            {kind === 'image' && (
              <Form.Item label={t('gameOps.media.altText')} htmlFor="game-media-alt" required>
                <Input id="game-media-alt" maxLength={LIMITS.altText} showCount value={alt} onChange={(e) => setAlt(e.target.value)} />
              </Form.Item>
            )}
            <Button type="primary" disabled={!file || !license.trim() || (kind === 'image' && !alt.trim())} loading={presign.isPending} onClick={() => void upload()}>
              {t('gameOps.media.send')}
            </Button>
          </Form>
          {presign.error ? <GameOpsError error={presign.error} /> : null}
          {uploadError && <Alert type="error" showIcon message={uploadError} />}
        </Card>
      </ActionGate>
      <Card size="small" title={t('gameOps.media.lookup')}>
        <Flex gap="small">
          <Input aria-label={t('gameOps.media.assetId')} maxLength={LIMITS.assetId} value={lookup} onChange={(e) => setLookup(e.target.value.trim())} />
          <Button disabled={!UUID.test(lookup)} onClick={() => setAssetId(lookup)}>
            {t('gameOps.media.find')}
          </Button>
        </Flex>
      </Card>
      {asset.error ? <GameOpsError error={asset.error} /> : null}
      {asset.data && (
        <Descriptions bordered size="small" column={1} data-testid="game-media-asset">
          <Descriptions.Item label={t('gameOps.media.assetId')}>{asset.data.asset_id}</Descriptions.Item>
          <Descriptions.Item label={t('gameOps.media.status')}>{t(`gameOps.media.statuses.${asset.data.status}`, { defaultValue: asset.data.status })}</Descriptions.Item>
          <Descriptions.Item label={t('gameOps.media.file')}>{asset.data.original_filename}</Descriptions.Item>
          {asset.data.reject_reason && <Descriptions.Item label={t('gameOps.media.rejectReason')}>{asset.data.reject_reason}</Descriptions.Item>}
        </Descriptions>
      )}
    </Flex>
  );
}
