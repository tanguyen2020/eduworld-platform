// Form of a school event (create and edit — F081-AC-02, AC-05; T011 AllowedParamField, AudiencePicker,
// TemplateOptionPicker): only the parameters the template allows (`allowed_params`) are rendered; quests, challenges and
// the reward come from the template lists (`quest_option_items`, `challenge_option_items`, `reward_options`); the
// audience is schools of the caller's scope (listTenantSchools — every page) and grades 1–9 (no class, ADR-0095). There
// is no input for rules, scoring or rewards outside the template. Name `vi` required, `en` optional, every text limited
// by the OpenAPI maxLength with a counter; dates picked in ICT; the template maximum duration is shown. The server makes
// every final check (AGENTS R1).
import { SearchSelect } from '@eduworld/ops-ui';
import { Alert, Button, Checkbox, DatePicker, Flex, Form, Input, InputNumber, Radio, Typography, type FormInstance } from 'antd';
import type { Dayjs } from 'dayjs';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { FIELDS, LIMITS } from '../common/fieldSchemas';
import { localizedName } from '../common/format';
import { useFieldRules } from '../common/ui';
import { useTenantSchools, type EventTemplate } from './api';
import { durationDays } from './ui';

/** Grades a school event may target (SchoolEventCreate.audience.grade_levels: 1–9; the template has no grade list). */
export const EVENT_GRADES = [1, 2, 3, 4, 5, 6, 7, 8, 9];

/** Values of the antd form (dates as ICT wall times). */
export interface EventFormValues {
  name: { vi: string; en?: string };
  description?: { vi?: string; en?: string };
  starts_at?: Dayjs | null;
  ends_at?: Dayjs | null;
  audience?: { school_ids?: string[]; grade_levels?: number[] };
  community_goal_target?: number | null;
  quest_ids?: string[];
  challenge_ids?: string[];
  reward_option_id?: string;
}

/** Parameters of the template the form renders (allowed_params). */
export function allows(template: Pick<EventTemplate, 'allowed_params'> | null | undefined, param: string): boolean {
  return !!template?.allowed_params.includes(param);
}

const DATE_FORMAT = 'DD/MM/YYYY HH:mm';

/** Codes of allowed_params (EVENT_SEASON_SYSTEM §10). */
const P = { description: 'description', schools: 'audience.school_ids', grades: 'audience.grade_levels', goalTarget: 'community_goal_target', quests: 'quest_ids', challenges: 'challenge_ids', reward: 'reward_option_id' } as const;

export interface SchoolEventFormProps {
  form: FormInstance<EventFormValues>;
  template: EventTemplate;
  tenantHeaders: Record<string, string>;
  tenantKey: string;
  submitLabel: string;
  submitting: boolean;
  onSubmit: (values: EventFormValues) => void;
  onBack?: () => void;
  backLabel?: string;
  /** Banner of a write error that is not on a field (code + trace_id). */
  banner?: ReactNode;
}

export function SchoolEventForm({ form, template, tenantHeaders, tenantKey, submitLabel, submitting, onSubmit, onBack, backLabel, banner }: SchoolEventFormProps) {
  const { t, i18n } = useTranslation();
  const field = useFieldRules();
  const schools = useTenantSchools(tenantHeaders, tenantKey, allows(template, P.schools));
  const starts = Form.useWatch('starts_at', form);
  const ends = Form.useWatch('ends_at', form);
  const days = durationDays(starts, ends);
  const max = template.max_duration_days;
  return (
    <Form<EventFormValues> form={form} layout="vertical" name="school_event" onFinish={onSubmit} scrollToFirstError>
      <Typography.Title level={2}>{localizedName(template.name, i18n.language)}</Typography.Title>
      {template.description ? <Typography.Paragraph type="secondary">{localizedName(template.description, i18n.language)}</Typography.Paragraph> : null}
      <Form.Item name={['name', 'vi']} label={t('portal.school_lms.schoolEvents.form.nameVi')} rules={field.rules(FIELDS.eventNameVi, true)}>
        <Input {...field.input(FIELDS.eventNameVi)} />
      </Form.Item>
      <Form.Item name={['name', 'en']} label={t('portal.school_lms.schoolEvents.form.nameEn')} rules={field.rules(FIELDS.eventNameEn)}>
        <Input {...field.input(FIELDS.eventNameEn)} />
      </Form.Item>
      {allows(template, P.description) ? (
        <>
          <Form.Item name={['description', 'vi']} label={t('portal.school_lms.schoolEvents.form.descriptionVi')} rules={field.rules(FIELDS.eventDescriptionVi)}>
            <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} {...field.input(FIELDS.eventDescriptionVi)} />
          </Form.Item>
          <Form.Item name={['description', 'en']} label={t('portal.school_lms.schoolEvents.form.descriptionEn')} rules={field.rules(FIELDS.eventDescriptionEn)}>
            <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} {...field.input(FIELDS.eventDescriptionEn)} />
          </Form.Item>
        </>
      ) : null}
      <Flex gap="middle" wrap>
        <Form.Item name="starts_at" label={t('portal.school_lms.schoolEvents.form.startsAt')} rules={[{ required: true, message: t('portal.school_lms.schoolEvents.form.startsRequired') }]}>
          <DatePicker aria-label={t('portal.school_lms.schoolEvents.form.startsAt')} showTime={{ format: 'HH:mm' }} format={DATE_FORMAT} needConfirm={false} />
        </Form.Item>
        <Form.Item name="ends_at" label={t('portal.school_lms.schoolEvents.form.endsAt')} rules={[{ required: true, message: t('portal.school_lms.schoolEvents.form.endsRequired') }]}>
          <DatePicker aria-label={t('portal.school_lms.schoolEvents.form.endsAt')} showTime={{ format: 'HH:mm' }} format={DATE_FORMAT} needConfirm={false} />
        </Form.Item>
      </Flex>
      <Typography.Paragraph type="secondary" data-testid="event-duration">
        {t('portal.school_lms.schoolEvents.form.duration', { days, max })} · {t('portal.school_lms.schoolEvents.form.ictHint')}
      </Typography.Paragraph>
      {allows(template, P.schools) ? (
        <Form.Item name={['audience', 'school_ids']} label={t('portal.school_lms.schoolEvents.form.schools')} extra={t('portal.school_lms.schoolEvents.form.schoolsHint')}>
          <SearchSelect<string[]>
            mode="multiple"
            aria-label={t('portal.school_lms.schoolEvents.form.schools')}
            loading={schools.isFetching}
            optionFilterProp="label"
            options={schools.items.map((s) => ({ value: s.school_id, label: localizedName(s.name, i18n.language) }))}
          />
        </Form.Item>
      ) : null}
      {allows(template, P.grades) ? (
        <Form.Item name={['audience', 'grade_levels']} label={t('portal.school_lms.schoolEvents.form.grades')} extra={t('portal.school_lms.schoolEvents.form.gradesHint')}>
          <Checkbox.Group aria-label={t('portal.school_lms.schoolEvents.form.grades')} options={EVENT_GRADES.map((g) => ({ value: g, label: t('portal.school_lms.schoolEvents.form.grade', { grade: g }) }))} />
        </Form.Item>
      ) : null}
      {allows(template, P.goalTarget) ? (
        <Form.Item name="community_goal_target" label={t('portal.school_lms.schoolEvents.form.goalTarget')} rules={[{ type: 'number', min: LIMITS.eventGoalTarget.minimum, message: t('portal.school_lms.schoolEvents.form.goalTargetMin') }]}>
          <InputNumber aria-label={t('portal.school_lms.schoolEvents.form.goalTarget')} min={LIMITS.eventGoalTarget.minimum} precision={0} />
        </Form.Item>
      ) : null}
      {allows(template, P.quests) ? (
        <Form.Item name="quest_ids" label={t('portal.school_lms.schoolEvents.form.quests')} rules={[{ type: 'array', max: LIMITS.eventQuests.maxItems }]}>
          <SearchSelect<string[]>
            mode="multiple"
            aria-label={t('portal.school_lms.schoolEvents.form.quests')}
            optionFilterProp="label"
            options={(template.quest_option_items ?? []).map((q) => ({ value: q.id, label: optionLabel(localizedName(q.title, i18n.language), q.grade_levels, t) }))}
          />
        </Form.Item>
      ) : null}
      {allows(template, P.challenges) ? (
        <Form.Item name="challenge_ids" label={t('portal.school_lms.schoolEvents.form.challenges')} rules={[{ type: 'array', max: LIMITS.eventChallenges.maxItems }]}>
          <SearchSelect<string[]>
            mode="multiple"
            aria-label={t('portal.school_lms.schoolEvents.form.challenges')}
            optionFilterProp="label"
            options={(template.challenge_option_items ?? []).map((q) => ({ value: q.id, label: optionLabel(localizedName(q.title, i18n.language), q.grade_levels, t) }))}
          />
        </Form.Item>
      ) : null}
      {allows(template, P.reward) && (template.reward_options ?? []).length ? (
        <Form.Item name="reward_option_id" label={t('portal.school_lms.schoolEvents.form.reward')}>
          <Radio.Group aria-label={t('portal.school_lms.schoolEvents.form.reward')}>
            <Flex vertical>
              {(template.reward_options ?? []).map((o) => (
                <Radio key={o.reward_option_id} value={o.reward_option_id}>
                  {o.label ? localizedName(o.label, i18n.language) : o.reward_option_id}
                </Radio>
              ))}
            </Flex>
          </Radio.Group>
        </Form.Item>
      ) : null}
      <Form.Item>
        <Alert type="info" showIcon message={t('portal.school_lms.schoolEvents.form.rulesFromTemplate')} />
      </Form.Item>
      {banner}
      <Form.Item>
        <Flex gap="small">
          <Button type="primary" htmlType="submit" loading={submitting}>
            {submitLabel}
          </Button>
          {onBack ? <Button onClick={onBack}>{backLabel}</Button> : null}
        </Flex>
      </Form.Item>
    </Form>
  );
}

function optionLabel(title: string, grades: number[], t: (k: string, o?: Record<string, unknown>) => string) {
  return grades.length ? `${title} (${t('portal.school_lms.schoolEvents.audience.grades', { list: grades.join(', ') })})` : title;
}

/** Form values → the parameters of SchoolEventCreate/Update the template allows (dates in UTC). */
export function eventParams(v: EventFormValues, template: Pick<EventTemplate, 'allowed_params'>, toIso: (d: Dayjs) => string) {
  const name = { vi: v.name.vi.trim(), ...(v.name.en?.trim() ? { en: v.name.en.trim() } : {}) };
  const out: Record<string, unknown> = { name };
  if (v.starts_at) out.starts_at = toIso(v.starts_at);
  if (v.ends_at) out.ends_at = toIso(v.ends_at);
  const audience: Record<string, unknown> = {};
  if (allows(template, P.schools) && v.audience?.school_ids?.length) audience.school_ids = v.audience.school_ids;
  if (allows(template, P.grades) && v.audience?.grade_levels?.length) audience.grade_levels = [...v.audience.grade_levels].sort((a, b) => a - b);
  out.audience = audience;
  if (allows(template, P.description) && v.description?.vi?.trim()) {
    out.description = { vi: v.description.vi.trim(), ...(v.description.en?.trim() ? { en: v.description.en.trim() } : {}) };
  }
  if (allows(template, P.goalTarget) && v.community_goal_target != null) out.community_goal_target = v.community_goal_target;
  if (allows(template, P.quests) && v.quest_ids?.length) out.quest_ids = v.quest_ids;
  if (allows(template, P.challenges) && v.challenge_ids?.length) out.challenge_ids = v.challenge_ids;
  if (allows(template, P.reward) && v.reward_option_id) out.reward_option_id = v.reward_option_id;
  return out;
}
