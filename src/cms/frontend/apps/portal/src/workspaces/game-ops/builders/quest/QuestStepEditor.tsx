// QuestStepEditor (F079-AC-06, AC-07, QUEST_SYSTEM §3, ADMIN_PORTAL §3.3): the quest builder — the quest fields from
// quest.schema.json, and the steps as a drag-and-drop list (also movable with buttons, keyboard accessible). A new step
// is added by type (talk_npc, go_to, activity, collect, minigame, party_activity…); each step is edited with the step
// schema (conditions, references picked from the client manifest / published content). `order` follows the position in
// the list. The server validates the whole quest again (step references, depends_on cycles, completion rule).
import { effective, SchemaForm, type JsonSchema } from '@eduworld/ops-ui/game-console';
import { SearchSelect } from '@eduworld/ops-ui';
import { Button, Card, Collapse, Flex, Tag, Typography } from 'antd';
import { useState, type DragEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { gameOverrides } from '../fields';
import { GenericSchemaBuilder } from '../GenericSchemaBuilder';
import { useFieldLabel } from '../labels';
import type { BuilderProps } from '../types';

type Step = Record<string, unknown> & { step_id?: string; type?: string; order?: number };

/** Step types offered first (ADMIN_PORTAL §3.3); the rest of the schema enum follows. */
export const PRIMARY_STEP_TYPES = ['talk_npc', 'go_to', 'activity', 'collect', 'minigame', 'party_activity'];

/** Renumbers `order` from the list position (1-based). */
export function renumber(steps: readonly Step[]): Step[] {
  return steps.map((s, i) => ({ ...s, order: i + 1 }));
}

/** First free `step_N` id. */
export function nextStepId(steps: readonly Step[]): string {
  const used = new Set(steps.map((s) => s.step_id));
  let n = steps.length + 1;
  while (used.has(`step_${n}`)) n++;
  return `step_${n}`;
}

export function moveStep(steps: readonly Step[], from: number, to: number): Step[] {
  const next = [...steps];
  const [x] = next.splice(from, 1);
  next.splice(to, 0, x!);
  return renumber(next);
}

export function QuestStepEditor(props: BuilderProps) {
  const { def, value, onChange, readOnly, errors } = props;
  const { t, i18n } = useTranslation();
  const label = useFieldLabel();
  const lang = i18n.language.startsWith('en') ? 'en' : 'vi';
  const [newType, setNewType] = useState<string>('talk_npc');
  const [dragFrom, setDragFrom] = useState<number | null>(null);
  const schema = def.schema!;
  const stepSchema = effective((schema.properties!.steps as JsonSchema).items, schema);
  const typeEnum = ((stepSchema.properties?.type as JsonSchema | undefined)?.enum ?? PRIMARY_STEP_TYPES) as string[];
  const types = [...PRIMARY_STEP_TYPES.filter((x) => typeEnum.includes(x)), ...typeEnum.filter((x) => !PRIMARY_STEP_TYPES.includes(x))];
  const steps = (Array.isArray(value.steps) ? value.steps : []) as Step[];
  const setSteps = (next: Step[]) => onChange({ ...value, steps: next.length ? next : undefined } as Record<string, unknown>);
  const stepErrors = Object.keys(errors).some((k) => /^\/?(payload[./])?steps\b/.test(k));
  const onDrop = (to: number) => (e: DragEvent) => {
    e.preventDefault();
    if (dragFrom !== null && dragFrom !== to) setSteps(moveStep(steps, dragFrom, to));
    setDragFrom(null);
  };

  return (
    <Flex vertical gap="middle">
      <GenericSchemaBuilder {...props} omitKeys={['steps']} />
      <section aria-label={t('gameOps.quest.steps')} data-testid="quest-steps">
        <Card
          size="small"
          title={t('gameOps.quest.stepsTitle', { count: steps.length })}
          extra={
            !readOnly && (
              <Flex gap="small" align="center">
                <SearchSelect aria-label={t('gameOps.quest.newStepType')} value={newType} onChange={(v) => setNewType(v as string)} options={types.map((x) => ({ value: x, label: t(`gameOps.quest.type.${x}`, { defaultValue: x }) }))} />
                <Button type="primary" onClick={() => setSteps(renumber([...steps, { step_id: nextStepId(steps), type: newType, title: undefined }]))}>
                  {t('gameOps.quest.addStep')}
                </Button>
              </Flex>
            )
          }
        >
          {stepErrors && <Typography.Text type="danger">{t('gameOps.quest.stepErrors')}</Typography.Text>}
          <ol aria-label={t('gameOps.quest.steps')}>
            {steps.map((s, i) => (
              <li
                key={`${s.step_id ?? i}`}
                data-testid={`quest-step-${i}`}
                draggable={!readOnly}
                onDragStart={() => setDragFrom(i)}
                onDragOver={(e) => e.preventDefault()}
                onDrop={onDrop(i)}
              >
                <Collapse
                  size="small"
                  items={[
                    {
                      key: 'step',
                      label: (
                        <Flex gap="small" align="center" wrap>
                          <Typography.Text strong>{t('gameOps.quest.stepN', { n: i + 1 })}</Typography.Text>
                          <Tag>{t(`gameOps.quest.type.${s.type}`, { defaultValue: String(s.type ?? '') })}</Tag>
                          <Typography.Text type="secondary">{String((s.title as Record<string, string> | undefined)?.[lang] ?? s.step_id ?? '')}</Typography.Text>
                        </Flex>
                      ),
                      extra: readOnly ? null : (
                        <Flex gap="small" onClick={(e) => e.stopPropagation()}>
                          <Button size="small" aria-label={t('gameOps.quest.moveUp', { n: i + 1 })} disabled={i === 0} onClick={() => setSteps(moveStep(steps, i, i - 1))}>
                            {t('gameOps.common.up')}
                          </Button>
                          <Button size="small" aria-label={t('gameOps.quest.moveDown', { n: i + 1 })} disabled={i === steps.length - 1} onClick={() => setSteps(moveStep(steps, i, i + 1))}>
                            {t('gameOps.common.down')}
                          </Button>
                          <Button size="small" danger aria-label={t('gameOps.quest.removeStep', { n: i + 1 })} onClick={() => setSteps(renumber(steps.filter((_, j) => j !== i)))}>
                            {t('gameOps.common.remove')}
                          </Button>
                        </Flex>
                      ),
                      children: (
                        <SchemaForm
                          schema={{ ...stepSchema, $defs: schema.$defs } as JsonSchema}
                          value={s}
                          basePath={['steps', i]}
                          omitKeys={['order']}
                          errors={errors}
                          readOnly={readOnly}
                          label={label}
                          renderOverride={gameOverrides('')}
                          idPrefix={`quest-step-${i}`}
                          onChange={(v) => setSteps(steps.map((x, j) => (j === i ? (v as Step) : x)))}
                        />
                      ),
                    },
                  ]}
                />
              </li>
            ))}
          </ol>
        </Card>
      </section>
    </Flex>
  );
}
