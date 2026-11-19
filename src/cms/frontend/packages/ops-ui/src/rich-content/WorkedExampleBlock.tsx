// WorkedExampleBlock (F053-AC-31, FR-CNT-020, CONTENT_MODEL §9.2): step-by-step example of a lesson / learning
// material — problem, 1–8 steps (rich content + optional `audio_ref`, required for grades 1–3 per §8.1), optional
// self-explanation prompt, and an `id` unique in the version (questions point to it with `worked_example_ref`).
import { Button, Card, Flex, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import type { ContentLang } from '../i18n-field/translationStatus';
import { OPS_UI_NS } from '../i18n';
import { RichContentEditor, type MediaSlotArgs } from './RichContentEditor';
import { emptyRichContent, WORKED_EXAMPLE_STEPS_MAX, WORKED_EXAMPLE_STEPS_MIN, type WorkedExample } from './types';

export type WorkedExampleIssue = 'STEPS_MIN' | 'STEPS_MAX' | 'STEP_AUDIO_REQUIRED' | 'ID_DUPLICATE' | 'ID_INVALID' | 'PROBLEM_REQUIRED';

/** Block ids: `we_` + base36 random (question.schema.json `worked_example_ref.block_id` pattern ^we_[a-z0-9_]{1,40}$). */
export const WORKED_EXAMPLE_ID = /^we_[a-z0-9_]{1,40}$/;

export function newWorkedExampleId(existing: readonly string[] = [], random: () => number = Math.random): string {
  const base = `we_${Math.floor(random() * 36 ** 6).toString(36).padStart(6, '0')}`;
  if (!existing.includes(base)) return base;
  // collision (or a fixed random source): add a counter until free
  for (let n = 1; ; n++) if (!existing.includes(`${base}_${n}`)) return `${base}_${n}`;
}

export function newWorkedExample(existingIds: readonly string[] = []): WorkedExample {
  return { type: 'worked_example', id: newWorkedExampleId(existingIds), problem: emptyRichContent(), steps: [{ content: emptyRichContent(), audio_ref: null }] };
}

/** Client checks of a block (the server checks again, F024). `otherIds`: ids of the other blocks of the version. */
export function validateWorkedExample(we: WorkedExample, opts: { requireAudio: boolean; otherIds?: readonly string[] }): { issue: WorkedExampleIssue; step?: number }[] {
  const out: { issue: WorkedExampleIssue; step?: number }[] = [];
  if (!WORKED_EXAMPLE_ID.test(we.id)) out.push({ issue: 'ID_INVALID' });
  if (opts.otherIds?.includes(we.id)) out.push({ issue: 'ID_DUPLICATE' });
  if (!we.problem.blocks.some((b) => b.text || b.latex || b.media_ref)) out.push({ issue: 'PROBLEM_REQUIRED' });
  if (we.steps.length < WORKED_EXAMPLE_STEPS_MIN) out.push({ issue: 'STEPS_MIN' });
  if (we.steps.length > WORKED_EXAMPLE_STEPS_MAX) out.push({ issue: 'STEPS_MAX' });
  if (opts.requireAudio) we.steps.forEach((s, i) => !s.audio_ref && out.push({ issue: 'STEP_AUDIO_REQUIRED', step: i }));
  return out;
}

const AUDIO = 'audio';

export interface WorkedExampleBlockProps {
  value: WorkedExample;
  onChange: (value: WorkedExample) => void;
  contentLanguage: ContentLang;
  /** grade_levels ∩ {1,2,3} ≠ ∅ (CONTENT_MODEL §8.1). */
  requireAudio: boolean;
  otherIds?: readonly string[];
  readOnly?: boolean;
  mediaSlot?: (args: MediaSlotArgs) => ReactNode;
}

export function WorkedExampleBlock({ value, onChange, contentLanguage, requireAudio, otherIds, readOnly = false, mediaSlot }: WorkedExampleBlockProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const issues = validateWorkedExample(value, { requireAudio, otherIds });
  const base = `worked_example.${value.id}`;
  const setStep = (i: number, patch: Partial<WorkedExample['steps'][number]>) => onChange({ ...value, steps: value.steps.map((s, j) => (j === i ? { ...s, ...patch } : s)) });
  return (
    <Card title={t('workedExample.title')} extra={<Typography.Text code>{value.id}</Typography.Text>} data-field={base}>
      <Flex vertical gap="middle">
        <RichContentEditor id={`${base}.problem`} label={t('workedExample.problem')} value={value.problem} onChange={(problem) => onChange({ ...value, problem })} contentLanguage={contentLanguage} readOnly={readOnly} mediaSlot={mediaSlot} required />
        {value.steps.map((s, i) => (
          <Card
            key={i}
            size="small"
            title={t('workedExample.step', { n: i + 1 })}
            extra={
              !readOnly && (
                <Button size="small" danger disabled={value.steps.length <= WORKED_EXAMPLE_STEPS_MIN} onClick={() => onChange({ ...value, steps: value.steps.filter((_, j) => j !== i) })}>
                  {t('workedExample.removeStep')}
                </Button>
              )
            }
          >
            <Flex vertical gap="small">
              <RichContentEditor id={`${base}.steps.${i}.content`} label={t('workedExample.stepContent')} value={s.content} onChange={(content) => setStep(i, { content })} contentLanguage={contentLanguage} readOnly={readOnly} mediaSlot={mediaSlot} required />
              <Typography.Text>{requireAudio ? t('workedExample.audioRequired') : t('workedExample.audioOptional')}</Typography.Text>
              {mediaSlot?.({ kind: AUDIO, value: s.audio_ref ?? undefined, onChange: (v) => setStep(i, { audio_ref: v ?? null }), id: `${base}.steps.${i}.audio_ref` })}
            </Flex>
          </Card>
        ))}
        {!readOnly && (
          <Button disabled={value.steps.length >= WORKED_EXAMPLE_STEPS_MAX} onClick={() => onChange({ ...value, steps: [...value.steps, { content: emptyRichContent(), audio_ref: null }] })}>
            {t('workedExample.addStep', { max: WORKED_EXAMPLE_STEPS_MAX })}
          </Button>
        )}
        <RichContentEditor
          id={`${base}.self_explain_prompt`}
          label={t('workedExample.selfExplain')}
          value={value.self_explain_prompt}
          onChange={(p) => onChange({ ...value, self_explain_prompt: p })}
          contentLanguage={contentLanguage}
          readOnly={readOnly}
          allowedBlocks={['paragraph', 'math']}
        />
        {issues.length > 0 && (
          <ul aria-label={t('workedExample.issues')}>
            {issues.map((x) => (
              <li key={`${x.issue}:${x.step ?? ''}`}>
                <Typography.Text type="danger">{t(`workedExample.issue.${x.issue}`, { n: (x.step ?? 0) + 1, min: WORKED_EXAMPLE_STEPS_MIN, max: WORKED_EXAMPLE_STEPS_MAX })}</Typography.Text>
              </li>
            ))}
          </ul>
        )}
      </Flex>
    </Card>
  );
}
