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
  const schools = useTenantSchools(tenantHeaders, tenantKey, allows(template, 'audience.school_ids'));
  const starts = Form.useWatch('starts_at', form);
  const ends = Form.useWatch('ends_at', form);
  const days = durationDays(starts, ends);
  const label = (k: string) => t(`portal.school_lms.schoolEvents.form.${k}`);
  const max = template.max_duration_days;
  return (
    <Form<EventFormValues> form={form} layout="vertical" name="school_event" onFinish={onSubmit} scrollToFirstError>
      <Typography.Title level={2}>{localizedName(template.name, i18n.language)}</Typography.Title>
      {template.description ? <Typography.Paragraph type="secondary">{localizedName(template.description, i18n.language)}</Typography.Paragraph> : null}
      <Form.Item name={['name', 'vi']} label={label('nameVi')} rules={field.rules(FIELDS.eventNameVi, true)}>
        <Input {...field.input(FIELDS.eventNameVi)} />
      </Form.Item>
      <Form.Item name={['name', 'en']} label={label('nameEn')} rules={field.rules(FIELDS.eventNameEn)}>
        <Input {...field.input(FIELDS.eventNameEn)} />
      </Form.Item>
      {allows(template, 'description') ? (
        <>
          <Form.Item name={['description', 'vi']} label={label('descriptionVi')} rules={field.rules(FIELDS.eventDescriptionVi)}>
            <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} {...field.input(FIELDS.eventDescriptionVi)} />
          </Form.Item>
          <Form.Item name={['description', 'en']} label={label('descriptionEn')} rules={field.rules(FIELDS.eventDescriptionEn)}>
            <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} {...field.input(FIELDS.eventDescriptionEn)} />
          </Form.Item>
        </>
      ) : null}
      <Flex gap="middle" wrap>
        <Form.Item name="starts_at" label={label('startsAt')} rules={[{ required: true, message: label('startsRequired') }]}>
          <DatePicker aria-label={label('startsAt')} showTime={{ format: 'HH:mm' }} format={DATE_FORMAT} needConfirm={false} />
        </Form.Item>
        <Form.Item name="ends_at" label={label('endsAt')} rules={[{ required: true, message: label('endsRequired') }]}>
          <DatePicker aria-label={label('endsAt')} showTime={{ format: 'HH:mm' }} format={DATE_FORMAT} needConfirm={false} />
        </Form.Item>
      </Flex>
      <Typography.Paragraph type="secondary" data-testid="event-duration">
        {t('portal.school_lms.schoolEvents.form.duration', { days, max })} · {label('ictHint')}
      </Typography.Paragraph>
      {allows(template, 'audience.school_ids') ? (
        <Form.Item name={['audience', 'school_ids']} label={label('schools')} extra={label('schoolsHint')}>
          <SearchSelect<string[]>
            mode="multiple"
            aria-label={label('schools')}
            loading={schools.isFetching}
            optionFilterProp="label"
            options={schools.items.map((s) => ({ value: s.school_id, label: localizedName(s.name, i18n.language) }))}
          />
        </Form.Item>
      ) : null}
      {allows(template, 'audience.grade_levels') ? (
        <Form.Item name={['audience', 'grade_levels']} label={label('grades')} extra={label('gradesHint')}>
          <Checkbox.Group aria-label={label('grades')} options={EVENT_GRADES.map((g) => ({ value: g, label: t('portal.school_lms.schoolEvents.form.grade', { grade: g }) }))} />
        </Form.Item>
      ) : null}
      {allows(template, 'community_goal_target') ? (
        <Form.Item name="community_goal_target" label={label('goalTarget')} rules={[{ type: 'number', min: LIMITS.eventGoalTarget.minimum, message: label('goalTargetMin') }]}>
          <InputNumber aria-label={label('goalTarget')} min={LIMITS.eventGoalTarget.minimum} precision={0} />
        </Form.Item>
      ) : null}
      {allows(template, 'quest_ids') ? (
        <Form.Item name="quest_ids" label={label('quests')} rules={[{ type: 'array', max: LIMITS.eventQuests.maxItems }]}>
          <SearchSelect<string[]>
            mode="multiple"
            aria-label={label('quests')}
            optionFilterProp="label"
            options={(template.quest_option_items ?? []).map((q) => ({ value: q.id, label: optionLabel(localizedName(q.title, i18n.language), q.grade_levels, t) }))}
          />
        </Form.Item>
      ) : null}
      {allows(template, 'challenge_ids') ? (
        <Form.Item name="challenge_ids" label={label('challenges')} rules={[{ type: 'array', max: LIMITS.eventChallenges.maxItems }]}>
          <SearchSelect<string[]>
            mode="multiple"
            aria-label={label('challenges')}
            optionFilterProp="label"
            options={(template.challenge_option_items ?? []).map((q) => ({ value: q.id, label: optionLabel(localizedName(q.title, i18n.language), q.grade_levels, t) }))}
          />
        </Form.Item>
      ) : null}
      {allows(template, 'reward_option_id') && (template.reward_options ?? []).length ? (
        <Form.Item name="reward_option_id" label={label('reward')}>
          <Radio.Group aria-label={label('reward')}>
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
        <Alert type="info" showIcon message={label('rulesFromTemplate')} />
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
  if (allows(template, 'audience.school_ids') && v.audience?.school_ids?.length) audience.school_ids = v.audience.school_ids;
  if (allows(template, 'audience.grade_levels') && v.audience?.grade_levels?.length) audience.grade_levels = [...v.audience.grade_levels].sort((a, b) => a - b);
  out.audience = audience;
  if (allows(template, 'description') && v.description?.vi?.trim()) {
    out.description = { vi: v.description.vi.trim(), ...(v.description.en?.trim() ? { en: v.description.en.trim() } : {}) };
  }
  if (allows(template, 'community_goal_target') && v.community_goal_target != null) out.community_goal_target = v.community_goal_target;
  if (allows(template, 'quest_ids') && v.quest_ids?.length) out.quest_ids = v.quest_ids;
  if (allows(template, 'challenge_ids') && v.challenge_ids?.length) out.challenge_ids = v.challenge_ids;
  if (allows(template, 'reward_option_id') && v.reward_option_id) out.reward_option_id = v.reward_option_id;
  return out;
}
