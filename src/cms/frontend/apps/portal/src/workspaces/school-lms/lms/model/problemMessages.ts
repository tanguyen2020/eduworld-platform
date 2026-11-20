// ProblemCode → message key of the LMS screens (F055 T003, F055-AC-06): own LMS messages first (context-specific
// wording, e.g. "Bài này đã được nhắc hôm nay"), then the shared `error.<CODE>` of the apiClient. Never the raw
// `detail` of the server; the trace id is shown next to the message by LmsProblem.
import { ApiError } from '../../../../shared/api/middleware/problem';

/** Codes with an LMS-specific message (lms.errors.<CODE>). */
export const LMS_OWN_CODES = [
  'CONTENT_NOT_PUBLISHED',
  'RESOURCE_NOT_ASSIGNED',
  'PERMISSION_DENIED',
  'ASSIGNMENT_NOT_EDITABLE',
  'VERSION_MISMATCH',
  'CONFLICT',
  'INVALID_STATE_TRANSITION',
  'RATE_LIMITED',
  'TENANT_MISMATCH',
  'FEATURE_DISABLED',
  'VALIDATION_FAILED',
] as const;

export function lmsErrorKey(error: unknown): string {
  if (!(error instanceof ApiError)) return 'error.generic';
  if (error.status === 412) return 'lms.errors.VERSION_MISMATCH';
  return (LMS_OWN_CODES as readonly string[]).includes(error.code) ? `lms.errors.${error.code}` : error.i18nKey;
}

/** Codes after which the wizard goes back to the content step (edge case: content unpublished / unassigned). */
export function isContentError(error: unknown): boolean {
  return error instanceof ApiError && (error.code === 'CONTENT_NOT_PUBLISHED' || error.code === 'RESOURCE_NOT_ASSIGNED');
}

/** The resource changed on the server: reload the fresh copy, never overwrite (F055-AC-12). */
export function isStale(error: unknown): boolean {
  return error instanceof ApiError && (error.status === 412 || error.code === 'VERSION_MISMATCH' || error.code === 'ASSIGNMENT_NOT_EDITABLE');
}
