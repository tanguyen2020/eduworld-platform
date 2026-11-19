// mapValidationErrors (F051 T050, API_DESIGN §4): `422 VALIDATION_FAILED` `errors[].field` → Ant Design form field
// errors (`form.setFields`). The message comes from i18n by the field error `code` (`error.field.<CODE>` if it exists,
// else the generic VALIDATION_FAILED message); fields the form does not know are returned separately.
import type { ApiError } from './middleware/problem';

export interface FieldErrorUpdate {
  name: (string | number)[];
  errors: string[];
}

export function mapValidationErrors(
  error: ApiError,
  knownFields: readonly string[],
  t: (key: string) => string,
  exists: (key: string) => boolean,
): { fields: FieldErrorUpdate[]; unknown: string[] } {
  const fields: FieldErrorUpdate[] = [];
  const unknown: string[] = [];
  if (error.code !== 'VALIDATION_FAILED') return { fields, unknown };
  for (const [field, fe] of Object.entries(error.fieldErrors)) {
    const key = `error.field.${fe.code}`;
    const message = exists(key) ? t(key) : t('error.VALIDATION_FAILED');
    const path = field.split('.').map((p) => (/^\d+$/.test(p) ? Number(p) : p));
    if (knownFields.includes(String(path[0]))) fields.push({ name: path, errors: [message] });
    else unknown.push(field);
  }
  return { fields, unknown };
}
