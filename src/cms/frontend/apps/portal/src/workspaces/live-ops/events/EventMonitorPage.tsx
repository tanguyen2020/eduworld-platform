// "Giám sát sự kiện" (F080 T040, F080-AC-19, ADMIN_PORTAL §3.15): platform event runs (listEventRuns, live + suspended
// by default, status filter, cursor pages with "Tải thêm") and the detail of the selected run — getEventRunStats
// (participants, participation rate, community goal progress, milestone claims, rewards granted, "Cập nhật lúc") and
// the emergency operations (EventRunActions). The selected run is looked up again in the reloaded rows, so a 409 or a
// finished operation shows the current status. Display only (AGENTS R1).
import { SearchSelect } from '@eduworld/ops-ui';
import { Button, Card, Descriptions, Empty, Flex, Progress, Spin, Table, Tag, Typography } from 'antd';
import dayjs from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { DEFAULT_RUN_STATUSES, RUN_STATUSES, useEventRunStats, useEventRuns, type EventRunStatus, type EventRunSummary } from '../api/eventRuns';
import { LiveOpsError, nameIn } from '../shared/ui';
import { EventRunActions } from './EventRunActions';

const STATUS_COLOR: Record<EventRunStatus, string> = { scheduled: 'default', live: 'processing', suspended: 'warning', ended: 'default', cancelled: 'error', archived: 'default' };
const time = (iso: string | undefined | null) => (iso ? dayjs(iso).format('DD/MM/YYYY HH:mm') : '');

function RunStatusTag({ status }: { status: EventRunStatus }) {
  const { t } = useTranslation();
  return (
    <Tag color={STATUS_COLOR[status]} data-run-status={status}>
      {t(`liveOps.events.status.${status}`)}
    </Tag>
  );
}

function RunDetail({ run }: { run: EventRunSummary }) {
  const { t, i18n } = useTranslation();
  const stats = useEventRunStats(run);
  const name = nameIn(run.name, i18n.language) || run.code || run.event_run_id;
  const pct = (v: number | null | undefined) => (v == null ? t('liveOps.dashboard.noValue') : new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 1 }).format(v));
  const n = (v: number | null | undefined) => (v == null ? t('liveOps.dashboard.noValue') : new Intl.NumberFormat(i18n.language).format(v));
  const s = stats.data;
  const goal = s?.community_goal;
  return (
    <Card title={name} data-testid="run-detail" extra={<RunStatusTag status={run.status} />}>
      <Flex vertical gap="middle">
        {run.status === 'suspended' && run.suspended_reason ? <Typography.Text type="secondary">{t('liveOps.events.suspendedReason', { reason: run.suspended_reason })}</Typography.Text> : null}
        <EventRunActions run={run} name={name} />
        {stats.isPending ? <Spin aria-label={t('app.loading')} /> : null}
        {stats.isError ? <LiveOpsError error={stats.error} /> : null}
        {s ? (
          <Descriptions size="small" column={{ xs: 1, md: 2 }} bordered>
            <Descriptions.Item label={t('liveOps.events.stats.participants')}>
              <span data-testid="stat-participants">{n(s.participants)}</span>
            </Descriptions.Item>
            <Descriptions.Item label={t('liveOps.events.stats.participationRate')}>{pct(s.participation_rate)}</Descriptions.Item>
            <Descriptions.Item label={t('liveOps.events.stats.rewardsGranted')}>{n(s.rewards_granted)}</Descriptions.Item>
            <Descriptions.Item label={t('liveOps.events.stats.updatedAt')}>{time(s.updated_at)}</Descriptions.Item>
            {goal ? (
              <Descriptions.Item label={t('liveOps.events.stats.communityGoal')} span={2}>
                <Progress
                  percent={goal.target > 0 ? Math.min(100, Math.floor((goal.progress / goal.target) * 100)) : 0}
                  format={() => t('liveOps.events.stats.goalProgress', { progress: n(goal.progress), target: n(goal.target) })}
                  aria-label={t('liveOps.events.stats.communityGoal')}
                />
              </Descriptions.Item>
            ) : null}
            {s.milestone_claims?.length ? (
              <Descriptions.Item label={t('liveOps.events.stats.claims')} span={2}>
                <ul>
                  {s.milestone_claims.map((c) => (
                    <li key={c.milestone_id}>{t('liveOps.events.stats.claimRow', { milestone: c.milestone_id ?? '', claims: n(c.claims) })}</li>
                  ))}
                </ul>
              </Descriptions.Item>
            ) : null}
          </Descriptions>
        ) : null}
      </Flex>
    </Card>
  );
}

export function EventMonitorPage() {
  const { t, i18n } = useTranslation();
  const [statuses, setStatuses] = useState<EventRunStatus[]>(DEFAULT_RUN_STATUSES);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const runs = useEventRuns(statuses);
  const selected = runs.rows.find((r) => r.event_run_id === selectedId) ?? null;
  return (
    <Flex vertical gap="middle" data-testid="event-monitor">
      <Flex wrap gap="small" align="center">
        <SearchSelect<EventRunStatus[]>
          mode="multiple"
          aria-label={t('liveOps.events.statusFilter')}
          value={statuses}
          onChange={(v) => setStatuses(v.length ? v : DEFAULT_RUN_STATUSES)}
          options={RUN_STATUSES.map((s) => ({ value: s, label: t(`liveOps.events.status.${s}`) }))}
          style={{ minWidth: '16rem' }}
        />
      </Flex>
      {runs.isError ? <LiveOpsError error={runs.error} /> : null}
      <Table<EventRunSummary>
        size="small"
        rowKey="event_run_id"
        loading={runs.isPending}
        pagination={false}
        scroll={{ x: true }}
        dataSource={runs.rows}
        locale={{ emptyText: <Empty description={t('liveOps.events.empty')} /> }}
        onRow={(r) => ({ onClick: () => setSelectedId(r.event_run_id) })}
        rowClassName={(r) => (r.event_run_id === selectedId ? 'ant-table-row-selected' : '')}
        columns={[
          {
            key: 'name',
            title: t('liveOps.events.col.name'),
            render: (_, r) => (
              <Button type="link" onClick={() => setSelectedId(r.event_run_id)}>
                {nameIn(r.name, i18n.language) || r.code || r.event_run_id}
              </Button>
            ),
          },
          { key: 'kind', title: t('liveOps.events.col.kind'), render: (_, r) => t(`liveOps.events.kind.${r.event_kind}`) },
          { key: 'status', title: t('liveOps.events.col.status'), render: (_, r) => <RunStatusTag status={r.status} /> },
          { key: 'starts', title: t('liveOps.events.col.starts'), render: (_, r) => time(r.starts_at) },
          { key: 'ends', title: t('liveOps.events.col.ends'), render: (_, r) => time(r.ends_at) },
        ]}
      />
      {runs.hasNextPage ? (
        <Button onClick={() => void runs.fetchNextPage()} loading={runs.isFetchingNextPage}>
          {t('liveOps.common.loadMore')}
        </Button>
      ) : null}
      {selected ? <RunDetail run={selected} /> : null}
    </Flex>
  );
}
