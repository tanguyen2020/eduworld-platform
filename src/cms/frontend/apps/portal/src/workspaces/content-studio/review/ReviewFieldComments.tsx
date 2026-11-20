// ReviewFieldComments (F054-AC-13, OQ F054-1, F025 T046): the per-field comments of the latest rejection of one version
// (`ContentVersionSummary.field_comments` from listContentVersions — the same cached query as the version history),
// shown to the author in the editor while the version is rejected or back in draft, and to the reviewer of a
// resubmitted version. `path` is a JSON pointer (empty = the whole version). Display only; nothing is decided here.
import { Alert, List, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { useVersions, type ItemRef } from '../release/useRelease';

export function ReviewFieldComments({ item, version }: { item: ItemRef; version: number }) {
  const { t } = useTranslation();
  const versions = useVersions(item);
  const comments = versions.data?.data.find((v) => v.version === version)?.field_comments ?? [];
  if (comments.length === 0) return null;
  return (
    <Alert
      type="warning"
      showIcon
      data-testid="review-field-comments"
      message={t('content_studio.review.fieldComments.title', { version })}
      description={
        <List
          size="small"
          dataSource={comments}
          renderItem={(c, i) => (
            <List.Item key={`${c.path}:${i}`}>
              <Typography.Text>
                <Typography.Text code>{c.path || t('content_studio.review.fieldComments.whole')}</Typography.Text> {c.comment}
              </Typography.Text>
            </List.Item>
          )}
        />
      }
    />
  );
}
