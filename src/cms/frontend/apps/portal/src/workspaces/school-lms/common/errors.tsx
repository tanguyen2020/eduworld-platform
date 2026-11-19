// Error display of workspace C screens (F052-AC-07, F052-AC-25, F052-AC-26, F052-AC-28):
// - a resource outside the user's scope answers 404 (or 403) → "Không tìm thấy", never "exists but forbidden";
// - 403 PERMISSION_DENIED on a write → "không có quyền" (the apiClient also refreshes getMyPermissions);
// - 403 FEATURE_DISABLED → "tính năng tạm tắt" (the shell refreshes permissions, F051-AC-37);
// - 422 VALIDATION_FAILED errors[] → the matching form field (mapValidationErrors of F051).
import { Result } from 'antd';
import type { FormInstance } from 'antd';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../../../shared/api/middleware/problem';
import { mapValidationErrors } from '../../../shared/api/mapValidationErrors';
import { ProblemMessage } from '../../../shell/ProblemMessage';

/** 404, or 403 other than FEATURE_DISABLED, on a read of one resource: shown as "not found" (AC-ORG-001/003). */
export function isHiddenResource(error: unknown): boolean {
  return error instanceof ApiError && (error.status === 404 || (error.status === 403 && error.code !== 'FEATURE_DISABLED'));
}

export function NotFoundResult() {
  const { t } = useTranslation();
  return (
    <div data-testid="school-lms-not-found">
      <Result status="404" title={t('portal.school_lms.notFound.title')} subTitle={t('portal.school_lms.notFound.description')} />
    </div>
  );
}

/** Error of a read query: "not found" for out-of-scope resources, the translated Problem otherwise. */
export function QueryError({ error, resource = false }: { error: unknown; resource?: boolean }) {
  if (resource && isHiddenResource(error)) return <NotFoundResult />;
  return <ProblemMessage error={error} />;
}

/** Message key of a write error code handled by F052 screens (own messages first, then the shared `error.<CODE>`). */
export function writeErrorKey(error: unknown): string {
  if (!(error instanceof ApiError)) return 'error.generic';
  const own: Record<string, string> = {
    USERNAME_TAKEN: 'portal.school_lms.errors.USERNAME_TAKEN',
    EMAIL_TAKEN: 'portal.school_lms.errors.EMAIL_TAKEN',
    ROLE_NOT_FOUND: 'portal.school_lms.errors.ROLE_NOT_FOUND',
    RETENTION_BELOW_MINIMUM: 'portal.school_lms.errors.RETENTION_BELOW_MINIMUM',
    PERMISSION_DENIED: 'portal.school_lms.errors.PERMISSION_DENIED',
    FEATURE_DISABLED: 'portal.school_lms.errors.FEATURE_DISABLED',
  };
  return own[error.code] ?? error.i18nKey;
}

/**
 * Applies a write error to an antd form: VALIDATION_FAILED errors[] to their fields; `fieldFor` maps other codes to one
 * field (e.g. 409 EMAIL_TAKEN → `email`). Returns true when the error was shown on a field.
 */
export function applyFormError(
  form: FormInstance,
  error: unknown,
  knownFields: readonly string[],
  t: (key: string) => string,
  exists: (key: string) => boolean,
  fieldFor: Record<string, string> = {},
): boolean {
  if (!(error instanceof ApiError)) return false;
  const target = fieldFor[error.code];
  if (target) {
    form.setFields([{ name: target.split('.'), errors: [t(writeErrorKey(error))] }]);
    return true;
  }
  const own = (key: string) => (exists(`portal.school_lms.fieldErrors.${key.replace(/^error\.field\./, '')}`) ? `portal.school_lms.fieldErrors.${key.replace(/^error\.field\./, '')}` : key);
  const { fields } = mapValidationErrors(error, knownFields, (k) => t(own(k)), (k) => exists(own(k)));
  if (fields.length) form.setFields(fields);
  return fields.length > 0;
}

export { ProblemMessage };
