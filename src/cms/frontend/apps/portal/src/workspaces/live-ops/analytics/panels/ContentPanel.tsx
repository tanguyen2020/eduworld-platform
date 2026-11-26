// Content panels of Game Analytics (F080 T044, F080-AC-23, GAME_ANALYTICS §7.1): "Quest & cốt truyện" (kind quest +
// quest_completion_rate, chapter_completion_rate) and "Challenge & boss" (challenge_success_rate, boss_defeat_rate +
// kind challenge | boss). The content table (getGameContentStats, sorted by abandon rate, cursor pages) lists starts,
// completion, abandon and success rates and the "cần xem lại" flag — no duration column (play time only in Chơi lành
// mạnh, AC-ANL-109). A title opens the item in workspace A (latest version editor of Game Operations Console) only when
// the user sees that menu (F080-AC-23); otherwise it is plain text.
import { DashboardWidget } from '@eduworld/ops-ui/dashboard';
import { Button, Flex, Segmented, Table, Tag } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import type { MenuKey } from '../../../../generated/menu-catalog';
import { menuByKey } from '../../../../permissions/menuRegistry';
import { liveOpsKey, useLiveOpsApi, type Schemas } from '../../api/common';
import { nameIn, useMenuVisible, useWidgetError } from '../../shared/ui';
import { useGameContentStats, useGameMetrics, type ContentKind, type ContentStatsItem } from '../api';
import { analyticsQuery, PANEL_METRICS, type AnalyticsFilters, type PanelKey } from '../model';
import { metaOf, MetricTiles } from './MetricPanel';
import { useQueryClient } from '@tanstack/react-query';

/** Workspace A menu + config type of each content kind (catalog keys of the generated registry, ADR-0090). */
const GAME_OPS: Record<ContentKind, { menu: MenuKey; type: 'quests' | 'challenges' | 'bosses' }> = {
  quest: { menu: 'menu.game_ops.quests', type: 'quests' },
  challenge: { menu: 'menu.game_ops.challenges', type: 'challenges' },
  boss: { menu: 'menu.game_ops.bosses', type: 'bosses' },
};

/**
 * Opens the editor of the current version of a content item in workspace A — route `:configType/:configId/v/:version`
 * of the menu (ConfigMenuScreen of F079); the version is the release pointer of listGameConfigVersions.
 */
export function useOpenInGameOps() {
  const api = useLiveOpsApi();
  const qc = useQueryClient();
  const navigate = useNavigate();
  return async (kind: ContentKind, contentId: string) => {
    const { menu, type } = GAME_OPS[kind];
    const history = await qc.fetchQuery({
      queryKey: liveOpsKey('content-versions', type, contentId),
      queryFn: () =>
        api.call<Schemas['ContentVersionHistory']>(
          api.client.GET('/game-admin/{config_type}/{config_id}/versions', { params: { path: { config_type: type, config_id: contentId }, query: { limit: 1 } } } as never),
        ),
    });
    const version = history.current_version ?? history.data[0]?.version;
    navigate(version ? `${menuByKey(menu)!.path}/${type}/${contentId}/v/${version}` : `${menuByKey(menu)!.path}/${type}`);
  };
}

function ContentTable({ kind, filters }: { kind: ContentKind; filters: AnalyticsFilters }) {
  const { t, i18n } = useTranslation();
  const [cursors, setCursors] = useState<(string | undefined)[]>([undefined]);
  const q = useGameContentStats(kind, analyticsQuery(filters), cursors.at(-1));
  const canOpen = useMenuVisible(GAME_OPS[kind].menu);
  const open = useOpenInGameOps();
  const [openError, setOpenError] = useState(false);
  const pct = (v: number | null | undefined) => (v == null ? '' : new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 1 }).format(v));
  const num = (v: number | undefined) => (v == null ? '' : new Intl.NumberFormat(i18n.language).format(v));
  const error = useWidgetError(q.error);
  const next = q.data?.page?.has_more ? q.data.page.next_cursor : undefined;
  return (
    <Flex vertical gap="small" data-testid={`content-${kind}`}>
      {error ? <DashboardWidget title={t(`liveOps.analytics.content.kind.${kind}`)} error={error} /> : null}
      <Table<ContentStatsItem>
        size="small"
        rowKey="content_id"
        loading={q.isPending}
        pagination={false}
        scroll={{ x: true }}
        dataSource={q.data?.data ?? []}
        columns={[
          {
            key: 'title',
            title: t('liveOps.analytics.content.title'),
            render: (_, r) => {
              const label = nameIn(r.title, i18n.language) || r.content_id;
              return canOpen ? (
                <Button type="link" onClick={() => open(r.kind, r.content_id).catch(() => setOpenError(true))} aria-label={t('liveOps.analytics.content.open', { name: label })}>
                  {label}
                </Button>
              ) : (
                label
              );
            },
          },
          { key: 'starts', title: t('liveOps.analytics.content.starts'), render: (_, r) => num(r.starts) },
          { key: 'completion', title: t('liveOps.analytics.content.completion'), render: (_, r) => pct(r.completion_rate) },
          { key: 'abandon', title: t('liveOps.analytics.content.abandon'), render: (_, r) => pct(r.abandon_rate) },
          { key: 'success', title: t('liveOps.analytics.content.success'), render: (_, r) => pct(r.success_rate) },
          { key: 'flag', title: t('liveOps.analytics.content.flag'), render: (_, r) => (r.flagged_for_review ? <Tag color="warning">{t('liveOps.analytics.content.flagged')}</Tag> : null) },
        ]}
      />
      {openError ? <Tag color="error">{t('liveOps.analytics.content.openFailed')}</Tag> : null}
      <Flex gap="small">
        {cursors.length > 1 ? <Button onClick={() => setCursors((c) => c.slice(0, -1))}>{t('liveOps.common.previous')}</Button> : null}
        {next ? <Button onClick={() => setCursors((c) => [...c, next])}>{t('liveOps.common.next')}</Button> : null}
      </Flex>
    </Flex>
  );
}

export function ContentPanel({ panel, filters, title }: { panel: Extract<PanelKey, 'quests' | 'challenges'>; filters: AnalyticsFilters; title: string }) {
  const { t } = useTranslation();
  const codes = PANEL_METRICS[panel];
  const [kind, setKind] = useState<ContentKind>(panel === 'quests' ? 'quest' : 'challenge');
  const q = useGameMetrics(panel, codes, analyticsQuery(filters));
  const error = useWidgetError(q.error);
  return (
    <DashboardWidget testId={`panel-${panel}`} title={title} meta={metaOf(q.data, filters.range.from)} loading={q.isPending} error={error}>
      {q.data ? (
        <Flex vertical gap="middle">
          <MetricTiles codes={codes} data={q.data} partial={q.data.is_partial} />
          {panel === 'challenges' ? (
            <Segmented<ContentKind>
              value={kind}
              onChange={setKind}
              options={(['challenge', 'boss'] as ContentKind[]).map((k) => ({ value: k, label: t(`liveOps.analytics.content.kind.${k}`) }))}
            />
          ) : null}
          <ContentTable key={kind} kind={kind} filters={filters} />
        </Flex>
      ) : null}
    </DashboardWidget>
  );
}
