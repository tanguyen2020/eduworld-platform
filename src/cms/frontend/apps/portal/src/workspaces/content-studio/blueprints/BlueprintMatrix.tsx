// BlueprintMatrix (F054-AC-22, ADMIN_PORTAL §3.2, EXAM_ENGINE §3): the "LO × độ khó" matrix of an assessment
// blueprint section — rows = LOs, columns = difficulty levels with the points of one question, cells = number of
// questions; row / column / grand totals and the check against `total_points`; each cell shows the pool of its rule
// (available / required) and is highlighted when the pool is insufficient (QUESTION_POOL_INSUFFICIENT, pool stats).
import { DeleteOutlined } from '@ant-design/icons';
import { Alert, Button, Flex, InputNumber, Select, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { BP_LIMITS, cellKey, totalPoints, totalQuestions, toRules, type CellPool, type Matrix } from './matrixModel';

export interface BlueprintMatrixProps {
  matrix: Matrix;
  onChange: (m: Matrix) => void;
  /** LO id → label (code — description). */
  objectiveLabels: ReadonlyMap<string, string>;
  /** LOs that can be added as rows. */
  objectiveOptions: { value: string; label: string }[];
  targetPoints: number | undefined;
  pools: Readonly<Record<string, CellPool>>;
  /** Cells to highlight after QUESTION_POOL_INSUFFICIENT. */
  highlightInsufficient: boolean;
  readOnly: boolean;
}

interface Row {
  lo: string;
}

export function BlueprintMatrix({ matrix, onChange, objectiveLabels, objectiveOptions, targetPoints, pools, highlightInsufficient, readOnly }: BlueprintMatrixProps) {
  const { t } = useTranslation();
  const rules = toRules(matrix);
  const points = totalPoints(rules);
  const questions = totalQuestions(rules);
  const setCount = (lo: string, d: number, v: number | null) => onChange({ ...matrix, counts: { ...matrix.counts, [cellKey(lo, d)]: Math.max(0, Math.min(BP_LIMITS.count, v ?? 0)) } });
  const columns: ColumnsType<Row> = [
    { key: 'lo', title: t('content_studio.blueprint.lo'), render: (_, r) => <Typography.Text>{objectiveLabels.get(r.lo) ?? r.lo}</Typography.Text>, fixed: 'left' },
    ...matrix.difficulties.map((d) => ({
      key: `d${d}`,
      title: (
        <Flex vertical gap="small">
          <Typography.Text strong>{t('content_studio.blueprint.difficulty', { d })}</Typography.Text>
          <InputNumber
            size="small"
            aria-label={t('content_studio.blueprint.pointsOf', { d })}
            addonAfter={t('content_studio.blueprint.pointsUnit')}
            min={0.25}
            step={0.25}
            disabled={readOnly}
            value={matrix.points[d]}
            onChange={(v) => onChange({ ...matrix, points: { ...matrix.points, [d]: v ?? 1 } })}
          />
        </Flex>
      ),
      render: (_: unknown, r: Row) => {
        const k = cellKey(r.lo, d);
        const pool = pools[k];
        const bad = !!pool?.insufficient;
        return (
          <Flex vertical gap="small" data-cell={k} data-insufficient={bad ? 'true' : undefined}>
            <InputNumber size="small" aria-label={t('content_studio.blueprint.cell', { lo: objectiveLabels.get(r.lo) ?? r.lo, d })} min={0} max={BP_LIMITS.count} disabled={readOnly} value={matrix.counts[k] ?? 0} status={bad && highlightInsufficient ? 'error' : undefined} onChange={(v) => setCount(r.lo, d, v)} />
            {pool && (matrix.counts[k] ?? 0) > 0 ? (
              <Tag color={bad ? 'error' : pool.warning ? 'warning' : 'success'}>{t('content_studio.questionSet.pool', { available: pool.available, required: pool.required })}</Tag>
            ) : null}
          </Flex>
        );
      },
    })),
    {
      key: 'row',
      title: t('content_studio.blueprint.rowTotal'),
      render: (_, r) => matrix.difficulties.reduce((n, d) => n + (matrix.counts[cellKey(r.lo, d)] ?? 0), 0),
    },
    ...(readOnly
      ? []
      : [
          {
            key: 'rm',
            title: '',
            render: (_: unknown, r: Row) => (
              <Button size="small" icon={<DeleteOutlined />} aria-label={t('content_studio.blueprint.removeLo', { lo: objectiveLabels.get(r.lo) ?? r.lo })} onClick={() => onChange({ ...matrix, objectives: matrix.objectives.filter((x) => x !== r.lo) })} />
            ),
          },
        ]),
  ];
  const mismatch = targetPoints !== undefined && targetPoints > 0 && Math.abs(points - targetPoints) > 0.001;
  return (
    <Flex vertical gap="small" data-testid="blueprint-matrix">
      <Flex gap="small" wrap align="center">
        <Select
          mode="multiple"
          aria-label={t('content_studio.blueprint.difficulties')}
          placeholder={t('content_studio.blueprint.difficulties')}
          disabled={readOnly}
          value={matrix.difficulties}
          options={[1, 2, 3, 4, 5].map((d) => ({ value: d, label: t('content_studio.blueprint.difficulty', { d }) }))}
          onChange={(v: number[]) => onChange({ ...matrix, difficulties: [...v].sort((a, b) => a - b), points: Object.fromEntries(v.map((d) => [d, matrix.points[d] ?? 1])) })}
          style={{ minWidth: 240 }}
        />
        <Select
          showSearch
          optionFilterProp="label"
          aria-label={t('content_studio.blueprint.addLo')}
          placeholder={t('content_studio.blueprint.addLo')}
          disabled={readOnly}
          value={null}
          options={objectiveOptions.filter((o) => !matrix.objectives.includes(o.value))}
          onChange={(lo: string) => onChange({ ...matrix, objectives: [...matrix.objectives, lo] })}
          style={{ minWidth: 280 }}
        />
      </Flex>
      <Table<Row> size="small" rowKey="lo" columns={columns} dataSource={matrix.objectives.map((lo) => ({ lo }))} pagination={false} scroll={{ x: true }} locale={{ emptyText: t('content_studio.blueprint.noLo') }} />
      <Typography.Text data-testid="blueprint-totals">{t('content_studio.blueprint.totals', { questions, points, target: targetPoints ?? '—' })}</Typography.Text>
      {mismatch && <Alert type="error" showIcon message={t('content_studio.blueprint.pointsMismatch', { points, target: targetPoints })} />}
      {rules.length > BP_LIMITS.rules && <Alert type="error" showIcon message={t('content_studio.validation.maxItems', { limit: BP_LIMITS.rules })} />}
      {highlightInsufficient && Object.values(pools).some((p) => p.insufficient) && <Alert type="error" showIcon message={t('content_studio.blueprint.poolInsufficient')} />}
    </Flex>
  );
}
