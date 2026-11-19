// Empty workspace page (F050): translated title only; content arrives with F051–F057, F079, F080.
import { Empty, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import type { WorkspaceId } from './workspaces';

export function WorkspacePlaceholder({ workspace }: { workspace: WorkspaceId }) {
  const { t } = useTranslation();
  return (
    <section aria-labelledby={`ws-${workspace}`}>
      <Typography.Title level={2} id={`ws-${workspace}`}>
        {t(`workspace.${workspace}.title`)}
      </Typography.Title>
      <Empty description={t('workspace.placeholder')} />
    </section>
  );
}
