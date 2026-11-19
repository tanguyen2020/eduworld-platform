// CharCounter (F050-AC-09): "count/max" next to a text field limited by OpenAPI `maxLength` (ADR-0041),
// announced politely to screen readers.
import { Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';
import type { OpenapiStringSchema } from './openapiRules';

export interface CharCounterProps {
  count: number;
  max: number;
}

export function CharCounter({ count, max }: CharCounterProps) {
  const { t } = useTranslation(OPS_UI_NS);
  return (
    <Typography.Text type={count > max ? 'danger' : 'secondary'} aria-live="polite" aria-label={t('charCounter.label', { count, max })}>
      {`${count}/${max}`}
    </Typography.Text>
  );
}

/** Props for antd Input/TextArea: hard cap + counter rendered by CharCounter. */
export function openapiInputProps(schema: OpenapiStringSchema) {
  if (typeof schema.maxLength !== 'number') return {};
  const max = schema.maxLength;
  return {
    maxLength: max,
    showCount: { formatter: ({ count }: { count: number }) => <CharCounter count={count} max={max} /> },
  };
}
