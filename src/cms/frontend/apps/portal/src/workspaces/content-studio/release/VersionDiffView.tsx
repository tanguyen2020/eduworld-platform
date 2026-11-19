// VersionDiffView (F054-AC-03, AC-30, FR-CNT-010): diffContentVersions of two versions of one item shown by field /
// block in four groups — text, answers, media, metadata. Answer changes are shown only with
// `learning_content.review` (the server omits them otherwise; the client drops them too, AGENTS R6). Needs ≥ 1024 px
// (ADMIN_PORTAL §1.5): smaller screens get "Tác vụ này cần màn hình lớn hơn".
import { Alert, Collapse, Empty, Skeleton, Table, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { LargeScreenRequired } from '../../../shell/StatusPages';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { EDITOR_MIN_WIDTH_QUERY } from '../shared/config';
import { diffValueText, groupDiff, type DiffChange, type DiffGroup } from './releaseOptions';
import { useDiff, type ItemRef, type VersionDiff } from './useRelease';

const GROUPS: DiffGroup[] = ['text', 'answer', 'media', 'metadata'];

export interface VersionDiffViewProps {
  item: ItemRef;
  /** Newer version. */
  version: number;
  /** Version compared with. */
  against: number;
  canReview: boolean;
}

export function VersionDiffView({ item, version, against, canReview }: VersionDiffViewProps) {
  const wide = useMediaQuery(EDITOR_MIN_WIDTH_QUERY);
  const diff = useDiff(item, wide ? version : undefined, against);
  if (!wide) return <LargeScreenRequired />;
  if (diff.isPending) return <Skeleton active />;
  if (diff.error) return <ProblemMessage error={diff.error} />;
  return <DiffGroups diff={diff.data} canReview={canReview} />;
}

/** Presentational part (stories): the changes of a diff by group. */
export function DiffGroups({ diff, canReview }: { diff: VersionDiff; canReview: boolean }) {
  const { t } = useTranslation();
  const groups = groupDiff(diff.changes, canReview);
  const columns = [
    { key: 'path', title: t('content_studio.diff.field'), render: (_: unknown, c: DiffChange) => <Typography.Text code>{c.path}</Typography.Text> },
    { key: 'op', title: t('content_studio.diff.op'), render: (_: unknown, c: DiffChange) => <Tag>{t(`content_studio.diff.ops.${c.op}`)}</Tag> },
    { key: 'before', title: t('content_studio.diff.before', { version: diff.from_version }), render: (_: unknown, c: DiffChange) => <Typography.Text delete={c.op !== 'added'}>{diffValueText(c.before)}</Typography.Text> },
    { key: 'after', title: t('content_studio.diff.after', { version: diff.to_version }), render: (_: unknown, c: DiffChange) => <Typography.Text mark={c.op !== 'removed'}>{diffValueText(c.after)}</Typography.Text> },
  ];
  const total = GROUPS.reduce((n, g) => n + groups[g].length, 0);
  const title = t('content_studio.diff.title', { a: diff.from_version, b: diff.to_version });
  return (
    <section aria-label={title} data-testid="version-diff">
      <Typography.Title level={3}>{title}</Typography.Title>
      {!canReview && <Alert type="info" showIcon message={t('content_studio.diff.answersHidden')} />}
      {total === 0 ? (
        <Empty description={t('content_studio.diff.none')} />
      ) : (
        <Collapse
          defaultActiveKey={GROUPS.filter((g) => groups[g].length > 0)}
          items={GROUPS.filter((g) => g !== 'answer' || canReview).map((g) => ({
            key: g,
            label: t('content_studio.diff.group', { group: t(`content_studio.diff.groups.${g}`), count: groups[g].length }),
            children: groups[g].length ? <Table size="small" rowKey={(c) => `${c.path}:${c.op}`} columns={columns} dataSource={groups[g]} pagination={false} scroll={{ x: true }} /> : <Typography.Text type="secondary">{t('content_studio.diff.noneInGroup')}</Typography.Text>,
          }))}
        />
      )}
    </section>
  );
}
