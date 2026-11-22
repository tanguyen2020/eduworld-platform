// SimulationEditor (F066 T045, F066-AC-26, SIMULATION_ENGINE §4, ADMIN_PORTAL §3.13 Simulation Configuration): the
// config of `simulation` in the Activity Builder (F058 editor registry) — a packaged template from
// listSimulationTemplates (active versions only), its parameters as a form generated from `parameter_schema` (numbers,
// booleans, choices; never a script or code field, ADR-0052), goals whose check metric comes from the template metrics,
// steps, conclusion questions from the Question Bank, seed policy and runs. Early checks only; module learning
// validates with the template rules (ACTIVITY_CONFIG_INVALID).
import { DeleteOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Card, Flex, Form, Input, InputNumber, List, Switch, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePortalRuntime } from '../../../../../app/runtime';
import { actionStatus } from '../../../../../permissions/menuRegistry';
import { usePermissions } from '../../../../../permissions/usePermissions';
import { useSession } from '../../../../../session/SessionProvider';
import type { components } from '../../../../../shared/api/schema.gen';
import { QuestionPicker } from '../../../question-sets/SelectionRuleEditor';
import type { Config } from '../../activityModel';
import type { ActivityEditorProps, BlockerContext } from '../../editorRegistry';
import { AUTO_TYPES } from '../../validators/questionSourceRules';
import { SearchSelect } from '@eduworld/ops-ui';

export type SimulationTemplate = components['schemas']['SimulationTemplate'];
type ParamSchema = { type?: string | string[]; minimum?: number; maximum?: number; enum?: unknown[]; maxLength?: number; title?: string };
type Goal = { goal_id: string; description?: { vi?: string; en?: string }; check: { metric: string; operator: string; value: number; tolerance?: number } };
type Step = { step_id: string; instruction?: { vi?: string; en?: string }; safety_required?: boolean };

/** goals[].check.operator enum (simulation.schema.json). */
export const OPERATORS = ['eq', 'lt', 'lte', 'gt', 'gte', 'abs_diff_lte'] as const;

export function useSimulationTemplates(enabled = true) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    enabled,
    queryKey: ['content-studio', 'simulation-templates', scopeKey],
    queryFn: () => api.call<{ data: SimulationTemplate[] }>(api.client.GET('/cms/learning/simulation-templates', {})),
    staleTime: 5 * 60_000,
  });
}

export function simulationConfig(): Config {
  return {
    content: { simulation_kind: 'physics', template_code: '', min_template_version: 1, parameters: {}, goals: [], steps: [], questions: [], seed_policy: 'per_attempt' },
    interaction: { show_measurements: true },
    evaluation: { strategies: ['deterministic_check', 'auto'], max_score: 10 },
    feedback: { timing: 'immediate' },
    completion_rule: { mode: 'goals_met' },
  };
}

const obj = (v: unknown): Record<string, unknown> => (v && typeof v === 'object' ? (v as Record<string, unknown>) : {});

/** Local blockers (F066-AC-26): a template, at least one goal with a metric, required parameters set. */
export function simulationBlockers(config: Config, ctx: BlockerContext) {
  const c = obj(config.content);
  const out: { field: string; message: string }[] = [];
  if (!c.template_code) out.push({ field: '/config/content/template_code', message: ctx.t('content_studio.activity.simulation.noTemplate') });
  const goals = (c.goals as Goal[] | undefined) ?? [];
  if (goals.length === 0) out.push({ field: '/config/content/goals', message: ctx.t('content_studio.activity.simulation.noGoal') });
  goals.forEach((g, i) => {
    if (!g.check?.metric) out.push({ field: `/config/content/goals/${i}/check/metric`, message: ctx.t('content_studio.activity.simulation.goalMetric', { n: i + 1 }) });
  });
  return out;
}

function ParamField({ name, schema, value, required, readOnly, onChange }: { name: string; schema: ParamSchema; value: unknown; required: boolean; readOnly: boolean; onChange: (v: unknown) => void }) {
  const type = Array.isArray(schema.type) ? schema.type[0] : schema.type;
  const label = schema.title ?? name;
  return (
    <Form.Item label={label} required={required}>
      {schema.enum ? (
        <SearchSelect aria-label={label} disabled={readOnly} value={value as string} options={schema.enum.map((e) => ({ value: e as string, label: String(e) }))} onChange={(v) => onChange(v)} />
      ) : type === 'number' || type === 'integer' ? (
        <InputNumber aria-label={label} disabled={readOnly} min={schema.minimum} max={schema.maximum} step={type === 'integer' ? 1 : 0.1} precision={type === 'integer' ? 0 : undefined} value={value as number | undefined} onChange={(v) => onChange(v ?? undefined)} />
      ) : type === 'boolean' ? (
        <Switch aria-label={label} disabled={readOnly} checked={!!value} onChange={(v) => onChange(v)} />
      ) : (
        <Input aria-label={label} disabled={readOnly} maxLength={Math.min(schema.maxLength ?? 128, 128)} value={(value as string) ?? ''} onChange={(e) => onChange(e.target.value)} />
      )}
    </Form.Item>
  );
}

export function SimulationEditor({ config, onChange, readOnly, errors }: ActivityEditorProps) {
  const { t } = useTranslation();
  const { view } = usePermissions();
  // F066-AC-29: the template list is action.content_studio.simulations.view of the catalog (turned off → notice).
  const canList = !!view && actionStatus('action.content_studio.simulations.view', view) === 'enabled';
  const templates = useSimulationTemplates(canList);
  const content = obj(config.content);
  const interaction = obj(config.interaction);
  const [picking, setPicking] = useState(false);
  const active = (templates.data?.data ?? []).filter((x) => x.status !== 'retired');
  const tpl = (templates.data?.data ?? []).find((x) => x.template_code === content.template_code && x.template_version >= ((content.min_template_version as number) ?? 1));
  const set = (patch: Record<string, unknown>) => onChange({ ...config, content: { ...content, ...patch } });
  const params = obj(content.parameters);
  const schema = obj(tpl?.parameter_schema) as { properties?: Record<string, ParamSchema>; required?: string[] };
  const goals = (content.goals as Goal[] | undefined) ?? [];
  const steps = (content.steps as Step[] | undefined) ?? [];
  const questions = (content.questions as { question_version_id: string }[] | undefined) ?? [];
  const metrics = tpl?.metrics ?? [];
  const upd = <T,>(list: T[], i: number, patch: Partial<T>) => list.map((x, k) => (k === i ? { ...x, ...patch } : x));
  const err = (p: string) => errors[`/config/content/${p}`];
  const paramErr = err('parameters');
  const goalsErr = err('goals');
  return (
    <Flex vertical gap="middle" data-testid="simulation-editor">
      <Card size="small" title={t('content_studio.activity.simulation.template')}>
        <SearchSelect
          aria-label={t('content_studio.activity.simulation.template')}
          placeholder={t('content_studio.activity.simulation.template')}
          disabled={readOnly}
          loading={templates.isFetching}
          style={{ minWidth: 320 }}
          value={content.template_code ? `${content.template_code as string}@${(content.min_template_version as number) ?? 1}` : undefined}
          options={active.map((x) => ({ value: `${x.template_code}@${x.template_version}`, label: `${x.title} (${x.template_code} v${x.template_version})` }))}
          onChange={(v: string) => {
            const [code, version] = v.split('@');
            const next = active.find((x) => x.template_code === code && String(x.template_version) === version);
            set({ template_code: code, min_template_version: Number(version), simulation_kind: next?.simulation_kind ?? content.simulation_kind, parameters: {}, goals: [] });
          }}
        />
        {!canList && <Alert type="warning" showIcon data-testid="simulation-templates-off" message={t('shell.featureDisabled.title')} />}
        {tpl && !tpl.checker_available && <Alert type="info" showIcon message={t('content_studio.activity.simulation.noChecker')} />}
      </Card>
      {tpl && (
        <Card size="small" title={t('content_studio.activity.simulation.parameters')} data-testid="simulation-parameters">
          <Form layout="vertical" component="div" disabled={readOnly}>
            {Object.entries(schema.properties ?? {}).map(([name, s]) => (
              <ParamField key={name} name={name} schema={s} value={params[name]} required={(schema.required ?? []).includes(name)} readOnly={readOnly} onChange={(v) => set({ parameters: Object.fromEntries(Object.entries({ ...params, [name]: v }).filter(([, x]) => x !== undefined && x !== '')) })} />
            ))}
          </Form>
          {paramErr && <Typography.Text type="danger">{paramErr}</Typography.Text>}
        </Card>
      )}
      <Card size="small" title={t('content_studio.activity.simulation.goals')}>
        <Flex vertical gap="small" data-testid="simulation-goals">
          {goals.map((g, i) => (
            <Flex key={i} gap="small" align="center" wrap>
              <Input aria-label={t('content_studio.activity.simulation.goalIdOf', { n: i + 1 })} maxLength={64} disabled={readOnly} value={g.goal_id} style={{ width: 140 }} onChange={(e) => set({ goals: upd(goals, i, { goal_id: e.target.value }) })} />
              <Input aria-label={t('content_studio.activity.simulation.goalTextOf', { n: i + 1 })} maxLength={128} disabled={readOnly} value={g.description?.vi ?? ''} onChange={(e) => set({ goals: upd(goals, i, { description: { vi: e.target.value } }) })} />
              <SearchSelect aria-label={t('content_studio.activity.simulation.metricOf', { n: i + 1 })} disabled={readOnly} style={{ width: 160 }} value={g.check.metric || undefined} options={metrics.map((m) => ({ value: m.metric, label: m.unit ? `${m.metric} (${m.unit})` : m.metric }))} onChange={(v: string) => set({ goals: upd(goals, i, { check: { ...g.check, metric: v } }) })} />
              <SearchSelect aria-label={t('content_studio.activity.simulation.operatorOf', { n: i + 1 })} disabled={readOnly} style={{ width: 120 }} value={g.check.operator} options={OPERATORS.map((o) => ({ value: o, label: t(`content_studio.activity.simulation.operators.${o}`) }))} onChange={(v: string) => set({ goals: upd(goals, i, { check: { ...g.check, operator: v } }) })} />
              <InputNumber aria-label={t('content_studio.activity.simulation.valueOf', { n: i + 1 })} disabled={readOnly} value={g.check.value} onChange={(v) => set({ goals: upd(goals, i, { check: { ...g.check, value: v ?? 0 } }) })} />
              {g.check.operator === 'abs_diff_lte' && <InputNumber aria-label={t('content_studio.activity.simulation.toleranceOf', { n: i + 1 })} disabled={readOnly} min={0} value={g.check.tolerance} onChange={(v) => set({ goals: upd(goals, i, { check: { ...g.check, tolerance: v ?? 0 } }) })} />}
              {!readOnly && <Button size="small" icon={<DeleteOutlined />} aria-label={t('content_studio.activity.simulation.removeGoal', { n: i + 1 })} onClick={() => set({ goals: goals.filter((_, k) => k !== i) })} />}
            </Flex>
          ))}
          {goalsErr && <Typography.Text type="danger">{goalsErr}</Typography.Text>}
          {!readOnly && goals.length < 20 && <Button disabled={!tpl} onClick={() => set({ goals: [...goals, { goal_id: `goal${goals.length + 1}`, description: {}, check: { metric: '', operator: 'eq', value: 1 } }] })}>{t('content_studio.activity.simulation.addGoal')}</Button>}
        </Flex>
      </Card>
      <Card size="small" title={t('content_studio.activity.simulation.steps')}>
        <Flex vertical gap="small">
          {steps.map((s, i) => (
            <Flex key={i} gap="small" align="center">
              <Input aria-label={t('content_studio.activity.simulation.stepOf', { n: i + 1 })} maxLength={500} disabled={readOnly} value={s.instruction?.vi ?? ''} onChange={(e) => set({ steps: upd(steps, i, { instruction: { vi: e.target.value } }) })} />
              <Switch aria-label={t('content_studio.activity.simulation.safetyOf', { n: i + 1 })} disabled={readOnly} checked={!!s.safety_required} onChange={(v) => set({ steps: upd(steps, i, { safety_required: v }) })} />
              {!readOnly && <Button size="small" icon={<DeleteOutlined />} aria-label={t('content_studio.activity.simulation.removeStep', { n: i + 1 })} onClick={() => set({ steps: steps.filter((_, k) => k !== i) })} />}
            </Flex>
          ))}
          {!readOnly && steps.length < 30 && <Button onClick={() => set({ steps: [...steps, { step_id: `step${steps.length + 1}`, instruction: {}, safety_required: false }] })}>{t('content_studio.activity.simulation.addStep')}</Button>}
        </Flex>
      </Card>
      <Card size="small" title={t('content_studio.activity.simulation.questions')}>
        <List size="small" bordered dataSource={questions} locale={{ emptyText: t('content_studio.activity.source.empty') }} renderItem={(q, i) => <List.Item actions={readOnly ? [] : [<Button key="rm" size="small" icon={<DeleteOutlined />} aria-label={t('content_studio.activity.source.remove', { n: i + 1 })} onClick={() => set({ questions: questions.filter((_, k) => k !== i) })} />]}>{t('content_studio.activity.audio.questionItem', { n: i + 1, id: q.question_version_id.slice(-8) })}</List.Item>} />
        {!readOnly && questions.length < 20 && <Button onClick={() => setPicking(true)}>{t('content_studio.activity.source.add')}</Button>}
        <QuestionPicker
          open={picking}
          onClose={() => setPicking(false)}
          onPick={(q) => {
            if (q.content_version_id && AUTO_TYPES.includes(q.type ?? '')) set({ questions: [...questions, { question_version_id: q.content_version_id }] });
            setPicking(false);
          }}
        />
      </Card>
      <Card size="small" title={t('content_studio.activity.simulation.runs')}>
        <Form layout="vertical" component="div" disabled={readOnly}>
          <Flex gap="middle" align="center" wrap>
            <Form.Item label={t('content_studio.activity.simulation.seedPolicy')}>
              <SearchSelect aria-label={t('content_studio.activity.simulation.seedPolicy')} style={{ minWidth: 200 }} value={(content.seed_policy as string) ?? 'per_attempt'} options={['per_attempt', 'fixed'].map((p) => ({ value: p, label: t(`content_studio.activity.simulation.seed.${p}`) }))} onChange={(v: string) => set({ seed_policy: v })} />
            </Form.Item>
            <Form.Item label={t('content_studio.activity.simulation.maxRuns')}>
              <InputNumber aria-label={t('content_studio.activity.simulation.maxRuns')} min={1} max={50} value={interaction.max_runs as number | undefined} onChange={(v) => onChange({ ...config, interaction: { ...interaction, max_runs: v ?? undefined } })} />
            </Form.Item>
            <Flex gap="small" align="center">
              <Switch aria-label={t('content_studio.activity.simulation.measurements')} checked={(interaction.show_measurements as boolean | undefined) ?? true} onChange={(v) => onChange({ ...config, interaction: { ...interaction, show_measurements: v } })} />
              <Typography.Text>{t('content_studio.activity.simulation.measurements')}</Typography.Text>
            </Flex>
          </Flex>
        </Form>
        <Typography.Text type="secondary">{t('content_studio.activity.simulation.noScript')}</Typography.Text>
      </Card>
    </Flex>
  );
}
