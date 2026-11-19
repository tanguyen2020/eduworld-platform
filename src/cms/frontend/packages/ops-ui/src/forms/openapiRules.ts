// Form rules from an OpenAPI schema (F050-AC-09, ADR-0041, DESIGN_SYSTEM §4.3): `required`, `maxLength`,
// `minLength`, `pattern` → Ant Design Form rules with translated messages; `maxLength` also caps the input and
// shows the character counter, so typing beyond the limit is blocked.
import type { FormRule } from 'antd';

export interface OpenapiStringSchema {
  type?: string | string[];
  maxLength?: number;
  minLength?: number;
  pattern?: string;
  format?: string;
}

export type Translate = (key: string, options?: Record<string, unknown>) => string;

export interface OpenapiRuleOptions {
  /** Field listed in the operation's `required` array. */
  required?: boolean;
  /** Translator bound to the `ops-ui` namespace (keys `field.*`). */
  t: Translate;
}

export function openapiRules(schema: OpenapiStringSchema, { required = false, t }: OpenapiRuleOptions): FormRule[] {
  const rules: FormRule[] = [];
  if (required) rules.push({ required: true, whitespace: true, message: t('field.required') });
  if (typeof schema.maxLength === 'number') {
    rules.push({ type: 'string', max: schema.maxLength, message: t('field.maxLength', { max: schema.maxLength }) });
  }
  if (typeof schema.minLength === 'number' && schema.minLength > 0) {
    rules.push({ type: 'string', min: schema.minLength, message: t('field.minLength', { min: schema.minLength }) });
  }
  if (typeof schema.pattern === 'string') {
    rules.push({ pattern: new RegExp(schema.pattern, 'u'), message: t('field.pattern') });
  }
  return rules;
}
