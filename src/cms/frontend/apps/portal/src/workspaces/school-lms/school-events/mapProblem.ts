// Write errors of the school event form (F081-AC-04, AC-07): `errors[].field` of 422 VALIDATION_FAILED or
// EVENT_TEMPLATE_PARAM_NOT_ALLOWED → the matching form field (`name.vi` → ['name', 'vi']); a flagged text only shows a
// generic message — the server never sends the matched word and the portal never builds one (R15). Other codes → a
// message key of the screen (409 = concurrent limit on create / changed on update, 503 = retry, form kept).
import { ApiError } from '../../../shared/api/middleware/problem';

/** Form fields of the school event form (top-level names of the paths the server may name). */
export const FORM_FIELDS = ['template_id', 'name', 'description', 'starts_at', 'ends_at', 'audience', 'community_goal_target', 'quest_ids', 'challenge_ids', 'reward_option_id'] as const;

export interface MappedProblem {
  /** Field errors to set on the form. */
  fields: { name: (string | number)[]; errors: string[] }[];
  /** Message key of the banner, null when every error went to a field. */
  banner: string | null;
}

const FIELD_CODES = new Set(['VALIDATION_FAILED', 'EVENT_TEMPLATE_PARAM_NOT_ALLOWED']);

/** Pure mapping (unit-tested): `kind` tells create (409 = limit reached) from update (409 = data changed). */
export function mapEventProblem(error: unknown, kind: 'create' | 'update', t: (key: string) => string): MappedProblem {
  if (!(error instanceof ApiError)) return { fields: [], banner: 'error.generic' };
  const fields: MappedProblem['fields'] = [];
  let unmatched = false;
  if (FIELD_CODES.has(error.code)) {
    const message = t(error.code === 'VALIDATION_FAILED' ? 'portal.school_lms.schoolEvents.errors.fieldInvalid' : 'portal.school_lms.schoolEvents.errors.paramNotAllowed');
    for (const field of Object.keys(error.fieldErrors)) {
      const path = field.split('.').map((p) => (/^\d+$/.test(p) ? Number(p) : p));
      if ((FORM_FIELDS as readonly string[]).includes(String(path[0]))) fields.push({ name: path, errors: [message] });
      else unmatched = true;
    }
    if (!fields.length) unmatched = true;
  }
  let banner: string | null = null;
  if (unmatched) banner = error.code === 'VALIDATION_FAILED' ? 'portal.school_lms.schoolEvents.errors.fieldInvalid' : 'portal.school_lms.schoolEvents.errors.paramNotAllowed';
  else if (!fields.length) {
    if (error.status === 409) banner = kind === 'create' ? 'portal.school_lms.schoolEvents.errors.tooMany' : 'portal.school_lms.schoolEvents.errors.changed';
    else if (error.status === 412) banner = 'portal.school_lms.schoolEvents.errors.changed';
    else if (error.status === 503) banner = 'portal.school_lms.schoolEvents.errors.unavailable';
    else if (error.code === 'FEATURE_DISABLED') banner = 'portal.school_lms.errors.FEATURE_DISABLED';
    else banner = error.i18nKey;
  }
  return { fields, banner };
}
