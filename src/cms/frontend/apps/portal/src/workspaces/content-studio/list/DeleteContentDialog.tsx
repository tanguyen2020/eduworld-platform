// DeleteContentDialog (F054-AC-25, AC-19, FR-CMS-014, ADR-0028, AGENTS R21): "Xóa" = soft delete into the Thùng rác
// (deleteQuestion / deleteCms*, If-Match, reason); the reason is required when the item was ever published; where it
// is used is listed first. The row then disappears from every normal list. There is no "xóa vĩnh viễn".
import { App as AntApp } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ReleaseReasonDialog } from '../release/ReleaseReasonDialog';
import { LIMITS } from '../shared/config';
import { useDeleteContent, type ContentRow, type ListType } from './useContentList';

export function DeleteContentDialog({ type, row, onClose }: { type: ListType; row: ContentRow | null; onClose: () => void }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const remove = useDeleteContent(type);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const published = !!row && (row.status === 'published' || !!row.published_at);
  const close = () => {
    setError(null);
    onClose();
  };
  return (
    <ReleaseReasonDialog
      open={row !== null}
      title={t('content_studio.list.deleteTitle')}
      okText={t('content_studio.list.delete')}
      description={published ? t('content_studio.list.deletePublished') : t('content_studio.list.deleteDraft')}
      item={row ? { contentType: type, contentId: row.content_id } : undefined}
      maxLength={LIMITS.releaseReason}
      required={published}
      loading={busy}
      error={error}
      onCancel={close}
      onSubmit={async (reason) => {
        if (!row) return;
        setBusy(true);
        setError(null);
        try {
          await remove(row, reason);
          message.success(t('content_studio.list.deleted'));
          close();
        } catch (e) {
          setError(e);
        } finally {
          setBusy(false);
        }
      }}
    />
  );
}
