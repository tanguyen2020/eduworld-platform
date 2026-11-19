// UnpublishDialog (F054-AC-07, AC-09, AC-19, ADR-0043): removes the release pointer (sessions already started keep
// their pinned version) — reason required, where-used warning, unpublishContent with one Idempotency-Key per intent.
import { App as AntApp } from 'antd';
import { useTranslation } from 'react-i18next';
import { LIMITS } from '../shared/config';
import { ReleaseReasonDialog } from './ReleaseReasonDialog';
import { useRelease, type ItemRef } from './useRelease';

export function UnpublishDialog({ item, open, onClose }: { item: ItemRef; open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const { unpublish } = useRelease();
  return (
    <ReleaseReasonDialog
      open={open}
      title={t('content_studio.release.unpublishTitle')}
      okText={t('content_studio.release.unpublish')}
      description={t('content_studio.release.unpublishImpact')}
      item={item}
      maxLength={LIMITS.releaseReason}
      loading={unpublish.isPending}
      error={unpublish.error}
      onCancel={() => {
        unpublish.reset();
        onClose();
      }}
      onSubmit={(reason) =>
        unpublish.mutate(
          { ...item, reason },
          {
            onSuccess: () => {
              message.success(t('content_studio.release.unpublished'));
              onClose();
            },
          },
        )
      }
    />
  );
}
