// GameDashboard (F079 T045, Clarification 2026-11-20): the minimal dashboard of workspace A — number of configs per type
// and workflow status (getGameConfigCounts, counted by the server in SQL). The "items needing translation" figure needs
// `translation_status` of F082 (not built yet): shown as not available instead of guessing. No play metrics (F080).
import { Alert, Card, Flex, Skeleton, Table, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { useConfigCounts } from '../api/console';
import type { ContentStatus } from '../api/gameConfigs';
import { GameOpsError } from '../shared/errors';

const STATUSES: ContentStatus[] = ['draft', 'in_review', 'approved', 'published', 'rejected', 'archived'];
const HEADLINE: ContentStatus[] = ['in_review', 'approved', 'draft'];

export function GameDashboard({ title }: { title: string }) {
  const { t } = useTranslation();
  const q = useConfigCounts();
  const rows = new Map<string, Record<string, number>>();
  for (const c of q.data?.data ?? []) rows.set(c.config_type, { ...(rows.get(c.config_type) ?? {}), [c.status]: c.count });
  const total = (s: ContentStatus) => (q.data?.data ?? []).filter((c) => c.status === s).reduce((n, c) => n + c.count, 0);
  return (
    <Flex vertical gap="middle" data-testid="game-dashboard">
      <Typography.Title level={1}>{title}</Typography.Title>
      {q.isPending ? (
        <Skeleton active />
      ) : q.error ? (
        <GameOpsError error={q.error} />
      ) : (
        <>
          <Flex gap="middle" wrap>
            {HEADLINE.map((s) => (
              <Card key={s} size="small" data-testid={`dashboard-total-${s}`}>
                <Typography.Text type="secondary">{t(`gameOps.status.${s}`)}</Typography.Text>
                <Typography.Title level={2}>{total(s)}</Typography.Title>
              </Card>
            ))}
          </Flex>
          <Table
            size="small"
            pagination={false}
            rowKey="type"
            data-testid="dashboard-counts"
            dataSource={[...rows.entries()].map(([type, counts]) => ({ type, ...counts }))}
            columns={[
              { key: 'type', title: t('gameOps.dashboard.type'), render: (_: unknown, r: { type: string }) => t(`gameOps.type.${r.type}`) },
              ...STATUSES.map((s) => ({ key: s, title: t(`gameOps.status.${s}`), render: (_: unknown, r: Record<string, unknown>) => (r[s] as number | undefined) ?? 0 })),
            ]}
          />
          <Alert type="info" showIcon message={t('gameOps.dashboard.translationsPending')} />
        </>
      )}
    </Flex>
  );
}
