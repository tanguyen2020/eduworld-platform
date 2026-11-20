// RegradeEntryButton (F058 T049, F058-AC-22, AC-24): "Chấm lại hệ thống" on the page of a published question or
// activity version — only with learning_content.re_evaluate (school_admin, tenant_content_editor never see it), inside
// the ActionGate of the catalog; with the history of requests of the version. Display only: the backend authorizes.
import { Button, Card, List, Tag, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import type { ItemRef } from '../release/useRelease';
import { useContentAccess } from '../shared/access';
import { RegradeDialog } from './RegradeDialog';
import { useCorrectedVersions, useRegradeHistory, type RegradeScope } from './useRegrade';

/** item = the question of a question version: its other published versions are the choices of use_new_key. */
export function RegradeEntryButton({ scope, versionId, item }: { scope: RegradeScope; versionId?: string; item?: ItemRef }) {
  const { t } = useTranslation();
  const access = useContentAccess();
  const versions = useCorrectedVersions(access.activity.reEvaluate ? item : undefined);
  const correctedOptions = (versions.data?.data ?? [])
    .filter((v) => v.status === 'published' && !!v.content_version_id && v.content_version_id !== versionId)
    .map((v) => ({ value: v.content_version_id!, label: `v${v.version}` }));
  const [open, setOpen] = useState(false);
  const history = useRegradeHistory(scope, access.activity.reEvaluate ? versionId : undefined);
  if (!access.activity.reEvaluate || !versionId) return null;
  return (
    <Card size="small" title={t('content_studio.regrade.title')} data-testid="regrade-entry">
      <ActionGate actionKey="action.content_studio.regrade.request">
        <Button onClick={() => setOpen(true)}>{t('content_studio.regrade.open')}</Button>
      </ActionGate>
      <List
        size="small"
        data-testid="regrade-history"
        locale={{ emptyText: t('content_studio.regrade.noHistory') }}
        dataSource={history.data?.data ?? []}
        renderItem={(r) => (
          <List.Item>
            <Typography.Text>{new Date(r.created_at).toLocaleString()}</Typography.Text>
            <Tag>{t(`content_studio.regrade.statuses.${r.status}`)}</Tag>
            <Typography.Text type="secondary">{t('content_studio.regrade.counts', { done: r.re_evaluated_attempts ?? 0, total: r.affected_attempts ?? 0 })}</Typography.Text>
          </List.Item>
        )}
      />
      {open && (
        <RegradeDialog
          open={open}
          scope={scope}
          versionId={versionId}
          correctedOptions={correctedOptions}
          onClose={() => {
            setOpen(false);
            void history.refetch();
          }}
        />
      )}
    </Card>
  );
}
