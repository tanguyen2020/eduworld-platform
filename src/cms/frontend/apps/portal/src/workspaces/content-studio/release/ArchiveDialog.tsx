// ArchiveDialog (F054-AC-08, CONTENT_MODEL §6): archives a version that is not the current pointer —
// transitionContentVersion `archive` (If-Match, Idempotency-Key) with the reason as note; where-used list shown.
import { App as AntApp } from 'antd';
import { useTranslation } from 'react-i18next';
import { LIMITS } from '../shared/config';
import { ReleaseReasonDialog } from './ReleaseReasonDialog';
import { useRelease, type ItemRef } from './useRelease';

export function ArchiveDialog({ item, version, onClose }: { item: ItemRef; version: number | null; onClose: () => void }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const { archive } = useRelease();
  return (
    <ReleaseReasonDialog
      open={version !== null}
      title={t('content_studio.release.archiveTitle', { version })}
      okText={t('content_studio.release.archive')}
      description={t('content_studio.release.archiveImpact')}
      item={item}
      maxLength={LIMITS.releaseReason}
      loading={archive.isPending}
      error={archive.error}
      onCancel={() => {
        archive.reset();
        onClose();
      }}
      onSubmit={(reason) =>
        version !== null &&
        archive.mutate(
          { ...item, version, reason },
          {
            onSuccess: () => {
              message.success(t('content_studio.release.archived', { version }));
              onClose();
            },
          },
        )
      }
    />
  );
}
