// Efficacy report view (F035-AC-29…AC-31, AC-35): renders the aggregates computed by the backend job as they are —
// arms, effect size with its CI, gain by objective, reliability, usage, predictive validity. A suppressed cell
// (< analytics_min_cell_size) arrives as null and is shown as "ẩn"; nothing is computed here (R1).
import { Alert, Collapse, Descriptions, Table, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { formatTime } from '../ui';
import type { EfficacyReport } from './api';
import { StatusTag } from './ResearchForms';

type Row = Record<string, unknown>;

const ARM_FIELDS = ['cohorts', 'participants', 'analysed', 'withdrawn', 'missing_pre', 'missing_post', 'attrition_rate', 'pre_mean', 'post_mean', 'gain_mean', 'gain_sd', 'normalized_gain_mean'];
const EFFECT_FIELDS = ['hedges_g', 'ci_low', 'ci_high', 'se', 'se_adjusted', 'icc', 'design_effect', 'n_intervention', 'n_comparison'];

function Cell({ value }: { value: unknown }) {
  const { t } = useTranslation();
  if (value === null || value === undefined) return <Tag>{t('platform.research.report.hidden')}</Tag>;
  if (typeof value === 'boolean') return <>{t(value ? 'platform.research.common.yes' : 'platform.research.common.no')}</>;
  if (typeof value === 'object') return <Typography.Text code>{JSON.stringify(value)}</Typography.Text>;
  return <>{String(value)}</>;
}

export function ReportView({ report }: { report: EfficacyReport }) {
  const { t } = useTranslation();
  const r = (report.results ?? {}) as Row;
  const arms = (r.arms ?? {}) as Record<string, Row>;
  const effect = r.effect as Row | undefined;
  const objectives = (r.objectives ?? []) as Row[];
  return (
    <section aria-label={t('platform.research.report.title')}>
      <Descriptions size="small" column={{ xs: 1, md: 3 }}>
        <Descriptions.Item label={t('platform.research.common.status')}>
          <StatusTag status={report.status} />
        </Descriptions.Item>
        <Descriptions.Item label={t('platform.research.report.finishedAt')}>{formatTime(report.finished_at)}</Descriptions.Item>
        <Descriptions.Item label={t('platform.research.report.excludedCount')}>{report.excluded_question_version_ids?.length ?? 0}</Descriptions.Item>
      </Descriptions>
      {report.status === 'failed' ? <Alert type="error" showIcon message={t('platform.research.report.failed')} /> : null}
      {report.descriptive_only ? <Alert type="warning" showIcon message={t('platform.research.report.descriptiveOnly')} /> : null}
      {r.attrition_warning === true ? <Alert type="warning" showIcon message={t('platform.research.report.attritionWarning')} /> : null}
      {report.status === 'succeeded' ? (
        <>
          <Typography.Title level={4}>{t('platform.research.report.arms')}</Typography.Title>
          <Table
            rowKey="field"
            size="small"
            pagination={false}
            dataSource={ARM_FIELDS.map((f) => ({ field: f, intervention: arms.intervention?.[f], comparison: arms.comparison?.[f] }))}
            columns={[
              { key: 'field', title: t('platform.research.report.metric'), render: (_, x) => t(`platform.research.report.field.${x.field}`) },
              { key: 'intervention', title: t('platform.research.arm.intervention'), render: (_, x) => <Cell value={x.intervention} /> },
              { key: 'comparison', title: t('platform.research.arm.comparison'), render: (_, x) => <Cell value={x.comparison} /> },
            ]}
          />
          <Typography.Title level={4}>{t('platform.research.report.effect')}</Typography.Title>
          {effect ? (
            <Descriptions size="small" column={{ xs: 1, md: 3 }}>
              {EFFECT_FIELDS.map((f) => (
                <Descriptions.Item key={f} label={t(`platform.research.report.field.${f}`)}>
                  <Cell value={effect[f]} />
                </Descriptions.Item>
              ))}
            </Descriptions>
          ) : (
            <Typography.Paragraph type="secondary">{t('platform.research.report.noEffect')}</Typography.Paragraph>
          )}
          <Typography.Title level={4}>{t('platform.research.report.objectives')}</Typography.Title>
          <Table
            rowKey={(x) => String(x.objective_id)}
            size="small"
            pagination={false}
            dataSource={objectives}
            columns={[
              { key: 'lo', title: t('platform.research.report.objective'), render: (_, x) => <Typography.Text code>{String(x.objective_id)}</Typography.Text> },
              { key: 'ig', title: t('platform.research.report.interventionGain'), render: (_, x) => <Cell value={x.intervention_gain} /> },
              { key: 'cg', title: t('platform.research.report.comparisonGain'), render: (_, x) => <Cell value={x.comparison_gain} /> },
            ]}
          />
          <Collapse
            items={(['reliability', 'usage', 'validity', 'thresholds'] as const).map((k) => ({
              key: k,
              label: t(`platform.research.report.section.${k}`),
              children: <Typography.Paragraph code>{JSON.stringify(r[k] ?? null, null, 2)}</Typography.Paragraph>,
            }))}
          />
        </>
      ) : null}
    </section>
  );
}
