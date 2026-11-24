// EventWizard (F079-AC-12, AC-13, FR-EVT-002, ADMIN_PORTAL §3.15, EVENT_SEASON_SYSTEM §4, §11): the content steps of the
// Event Builder — 1 Thông tin song ngữ (vi/en side by side with the translation status of each text), 2 Đối tượng
// (scope, tenants, schools, grades, subjects), 3 Nội dung (quests, challenges, shop items, collection, community goal,
// decoration — references only from PUBLISHED content), 4 Phần thưởng (event points, milestone rewards from the
// catalog; the healthy-play reminder "mốc không cần > 3 phiên/tuần" is shown and never blocks), 5 Lịch (ICT). Server
// errors are mapped to their field and the step holding them is marked and blocked (TRANSLATION_INCOMPLETE, festival
// shorter than `event_festival_min_duration_days`, invalid reward, decoration bundle over `event_decoration_bundle_max_mb`).
// Steps 6–8 (preview, submit, schedule) are the create flow (EventCreatePage) or the editor status bar.
import { SchemaForm } from '@eduworld/ops-ui/game-console';
import { Alert, Button, Flex, Steps } from 'antd';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { canonicalPath } from '@eduworld/ops-ui/game-console';
import { gameOverrides } from '../fields';
import { useFieldLabel } from '../labels';
import type { BuilderProps } from '../types';

/** Root payload fields edited in each content step (event.schema.json). */
export const EVENT_STEP_FIELDS: readonly (readonly string[])[] = [
  ['code', 'event_kind', 'name', 'description', 'short_tagline', 'theme_code', 'banner_asset_id', 'template_ref'],
  ['audience', 'eligibility'],
  ['components'],
  ['event_points', 'participation_rewards', 'milestone_rewards'],
  ['schedule'],
];

/** Index of the content step holding a field path (server error), -1 when none. */
export function stepOfField(field: string): number {
  const root = canonicalPath(field).split('/')[0] ?? '';
  return EVENT_STEP_FIELDS.findIndex((keys) => keys.includes(root));
}

/** Steps (0–4) holding at least one server error. */
export function stepsWithErrors(errors: Record<string, string>): Set<number> {
  return new Set(Object.keys(errors).map(stepOfField).filter((i) => i >= 0));
}

export interface EventWizardProps extends BuilderProps {
  /** Extra steps after the 5 content steps (create flow: preview, submit, schedule). */
  extraSteps?: { title: string; content: ReactNode }[];
  step?: number;
  onStepChange?: (step: number) => void;
  /** Called before leaving the last content step (create flow saves the draft). */
  onBeforeExtra?: () => Promise<boolean>;
}

export function EventWizard({ def, value, onChange, readOnly, errors, extraSteps = [], step: stepProp, onStepChange, onBeforeExtra }: EventWizardProps) {
  const { t } = useTranslation();
  const label = useFieldLabel();
  const [own, setOwn] = useState(0);
  const step = stepProp ?? own;
  const setStep = (s: number) => (onStepChange ? onStepChange(s) : setOwn(s));
  const bad = stepsWithErrors(errors);
  const titles = ['info', 'audience', 'content', 'rewards', 'schedule'].map((k) => t(`gameOps.event.step.${k}`));
  const all = [...titles.map((title) => ({ title })), ...extraSteps.map((s) => ({ title: s.title }))];
  const allKeys = EVENT_STEP_FIELDS.flat();
  const goNext = async () => {
    if (step === EVENT_STEP_FIELDS.length - 1 && extraSteps.length && onBeforeExtra && !(await onBeforeExtra())) return;
    setStep(Math.min(step + 1, all.length - 1));
  };
  return (
    <Flex vertical gap="middle" data-testid="event-wizard">
      <Steps
        size="small"
        current={step}
        onChange={(s) => (s < EVENT_STEP_FIELDS.length || !extraSteps.length ? setStep(s) : undefined)}
        items={all.map((s, i) => ({ title: s.title, status: bad.has(i) ? 'error' : undefined }))}
      />
      {bad.size > 0 && <Alert type="error" showIcon role="alert" message={t('gameOps.event.stepsBlocked', { steps: [...bad].map((i) => titles[i]).join(', ') })} />}
      {step < EVENT_STEP_FIELDS.length ? (
        <section aria-label={titles[step]} data-testid={`event-step-${step}`}>
          {step === 3 && <Alert type="info" showIcon message={t('gameOps.event.healthyHint')} data-testid="event-healthy-hint" />}
          {step === 2 && <Alert type="info" showIcon message={t('gameOps.event.publishedOnly')} />}
          <SchemaForm
            schema={def.schema!}
            value={value}
            onChange={(v) => onChange(v as Record<string, unknown>)}
            label={label}
            managedKeys={def.managedKeys}
            omitKeys={allKeys.filter((k) => !EVENT_STEP_FIELDS[step]!.includes(k)).concat(Object.keys(def.schema!.properties ?? {}).filter((k) => !allKeys.includes(k)))}
            errors={errors}
            readOnly={readOnly}
            renderOverride={gameOverrides(def.idField)}
            idPrefix={`event-step-${step}`}
          />
        </section>
      ) : (
        extraSteps[step - EVENT_STEP_FIELDS.length]?.content
      )}
      <Flex gap="small">
        <Button disabled={step === 0} onClick={() => setStep(step - 1)}>
          {t('gameOps.common.back')}
        </Button>
        {step < all.length - 1 && (
          <Button type="primary" onClick={() => void goNext()}>
            {t('gameOps.common.next')}
          </Button>
        )}
      </Flex>
    </Flex>
  );
}
