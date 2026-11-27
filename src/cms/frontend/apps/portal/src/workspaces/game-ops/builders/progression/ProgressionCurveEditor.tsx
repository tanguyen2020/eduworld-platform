// ProgressionCurveEditor (F079-AC-09, AC-10, FR-GAM-007, PROGRESSION_SYSTEM §3–§4, ADMIN_PORTAL §3.12): Level & XP
// Configuration — the curve (formula base/exponent/linear/max_level or `level_overrides`), titles by level range and
// level-up rewards from progression-curve.schema.json; the threshold table, chart and simulation come from the server
// (SimulationPanel); the XP source catalog with its caps is the published reward rules (code, source, cap per day),
// edited in "Phần thưởng". Nothing is computed in the browser (AGENTS R1).
import { Card, Flex, Table, Typography } from 'antd';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAllConfigs } from '../../api/gameConfigs';
import { menuPathOf } from '../../registry/configTypes';
import { GenericSchemaBuilder } from '../GenericSchemaBuilder';
import type { BuilderProps } from '../types';
import { SimulationPanel } from './SimulationPanel';

export function XpSourceCatalog() {
  const { t } = useTranslation();
  const rules = useAllConfigs('reward-rules', 'published');
  const rows = rules.items.map((r) => {
    const caps = (r.payload.caps ?? {}) as Record<string, unknown>;
    return { key: r.config_id, code: String(r.payload.code ?? ''), source: String(r.payload.source_code ?? ''), cap: caps.xp_per_day as number | undefined };
  });
  return (
    <Card size="small" title={t('gameOps.progression.sources')} extra={<Link to={`${menuPathOf('rewards')}/reward-rules`}>{t('gameOps.progression.editSources')}</Link>}>
      <Table
        size="small"
        loading={rules.isPending}
        pagination={false}
        data-testid="xp-sources"
        dataSource={rows}
        columns={[
          { key: 'code', title: t('gameOps.progression.sourceCode'), dataIndex: 'code' },
          { key: 'source', title: t('gameOps.progression.sourceType'), dataIndex: 'source' },
          { key: 'cap', title: t('gameOps.progression.capPerDay'), render: (_: unknown, r: { cap?: number }) => r.cap ?? '—' },
        ]}
      />
      <Typography.Text type="secondary">{t('gameOps.progression.capHint')}</Typography.Text>
    </Card>
  );
}

export function ProgressionCurveEditor(props: BuilderProps) {
  return (
    <Flex vertical gap="middle">
      <GenericSchemaBuilder {...props} />
      <SimulationPanel configType="progression-curves" item={props.item} />
      <XpSourceCatalog />
    </Flex>
  );
}
