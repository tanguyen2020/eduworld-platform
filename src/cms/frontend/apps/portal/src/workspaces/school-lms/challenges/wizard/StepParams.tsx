// Step 2 "Thông tin" (F108 T028; F108-AC-14): name, schedule and one input per key of the template's `allowed_params`
// — and nothing else (no rule, reward or chat input; any other key would be refused by the server with 422). Keys named
// in the ChallengeCreate.params contract get a typed input; any other allowed key a text box limited to 255 characters.
// Limits come from the contract (challengeForm). Presentational only (AGENTS R1).
import { DatePicker, Form, Input, InputNumber, Switch, Typography } from 'antd';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';
import { useTenantSubjects } from '../../lms/api/targets';
import { localizedName } from '../../common/format';
import { CHALLENGE_ENUMS, CHALLENGE_FIELDS, PARAM_STRING_MAX, paramInput, type ChallengeField, type ChallengeFormValues, type ChallengeTemplate, type ParamValue } from '../model/challengeForm';
import { SearchSelect } from '@eduworld/ops-ui';

type Patch = (p: Partial<ChallengeFormValues>) => void;
export type FieldErrors = Partial<Record<ChallengeField, string>>;

const status = (e: string | undefined) => (e ? ('error' as const) : undefined);

function SubjectInput({ label, value, onChange }: { label: string; value: ParamValue; onChange: (v: ParamValue) => void }) {
  const { i18n } = useTranslation();
  const subjects = useTenantSubjects();
  return (
    <SearchSelect
      allowClear
      showSearch
      optionFilterProp="label"
      aria-label={label}
      placeholder={label}
      loading={subjects.isFetching}
      value={typeof value === 'string' ? value : undefined}
      onChange={(v?: string) => onChange(v)}
      options={(subjects.data ?? []).map((s) => ({ value: s.subject_id, label: localizedName(s.name as { vi: string; en?: string }, i18n.language) }))}
    />
  );
}

function ParamField({ name, value, onChange, error }: { name: string; value: ParamValue; onChange: (v: ParamValue) => void; error?: string }) {
  const { t } = useTranslation();
  const label = t(`lms.challenges.params.${name}`, { defaultValue: name });
  const id = `challenge-param-${name}`;
  const kind = paramInput(name);
  let input;
  switch (kind) {
    case 'integer':
      input = <InputNumber id={id} min={1} precision={0} value={typeof value === 'number' ? value : null} onChange={(n) => onChange(n ?? undefined)} />;
      break;
    case 'boolean':
      input = <Switch id={id} aria-label={label} checked={value === true} onChange={(c) => onChange(c)} />;
      break;
    case 'datetime':
      input = <DatePicker id={id} showTime aria-label={label} value={typeof value === 'string' && value ? dayjs(value) : null} onChange={(d) => onChange(d ? d.toISOString() : undefined)} />;
      break;
    case 'seeding_policy':
      input = (
        <SearchSelect
          allowClear
          aria-label={label}
          placeholder={label}
          value={typeof value === 'string' ? value : undefined}
          onChange={(v?: string) => onChange(v)}
          options={CHALLENGE_ENUMS.seedingPolicy.values.map((s) => ({ value: s, label: t(`lms.challenges.seedingPolicy.${s}`) }))}
        />
      );
      break;
    case 'subject':
      input = <SubjectInput label={label} value={value} onChange={onChange} />;
      break;
    default:
      input = <Input id={id} value={typeof value === 'string' ? value : ''} onChange={(e) => onChange(e.target.value)} maxLength={PARAM_STRING_MAX} showCount />;
  }
  return (
    <Form.Item label={label} htmlFor={kind === 'seeding_policy' || kind === 'subject' ? undefined : id} validateStatus={status(error)} help={error}>
      {input}
    </Form.Item>
  );
}

export function StepParams({ v, set, template, errors }: { v: ChallengeFormValues; set: Patch; template: ChallengeTemplate | undefined; errors: FieldErrors }) {
  const { t } = useTranslation();
  const allowed = template?.allowed_params ?? [];
  return (
    <>
      <Form.Item label={t('lms.challenges.wizard.name')} htmlFor="challenge-name" required validateStatus={status(errors.name)} help={errors.name}>
        <Input id="challenge-name" value={v.name} onChange={(e) => set({ name: e.target.value })} showCount maxLength={CHALLENGE_FIELDS.name.maxLength} />
      </Form.Item>
      <Form.Item label={t('lms.challenges.wizard.startsAt')} required validateStatus={status(errors.startsAt)} help={errors.startsAt}>
        <DatePicker showTime aria-label={t('lms.challenges.wizard.startsAt')} value={v.startsAt ? dayjs(v.startsAt) : null} onChange={(d) => set({ startsAt: d ? d.toISOString() : undefined })} />
      </Form.Item>
      <Form.Item label={t('lms.challenges.wizard.endsAt')} validateStatus={status(errors.endsAt)} help={errors.endsAt}>
        <DatePicker showTime aria-label={t('lms.challenges.wizard.endsAt')} value={v.endsAt ? dayjs(v.endsAt) : null} onChange={(d) => set({ endsAt: d ? d.toISOString() : undefined })} />
      </Form.Item>
      <Typography.Title level={4}>{t('lms.challenges.wizard.params')}</Typography.Title>
      {allowed.length ? (
        allowed.map((name) => <ParamField key={name} name={name} value={v.params[name]} error={errors[`params.${name}`]} onChange={(value) => set({ params: { ...v.params, [name]: value } })} />)
      ) : (
        <Typography.Text type="secondary">{t('lms.challenges.wizard.noParams')}</Typography.Text>
      )}
    </>
  );
}
