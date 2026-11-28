// VersionViewDrawer (F054-AC-02, AC-09, FR-CNT-010): one version read only (getContentVersion) — metadata, the
// student preview of that version, and for a published / archived version only "Tạo bản nháp từ phiên bản này"
// (create*Version, Idempotency-Key); for a question version also "Chấm lại hệ thống" of that version (F058-AC-22, found
// by the E2E T047: the editor shows the latest version only). There is no edit or delete action for a published version
// (ADR-0043, R11).
import { Alert, App as AntApp, Button, Descriptions, Drawer, Flex, Skeleton, Typography } from 'antd';
import { DomainTag } from '@eduworld/ops-ui';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { PreviewPanel } from '../preview/PreviewPanel';
import { RegradeEntryButton } from '../regrade/RegradeEntryButton';
import { CONTENT_TYPES } from '../shared/contentTypes';
import { formatIct, isReadOnlyVersion } from './releaseOptions';
import { canCreateVersion, useRelease, useVersionDetail, type ItemRef } from './useRelease';

export interface VersionViewDrawerProps {
  item: ItemRef;
  version: number | null;
  canWrite: boolean;
  /** Assigned platform content (tenant user): no new draft. */
  assigned?: boolean;
  onClose: () => void;
  onDraftCreated?: (version: number) => void;
}

export function VersionViewDrawer({ item, version, canWrite, assigned, onClose, onDraftCreated }: VersionViewDrawerProps) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const detail = useVersionDetail(item, version ?? undefined);
  const { newDraft } = useRelease();
  const v = detail.data?.data;
  const editKey = CONTENT_TYPES[item.contentType].actions.edit;
  const lang = v?.content_language === 'en' ? 'en' : 'vi';
  return (
    <Drawer open={version !== null} onClose={onClose} width="min(960px, 100vw)" title={t('content_studio.release.viewTitle', { version })} destroyOnHidden>
      {detail.isPending ? (
        <Skeleton active />
      ) : detail.error ? (
        <ProblemMessage error={detail.error} />
      ) : v ? (
        <Flex vertical gap="middle" data-testid="version-view">
          {isReadOnlyVersion(v.status) && <Alert type="info" showIcon message={t('content_studio.release.readOnly', { version: v.version })} />}
          <Descriptions size="small" column={{ xs: 1, md: 2 }} bordered>
            <Descriptions.Item label={t('content_studio.release.col.status')}>
              <DomainTag kind="content_status" value={v.status} />
            </Descriptions.Item>
            <Descriptions.Item label={t('content_studio.release.col.author')}>{v.author_id ?? '—'}</Descriptions.Item>
            <Descriptions.Item label={t('content_studio.release.col.reviewer')}>{v.reviewer_id ?? '—'}</Descriptions.Item>
            <Descriptions.Item label={t('content_studio.release.col.publishedAt')}>{formatIct(v.published_at) || '—'}</Descriptions.Item>
            <Descriptions.Item label={t('content_studio.question.contentLanguage')}>{t(`content_studio.preview.${lang}`)}</Descriptions.Item>
            <Descriptions.Item label={t('content_studio.release.col.translation')}>{v.translation_status ? t(`content_studio.translation.${v.translation_status}`) : '—'}</Descriptions.Item>
          </Descriptions>
          {isReadOnlyVersion(v.status) && canWrite && !assigned && canCreateVersion(item.contentType) && editKey && (
            <ActionGate actionKey={editKey} mode="disable">
              <Button
                loading={newDraft.isPending}
                onClick={() =>
                  newDraft.mutate(
                    { ...item, fromVersion: v.version },
                    {
                      onSuccess: (r) => {
                        message.success(t('content_studio.release.draftCreated', { version: r.version }));
                        onDraftCreated?.(r.version);
                      },
                    },
                  )
                }
              >
                {t('content_studio.release.newDraftFrom', { version: v.version })}
              </Button>
            </ActionGate>
          )}
          {newDraft.error ? <ProblemMessage error={newDraft.error} /> : null}
          {/* system re-evaluation of THIS version (F058-AC-22): the editor only shows the latest version, so a faulty
              version replaced by a corrected one is re-evaluated from here; hidden without learning_content.re_evaluate */}
          {item.contentType === 'question' && isReadOnlyVersion(v.status) && v.content_version_id ? <RegradeEntryButton scope="question_version" versionId={v.content_version_id} item={item} /> : null}
          <Typography.Title level={3}>{t('content_studio.preview.title')}</Typography.Title>
          <PreviewPanel item={item} version={v.version} contentLanguage={lang} />
        </Flex>
      ) : null}
    </Drawer>
  );
}
