// UploadQueue (F082 T012, F082-AC-03): the files of a useChecksumUpload queue — name, per-file progress, the server error
// of THAT file (message + code + trace id) with "Thử lại" / "Bỏ". Presentational: no API call (ADMIN_PORTAL §1.1).
import { Alert, Button, Flex, List, Progress, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';
import type { UploadItem } from './useChecksumUpload';

export interface UploadQueueProps {
  items: readonly UploadItem[];
  onRetry: (id: string) => void;
  onRemove: (id: string) => void;
}

export function UploadQueue({ items, onRetry, onRemove }: UploadQueueProps) {
  const { t } = useTranslation(OPS_UI_NS);
  if (items.length === 0) return null;
  return (
    <List
      size="small"
      aria-label={t('upload.queue')}
      dataSource={[...items]}
      renderItem={(it) => (
        <List.Item key={it.id} data-testid="upload-item" data-status={it.status}>
          <Flex vertical gap="small" flex={1}>
            <Flex justify="space-between" align="center" gap="small" wrap>
              <Typography.Text ellipsis>{it.name}</Typography.Text>
              <Typography.Text type={it.status === 'error' ? 'danger' : 'secondary'}>{t(`upload.status.${it.status}`)}</Typography.Text>
            </Flex>
            <Progress
              percent={it.progress}
              size="small"
              status={it.status === 'error' ? 'exception' : it.status === 'done' ? 'success' : 'active'}
              aria-label={t('upload.progress', { name: it.name })}
            />
            {it.error && (
              <Alert
                type="error"
                showIcon
                message={it.error.message}
                description={[it.error.code, it.error.traceId ? t('upload.traceId', { traceId: it.error.traceId }) : null].filter(Boolean).join(' · ') || undefined}
                action={
                  <Flex gap="small">
                    <Button size="small" aria-label={t('upload.retryFile', { name: it.name })} onClick={() => onRetry(it.id)}>
                      {t('upload.retry')}
                    </Button>
                    <Button size="small" aria-label={t('upload.removeFile', { name: it.name })} onClick={() => onRemove(it.id)}>
                      {t('upload.remove')}
                    </Button>
                  </Flex>
                }
              />
            )}
          </Flex>
        </List.Item>
      )}
    />
  );
}
