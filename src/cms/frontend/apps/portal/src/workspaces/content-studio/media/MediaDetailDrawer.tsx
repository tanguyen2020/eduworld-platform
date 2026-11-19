// MediaDetailDrawer (F054-AC-16, AC-18, ADR-0044): one asset (getMediaAsset) — pipeline status with the rejection
// reason, variants, license, alt text, private / exam storage, and "đang dùng ở version nào" (`used_in`). There is no
// way to overwrite the file: replacing an image = upload a new asset and choose it in a new draft (immutable keys).
import { Alert, Descriptions, Drawer, Empty, List, Skeleton, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { formatIct } from '../release/releaseOptions';
import { formatMb } from './mediaRules';
import { useMediaAsset } from './useMediaUpload';

export function MediaDetailDrawer({ assetId, onClose }: { assetId: string | null; onClose: () => void }) {
  const { t } = useTranslation();
  const q = useMediaAsset(assetId ?? undefined);
  const a = q.data;
  return (
    <Drawer open={assetId !== null} onClose={onClose} width="min(720px, 100vw)" title={a?.original_filename ?? t('content_studio.media.detail')} destroyOnHidden>
      {q.isPending ? (
        <Skeleton active />
      ) : q.error ? (
        <ProblemMessage error={q.error} />
      ) : a ? (
        <div data-testid="media-detail">
          {a.status === 'rejected' && <Alert type="error" showIcon message={t('content_studio.media.rejectedReason', { reason: a.reject_reason ?? '—' })} />}
          <Descriptions size="small" column={1} bordered>
            <Descriptions.Item label={t('content_studio.media.col.status')}>
              <Tag data-media-status={a.status}>{t(`content_studio.media.status.${a.status}`)}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label={t('content_studio.media.col.kind')}>{t(`content_studio.media.kind.${a.kind}`)}</Descriptions.Item>
            <Descriptions.Item label={t('content_studio.media.col.size')}>{a.size_bytes ? formatMb(a.size_bytes) : '—'}</Descriptions.Item>
            <Descriptions.Item label={t('content_studio.media.license')}>{a.license ?? '—'}</Descriptions.Item>
            <Descriptions.Item label={t('content_studio.media.altText')}>{a.alt_text ?? '—'}</Descriptions.Item>
            <Descriptions.Item label={t('content_studio.media.col.storage')}>{a.is_exam_material || a.bucket === 'ew-media-private' ? t('content_studio.media.private') : t('content_studio.media.public')}</Descriptions.Item>
            <Descriptions.Item label={t('content_studio.media.col.created')}>{formatIct(a.created_at) || '—'}</Descriptions.Item>
          </Descriptions>
          <Typography.Title level={4}>{t('content_studio.media.variants')}</Typography.Title>
          {(a.variants ?? []).length === 0 ? (
            <Typography.Text type="secondary">{t('content_studio.media.noVariants')}</Typography.Text>
          ) : (
            <List size="small" dataSource={a.variants} renderItem={(v) => <List.Item key={v.variant}>{`${v.variant} · ${v.mime_type ?? ''} · ${v.size_bytes ? formatMb(v.size_bytes) : ''}`}</List.Item>} />
          )}
          <Typography.Title level={4}>{t('content_studio.media.usedIn')}</Typography.Title>
          {(a.used_in ?? []).length === 0 ? (
            <Empty description={t('content_studio.media.notUsed')} />
          ) : (
            <List
              size="small"
              data-testid="media-used-in"
              dataSource={a.used_in}
              renderItem={(u) => (
                <List.Item key={`${u.item_type}:${u.item_id}:${u.version}`}>
                  {t('content_studio.media.usedInRow', { type: t(`content_studio.contentType.${u.item_type}`, { defaultValue: u.item_type }), id: u.item_id, version: u.version })}
                </List.Item>
              )}
            />
          )}
          <Alert type="info" showIcon message={t('content_studio.media.replaceHint')} />
        </div>
      ) : null}
    </Drawer>
  );
}
