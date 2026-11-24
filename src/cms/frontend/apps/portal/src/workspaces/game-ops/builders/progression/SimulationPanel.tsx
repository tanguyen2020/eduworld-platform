// SimulationPanel (F079-AC-09, AC-10, AC-15, PROGRESSION_SYSTEM §4, ADMIN_PORTAL §3.12): asks the SERVER to simulate a
// saved version (simulateGameConfig — "chơi N phút/ngày, N ngày/tuần trong N tuần đạt level nào") and shows what it
// returns: the threshold table + chart (level → cumulative XP / tier → points) and the level reached per week. The
// portal computes nothing (AGENTS R1). The panel re-runs when the saved copy changes (autosave revision).
import { DashboardChart } from '@eduworld/ops-ui/dashboard';
import { Alert, Button, Card, Flex, Form, InputNumber, Table, Typography } from 'antd';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { GameOpsError } from '../../shared/errors';
import { useSimulation, type Simulation } from '../../api/console';
import type { SavedVersion } from '../types';

export interface SimulationPanelProps {
  configType: 'progression-curves' | 'season-tracks';
  item?: SavedVersion;
}

/** Defaults of the form: PROGRESSION_SYSTEM §4.1 balance target (20–30 min/day ≈ level 10 after 4 weeks) [Proposed]. */
const DEFAULTS = { minutes: 25, days: 5, weeks: 8 };

export function SimulationPanel({ configType, item }: SimulationPanelProps) {
  const { t } = useTranslation();
  const sim = useSimulation();
  const curve = configType === 'progression-curves';
  const [form, setForm] = useState(DEFAULTS);
  const run = (f = form) => item && sim.mutate({ configType, configId: item.configId, version: item.version, ...(curve ? { minutesPerDay: f.minutes } : {}), daysPerWeek: f.days, weeks: f.weeks });
  // first run and after every save of the version: thresholds and chart come from the server
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => void run(), [item?.configId, item?.version, item?.revision]);
  if (!item) return <Alert type="info" showIcon message={t('gameOps.sim.saveFirst')} />;
  const data: Simulation | undefined = sim.data;
  const unit = curve ? t('gameOps.sim.xp') : t('gameOps.sim.points');
  return (
    <Card size="small" title={curve ? t('gameOps.sim.curveTitle') : t('gameOps.sim.seasonTitle')} data-testid="simulation-panel">
      <Flex vertical gap="middle">
        <Form layout="inline" component="div">
          {curve && (
            <Form.Item label={t('gameOps.sim.minutes')} htmlFor="sim-minutes">
              <InputNumber id="sim-minutes" aria-label={t('gameOps.sim.minutes')} min={1} max={240} value={form.minutes} onChange={(v) => setForm({ ...form, minutes: v ?? DEFAULTS.minutes })} />
            </Form.Item>
          )}
          <Form.Item label={t('gameOps.sim.days')} htmlFor="sim-days">
            <InputNumber id="sim-days" aria-label={t('gameOps.sim.days')} min={1} max={7} value={form.days} onChange={(v) => setForm({ ...form, days: v ?? DEFAULTS.days })} />
          </Form.Item>
          <Form.Item label={t('gameOps.sim.weeks')} htmlFor="sim-weeks">
            <InputNumber id="sim-weeks" aria-label={t('gameOps.sim.weeks')} min={1} max={52} value={form.weeks} onChange={(v) => setForm({ ...form, weeks: v ?? DEFAULTS.weeks })} />
          </Form.Item>
          <Button type="primary" loading={sim.isPending} onClick={() => run()}>
            {t('gameOps.sim.run')}
          </Button>
        </Form>
        {sim.error ? <GameOpsError error={sim.error} /> : null}
        {data && (
          <>
            <Typography.Text type="secondary">
              {t('gameOps.sim.assumptions', { perDay: data.assumptions.per_play_day, unit, days: data.assumptions.days_per_week, cap: data.assumptions.daily_cap ?? '—' })}
            </Typography.Text>
            <Table
              size="small"
              rowKey="week"
              pagination={false}
              data-testid="simulation-weeks"
              dataSource={data.points}
              columns={[
                { key: 'week', title: t('gameOps.sim.week'), dataIndex: 'week' },
                { key: 'level', title: curve ? t('gameOps.sim.level') : t('gameOps.sim.tier'), dataIndex: 'level' },
                { key: 'total', title: t('gameOps.sim.total', { unit }), dataIndex: 'total' },
              ]}
            />
            <DashboardChart
              kind="line"
              testId="threshold-chart"
              categories={data.thresholds.map((x) => String(x.level))}
              values={data.thresholds.map((x) => x.total)}
              ariaLabel={t('gameOps.sim.chartLabel', { unit, values: data.thresholds.map((x) => `${x.level}: ${x.total}`).join(', ') })}
            />
            <Table
              size="small"
              rowKey="level"
              data-testid="threshold-table"
              pagination={false}
              dataSource={data.thresholds}
              columns={[
                { key: 'level', title: curve ? t('gameOps.sim.level') : t('gameOps.sim.tier'), dataIndex: 'level' },
                { key: 'to_next', title: t('gameOps.sim.toNext', { unit }), dataIndex: 'to_next' },
                { key: 'total', title: t('gameOps.sim.cumulative', { unit }), dataIndex: 'total' },
              ]}
            />
          </>
        )}
      </Flex>
    </Card>
  );
}
