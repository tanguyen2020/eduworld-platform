// Field limits and enums of the F054 forms and filters, copied from specs/openapi/eduworld-api.v1.yaml (ADR-0041).
// Each entry names the OpenAPI location it mirrors; `fieldSchemas.test.ts` fails when the spec and this file
// disagree, so the spec stays the single source (AGENTS R33). Used with `openapiRules` / `openapiInputProps` of ops-ui.
import type { OpenapiStringSchema } from '@eduworld/ops-ui';
import type { components } from '../../../shared/api/schema.gen';

export interface FieldSchema extends OpenapiStringSchema {
  /** `schemas/<Schema>/properties/<prop>`, `parameters/<Param>`, `paths/<operationId>/<query param>` or `paths/<operationId>/body/<prop>`. */
  source: string;
}

export const FIELDS = {
  /** transitionContentVersion `note` (reject requires it). */
  transitionNote: { source: 'paths/transitionContentVersion/body/note', maxLength: 2000 },
  /** ReviewFieldComment.path — JSON pointer (RFC 6901), empty = the whole version (OQ F054-1). */
  reviewFieldPath: { source: 'schemas/ReviewFieldComment/properties/path', maxLength: 256 },
  /** ReviewFieldComment.comment. */
  reviewFieldComment: { source: 'schemas/ReviewFieldComment/properties/comment', minLength: 1, maxLength: 1000 },
} as const satisfies Record<string, FieldSchema>;

/** Array limits mirrored from the spec (same check). */
export const LIMITS = {
  /** transitionContentVersion `field_comments.maxItems` (accepted with `reject` only). */
  reviewFieldComments: { source: 'paths/transitionContentVersion/body/field_comments', maxItems: 50 },
} as const;

export type ContentLanguage = components['schemas']['ContentLanguage'];
export type TranslationStatus = components['schemas']['TranslationStatus'];

// Records keyed by the generated enum types: adding a value to the contract breaks typecheck until it is listed here;
// `fieldSchemas.test.ts` also compares the lists with the spec.
const LANGUAGE_KEYS: Record<ContentLanguage, true> = { vi: true, en: true };
const TRANSLATION_KEYS: Record<TranslationStatus, true> = { complete: true, partial: true, outdated: true };

/** `ContentLanguage` enum — filter `content_language` of searchQuestions / listCms* (OQ F054-2). */
export const CONTENT_LANGUAGES = Object.keys(LANGUAGE_KEYS) as ContentLanguage[];
/** `TranslationStatus` enum — filter `translation_status` of searchQuestions / listCms* (OQ F054-2). */
export const TRANSLATION_STATUSES = Object.keys(TRANSLATION_KEYS) as TranslationStatus[];

/** Enum sources checked by the test. */
export const ENUMS = {
  contentLanguage: { source: 'parameters/ContentLanguageFilter', values: CONTENT_LANGUAGES },
  translationStatus: { source: 'parameters/TranslationStatusFilter', values: TRANSLATION_STATUSES },
} as const;
