// Small form pieces of the research screens (F035 US8): UUID inputs, number inputs bounded by the OpenAPI limits,
// status tags. Presentational only — the backend validates every field (422 VALIDATION_FAILED mapped to the fields).
import { Form, InputNumber, Input, Tag } from 'antd';
import { useTranslation } from 'react-i18next';
import type { FieldLimit } from '../limits';
import { SearchSelect } from '@eduworld/ops-ui';

export const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** One UUID (class, subject, blueprint version…). */
export function UuidField({ name, label, required = true }: { name: string | (string | number)[]; label: string; required?: boolean }) {
  const { t } = useTranslation();
  return (
    <Form.Item name={name} label={label} rules={[{ required, message: t('platform.research.form.required') }, { pattern: UUID_PATTERN, message: t('platform.research.form.uuid') }]}>
      <Input aria-label={label} maxLength={36} />
    </Form.Item>
  );
}

/** Several UUIDs typed or pasted as tags (objectives, students, excluded questions). */
export function UuidListField({ name, label, limit, required = false }: { name: string; label: string; limit: FieldLimit; required?: boolean }) {
  const { t } = useTranslation();
  return (
    <Form.Item
      name={name}
      label={label}
      rules={[
        { required, message: t('platform.research.form.required') },
        { validator: (_, v?: string[]) => ((v ?? []).every((x) => UUID_PATTERN.test(x)) ? Promise.resolve() : Promise.reject(new Error(t('platform.research.form.uuid')))) },
      ]}
    >
      <SearchSelect mode="tags" open={false} aria-label={label} maxCount={limit.maxItems} tokenSeparators={[',', ' ', '\n']} />
    </Form.Item>
  );
}

/** Integer bounded by an OpenAPI minimum/maximum. */
export function BoundedNumberField({ name, label, limit, required = true }: { name: string | (string | number)[]; label: string; limit: FieldLimit; required?: boolean }) {
  const { t } = useTranslation();
  return (
    <Form.Item name={name} label={label} rules={[{ required, message: t('platform.research.form.required') }]}>
      <InputNumber aria-label={label} min={limit.minimum} max={limit.maximum} precision={0} />
    </Form.Item>
  );
}

const STATUS_COLORS: Record<string, string> = {
  active: 'processing',
  live: 'processing',
  shadow: 'default',
  draft: 'default',
  completed: 'success',
  succeeded: 'success',
  open: 'processing',
  scheduled: 'default',
  closed: 'success',
  stopped: 'warning',
  revoked: 'warning',
  withdrawn: 'warning',
  cancelled: 'warning',
  failed: 'error',
  enrolled: 'success',
};

/** Translated status tag with a semantic color of the Admin theme. */
export function StatusTag({ status }: { status: string }) {
  const { t } = useTranslation();
  return <Tag color={STATUS_COLORS[status] ?? 'default'}>{t(`platform.research.status.${status}`)}</Tag>;
}
