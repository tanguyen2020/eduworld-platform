// ConflictBanner (F053-AC-07, TC-CNT-003): someone else saved this draft (`412` / `409 CONTENT_VERSION_CONFLICT`).
// Autosave is stopped, the author's changes stay on screen; "Tải bản mới" replaces them with the server copy (after
// the author copied what they need). Never overwrites the other person's work.
import { Alert, Button, Space } from 'antd';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';

export interface ConflictBannerProps {
  onReload: () => void;
  /** Copies the local changes (e.g. to the clipboard) before reloading. */
  onCopyMine?: () => void;
}

export function ConflictBanner({ onReload, onCopyMine }: ConflictBannerProps) {
  const { t } = useTranslation(OPS_UI_NS);
  return (
    <Alert
      type="error"
      showIcon
      role="alert"
      message={t('editor.conflict.title')}
      description={t('editor.conflict.description')}
      action={
        <Space direction="vertical">
          <Button size="small" type="primary" onClick={onReload}>
            {t('editor.conflict.reload')}
          </Button>
          {onCopyMine && (
            <Button size="small" onClick={onCopyMine}>
              {t('editor.conflict.copyMine')}
            </Button>
          )}
        </Space>
      }
    />
  );
}
