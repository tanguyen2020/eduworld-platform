// ReviewQueuePage (F054-AC-12, AC-14, AC-15, AC-30, ADMIN_PORTAL §4): "Duyệt nội dung" — open review tasks by lane,
// content type and "giao cho tôi", with the submission time (ICT) and the overdue warning (`content_review_sla_days`);
// selection → bulk approval (≤ `content_review_bulk_max`). A tenant reviewer sees exactly what the API returns for
// the session tenant (no `tenant_ids`, no platform lane option); the System Admin filters tenants in the topbar.
// Cards below 768 px (reading, opening a task and deciding work on every breakpoint).
import { Button, Flex, Switch, Tag, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Route, Routes, useNavigate, useParams } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ResponsiveTable } from '../../../shell/ResponsiveTable';
import { formatIct } from '../release/releaseOptions';
import { useContentAccess } from '../shared/access';
import { ALL_CONTENT_TYPES, type ContentType } from '../shared/contentTypes';
import { BulkApproveDialog } from './BulkApproveDialog';
import { dueAt, isOverdue, taskKey, type ReviewTask } from './reviewRules';
import { ReviewTaskPage } from './ReviewTaskPage';
import { useReviewTasks, type ReviewFilters } from './useReviewTasks';
import { SearchSelect } from '@eduworld/ops-ui';

type Row = ReviewTask & { key: string };

function ReviewQueue({ title }: { title: string }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { platform } = useContentAccess();
  const [f, setF] = useState<ReviewFilters>({});
  const q = useReviewTasks(f);
  const [selected, setSelected] = useState<string[]>([]);
  // the tasks of a bulk run are frozen when the dialog opens (the queue refreshes while items get approved)
  const [bulk, setBulk] = useState<ReviewTask[] | null>(null);
  const rows = (q.data?.pages ?? []).flatMap((p) => p.data);
  const now = new Date();
  const open = (x: ReviewTask) => navigate(`${x.content_type}/${x.content_id}/${x.version}`);

  const columns: ColumnType<Row>[] = [
    {
      key: 'title',
      title: t('content_studio.review.col.title'),
      render: (_, x) => (
        <Button type="link" onClick={() => open(x)}>
          {x.title ?? x.content_id}
        </Button>
      ),
    },
    { key: 'type', title: t('content_studio.review.col.type'), render: (_, x) => t(`content_studio.contentType.${x.content_type}`) },
    { key: 'version', title: t('content_studio.release.col.version'), render: (_, x) => t('content_studio.release.v', { version: x.version }) },
    { key: 'lane', title: t('content_studio.review.col.lane'), render: (_, x) => t(`content_studio.review.lane.${x.lane}`) },
    { key: 'submitted', title: t('content_studio.review.col.submitted'), render: (_, x) => formatIct(x.submitted_at) },
    {
      key: 'due',
      title: t('content_studio.review.col.due'),
      render: (_, x) => (
        <span>
          {formatIct(dueAt(x))} {isOverdue(x, now) ? <Tag color="error">{t('content_studio.review.overdue')}</Tag> : null}
        </span>
      ),
    },
  ];

  return (
    <section aria-labelledby="ew-cs-reviews">
      <Flex vertical gap="middle">
        <Typography.Title level={1} id="ew-cs-reviews">
          {title}
        </Typography.Title>
        <Flex gap="small" wrap align="center">
          <SearchSelect
            allowClear
            aria-label={t('content_studio.review.col.lane')}
            placeholder={t('content_studio.review.col.lane')}
            value={f.lane}
            onChange={(lane) => setF({ ...f, lane })}
            options={(platform ? (['tenant', 'platform_learning'] as const) : (['tenant'] as const)).map((l) => ({ value: l, label: t(`content_studio.review.lane.${l}`) }))}
          />
          <SearchSelect
            allowClear
            aria-label={t('content_studio.review.col.type')}
            placeholder={t('content_studio.review.col.type')}
            value={f.contentType}
            onChange={(contentType?: ContentType) => setF({ ...f, contentType })}
            options={ALL_CONTENT_TYPES.map((c) => ({ value: c, label: t(`content_studio.contentType.${c}`) }))}
          />
          <Flex gap="small" align="center">
            <Switch aria-label={t('content_studio.review.mine')} checked={!!f.mine} onChange={(mine) => setF({ ...f, mine })} />
            <Typography.Text>{t('content_studio.review.mine')}</Typography.Text>
          </Flex>
          <ActionGate actionKey="action.content_studio.reviews.view">
            <Button disabled={selected.length === 0} onClick={() => setBulk(rows.filter((x) => selected.includes(taskKey(x))))}>
              {t('content_studio.review.bulk', { count: selected.length })}
            </Button>
          </ActionGate>
        </Flex>
        {q.error ? <ProblemMessage error={q.error} /> : null}
        <ResponsiveTable<Row>
          rowKey="key"
          columns={columns}
          data={rows.map((x) => ({ ...x, key: taskKey(x) }))}
          loading={q.isFetching}
          cardTitle={(x) => (
            <Button type="link" onClick={() => open(x)}>
              {x.title ?? x.content_id}
            </Button>
          )}
          hasMore={q.hasNextPage}
          onLoadMore={() => void q.fetchNextPage()}
          rowSelection={{ selectedRowKeys: selected, onChange: (k) => setSelected(k as string[]) }}
          loadMoreLabel={t('content_studio.loadMore')}
        />
      </Flex>
      <BulkApproveDialog
        open={bulk !== null}
        tasks={bulk ?? []}
        onClose={() => {
          setBulk(null);
          setSelected([]);
        }}
      />
    </section>
  );
}

function TaskRoute() {
  const { contentType, contentId, version } = useParams();
  return <ReviewTaskPage item={{ contentType: contentType as ContentType, contentId: contentId! }} version={Number(version)} />;
}

export function ReviewsScreen({ title }: { title: string }) {
  return (
    <Routes>
      <Route index element={<ReviewQueue title={title} />} />
      <Route path=":contentType/:contentId/:version" element={<TaskRoute />} />
    </Routes>
  );
}
