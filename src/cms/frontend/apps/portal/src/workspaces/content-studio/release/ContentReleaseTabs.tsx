// ContentReleaseTabs (F054 US1, ADMIN_PORTAL §1.3 "chi tiết = Tabs Phiên bản/Phát hành/Sử dụng"): the release
// part of any versioned learning item, attached under each editor (question, question set, lesson, blueprint) and
// opened by id for learning paths — Phiên bản (VersionHistoryTab), Phát hành (ReleaseTimeline), Sử dụng (UsageTab),
// Xem trước (PreviewPanel of a chosen version).
import { InputNumber, Flex, Tabs, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { PreviewPanel } from '../preview/PreviewPanel';
import { useContentAccess } from '../shared/access';
import { UsageTab } from '../usage/UsageTab';
import { ReleaseTimeline } from './ReleaseTimeline';
import { VersionHistoryTab } from './VersionHistoryTab';
import type { ItemRef } from './useRelease';

export interface ContentReleaseTabsProps {
  item: ItemRef;
  /** Version shown by default in "Xem trước". */
  version: number;
  assigned?: boolean;
  contentLanguage?: 'vi' | 'en';
  onDraftCreated?: (version: number) => void;
}

export function ContentReleaseTabs({ item, version, assigned, contentLanguage = 'vi', onDraftCreated }: ContentReleaseTabsProps) {
  const { t } = useTranslation();
  const access = useContentAccess();
  const [previewVersion, setPreviewVersion] = useState(version);
  return (
    <Tabs
      data-testid="release-tabs"
      destroyOnHidden
      items={[
        { key: 'versions', label: t('content_studio.release.tabs.versions'), children: <VersionHistoryTab item={item} assigned={assigned} onDraftCreated={onDraftCreated} /> },
        { key: 'releases', label: t('content_studio.release.tabs.releases'), children: <ReleaseTimeline item={item} canPublish={access.question.publish && !assigned} /> },
        { key: 'usages', label: t('content_studio.release.tabs.usages'), children: <UsageTab item={item} /> },
        {
          key: 'preview',
          label: t('content_studio.release.tabs.preview'),
          children: (
            <Flex vertical gap="small">
              <Flex gap="small" align="center">
                <Typography.Text>{t('content_studio.release.previewVersion')}</Typography.Text>
                <InputNumber aria-label={t('content_studio.release.previewVersion')} min={1} max={version} value={previewVersion} onChange={(v) => v && setPreviewVersion(v)} />
              </Flex>
              <PreviewPanel key={previewVersion} item={item} version={previewVersion} contentLanguage={contentLanguage} />
            </Flex>
          ),
        },
      ]}
    />
  );
}
