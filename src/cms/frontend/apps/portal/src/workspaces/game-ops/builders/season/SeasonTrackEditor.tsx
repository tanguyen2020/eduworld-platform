// SeasonTrackEditor (F079-AC-15, FR-EVT-005, EVENT_SEASON_SYSTEM, ADR-0049, AGENTS R19): the season track — table of
// tiers × rewards, season point sources with the daily cap, end-of-season rule — from season-track.schema.json, plus the
// server simulation ("chơi N ngày/tuần đạt cấp nào"). `paid_track` is `const false` in the schema: it is filled and
// has NO control (no premium track, no pay-to-win).
import { Card, Flex, Table, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { GenericSchemaBuilder } from '../GenericSchemaBuilder';
import { SimulationPanel } from '../progression/SimulationPanel';
import type { BuilderProps } from '../types';

type Tier = { tier?: number; is_milestone?: boolean; rewards?: Record<string, unknown>[] };

const rewardText = (r: Record<string, unknown>) => [r.type, r.currency_code ?? r.item_id ?? r.badge_id ?? r.title_code ?? r.collection_item_id, r.amount].filter((x) => x !== undefined && x !== null).join(' · ');

export function SeasonTrackEditor(props: BuilderProps) {
  const { t } = useTranslation();
  const tiers = (Array.isArray(props.value.tiers) ? props.value.tiers : []) as Tier[];
  const points = (props.value.points ?? {}) as { per_tier?: number; daily_cap?: number };
  return (
    <Flex vertical gap="middle">
      <Card size="small" title={t('gameOps.season.tiers', { count: tiers.length })}>
        <Typography.Text type="secondary">{t('gameOps.season.pointsSummary', { perTier: points.per_tier ?? '—', cap: points.daily_cap ?? '—' })}</Typography.Text>
        <Table
          size="small"
          pagination={false}
          data-testid="season-tiers"
          rowKey={(r: Tier) => String(r.tier)}
          dataSource={tiers}
          columns={[
            { key: 'tier', title: t('gameOps.season.tier'), render: (_: unknown, r: Tier) => <>{r.tier} {r.is_milestone ? <Tag>{t('gameOps.season.milestone')}</Tag> : null}</> },
            { key: 'rewards', title: t('gameOps.season.rewards'), render: (_: unknown, r: Tier) => (r.rewards ?? []).map(rewardText).join('; ') },
          ]}
        />
        <Typography.Text type="secondary">{t('gameOps.season.freeOnly')}</Typography.Text>
      </Card>
      <GenericSchemaBuilder {...props} />
      <SimulationPanel configType="season-tracks" item={props.item} />
    </Flex>
  );
}
